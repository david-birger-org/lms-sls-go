package fiscalchecksync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/externalcheckout"
	"github.com/apexwoot/lms-sls-go/internal/fiscalchecks"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/payments"
)

type DBStore struct{}

func (DBStore) ListMissing(ctx context.Context, limit int) ([]MissingPayment, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select p.id, p.invoice_id
		from external_registrations er
		join payments p on p.id = er.payment_id
		where p.provider = 'monobank'
		  and p.status = $1
		  and p.product_slug = $2
		  and nullif(trim(p.invoice_id), '') is not null
		  and not exists (
			  select 1
			  from fiscal_checks fc
			  where fc.payment_id = p.id
			    and (
			      nullif(trim(fc.tax_url), '') is not null
			      or nullif(trim(fc.file), '') ~* '^(https?://|data:application/pdf|JVBER)'
			    )
		  )
		order by p.updated_at desc, p.created_at desc
		limit $3
	`, string(payments.StatusPaid), externalcheckout.ParticipationProductSlug, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("select paid registration payments missing usable fiscal checks: %w", err)
	}
	defer rows.Close()

	out := make([]MissingPayment, 0)
	for rows.Next() {
		var payment MissingPayment
		if err := rows.Scan(&payment.PaymentID, &payment.InvoiceID); err != nil {
			return nil, err
		}
		out = append(out, payment)
	}
	return out, rows.Err()
}

func (DBStore) FindRegistrationPayment(ctx context.Context, paymentID string) (MissingPayment, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return MissingPayment{}, err
	}
	var payment MissingPayment
	var invoiceID *string
	err = pool.QueryRow(ctx, `
		select p.id, p.invoice_id
		from external_registrations er
		join payments p on p.id = er.payment_id
		where p.id = $1
		  and p.provider = 'monobank'
		limit 1
	`, paymentID).Scan(&payment.PaymentID, &invoiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissingPayment{}, ErrNotFound
	}
	if err != nil {
		return MissingPayment{}, err
	}
	if invoiceID == nil || strings.TrimSpace(*invoiceID) == "" {
		return MissingPayment{}, ErrMissingInvoice
	}
	payment.InvoiceID = strings.TrimSpace(*invoiceID)
	return payment, nil
}

func (DBStore) Upsert(ctx context.Context, paymentID, invoiceID string, checks []monobank.FiscalCheck) error {
	return fiscalchecks.UpsertForPayment(ctx, paymentID, invoiceID, checks)
}
