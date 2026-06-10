package externalregistrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type UpsertInput struct {
	PaymentID     string
	Source        string
	ExternalRef   string
	CustomerName  string
	CustomerEmail string
	RawPayload    any
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
