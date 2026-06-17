package externalregistrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/payments"
)

type UpsertInput struct {
	PaymentID     string
	Source        string
	ExternalRef   string
	CustomerName  string
	CustomerEmail string
	RawPayload    any
}

type ActiveDuplicateInput struct {
	CustomerEmail string
	CustomerName  string
	CustomerPhone string
	ExternalRef   string
	ProductSlug   string
	Source        string
}

type ActiveDuplicate struct {
	ExternalRef string
	MatchField  string
	PaymentID   string
	Status      payments.Status
}

func toJSONB(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal raw payload: %w", err)
	}
	return string(b), nil
}

func Upsert(ctx context.Context, in UpsertInput) error {
	raw, err := toJSONB(in.RawPayload)
	if err != nil {
		return err
	}
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		insert into external_registrations (
			id,
			payment_id,
			source,
			external_ref,
			customer_name,
			customer_email,
			raw_payload
		)
		values ($1, $2, $3, $4, $5, $6, $7::jsonb)
		on conflict (payment_id) do update
		set
			external_ref = excluded.external_ref,
			customer_name = excluded.customer_name,
			customer_email = excluded.customer_email,
			raw_payload = excluded.raw_payload,
			updated_at = timezone('utc', now())
	`,
		uuid.NewString(),
		in.PaymentID,
		in.Source,
		in.ExternalRef,
		in.CustomerName,
		in.CustomerEmail,
		raw,
	)
	if err != nil {
		return fmt.Errorf("upsert external registration %s: %w", in.ExternalRef, err)
	}
	return nil
}

func FindActiveDuplicate(ctx context.Context, in ActiveDuplicateInput) (*ActiveDuplicate, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}

	var duplicate ActiveDuplicate
	var status string
	err = pool.QueryRow(ctx, `
		with input as (
			select
				lower(btrim($3)) as email,
				lower(regexp_replace(btrim($4), '\s+', ' ', 'g')) as name,
				regexp_replace(coalesce($5, ''), '\D', '', 'g') as phone
		)
		select
			er.payment_id,
			er.external_ref,
			p.status,
			case
				when lower(btrim(er.customer_email)) = input.email then 'email'
				when lower(regexp_replace(btrim(er.customer_name), '\s+', ' ', 'g')) = input.name then 'name'
				when input.phone <> '' and (
					regexp_replace(coalesce(er.raw_payload->>'customerPhone', ''), '\D', '', 'g') = input.phone
					or regexp_replace(coalesce(er.raw_payload->>'phone', ''), '\D', '', 'g') = input.phone
					or regexp_replace(coalesce(er.raw_payload->>'billingPhone', ''), '\D', '', 'g') = input.phone
				) then 'phone'
				else 'unknown'
			end as match_field
		from external_registrations er
		join payments p on p.id = er.payment_id
		cross join input
		where er.source = $1
		  and p.product_slug = $2
		  and p.status in ($7, $8, $9, $10)
		  and lower(btrim(er.external_ref)) <> lower(btrim($6))
		  and (
			lower(btrim(er.customer_email)) = input.email
			or lower(regexp_replace(btrim(er.customer_name), '\s+', ' ', 'g')) = input.name
			or (
				input.phone <> ''
				and (
					regexp_replace(coalesce(er.raw_payload->>'customerPhone', ''), '\D', '', 'g') = input.phone
					or regexp_replace(coalesce(er.raw_payload->>'phone', ''), '\D', '', 'g') = input.phone
					or regexp_replace(coalesce(er.raw_payload->>'billingPhone', ''), '\D', '', 'g') = input.phone
				)
			)
		  )
		order by (p.status = $10) desc, p.updated_at desc
		limit 1
	`,
		in.Source,
		in.ProductSlug,
		in.CustomerEmail,
		in.CustomerName,
		in.CustomerPhone,
		in.ExternalRef,
		string(payments.StatusCreatingInvoice),
		string(payments.StatusInvoiceCreated),
		string(payments.StatusProcessing),
		string(payments.StatusPaid),
	).Scan(
		&duplicate.PaymentID,
		&duplicate.ExternalRef,
		&status,
		&duplicate.MatchField,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find active duplicate registration: %w", err)
	}
	duplicate.Status = payments.Status(status)
	return &duplicate, nil
}
