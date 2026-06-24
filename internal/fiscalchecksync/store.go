package fiscalchecksync

import (
	"context"
	"fmt"

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
		  )
		order by p.updated_at desc, p.created_at desc
		limit $3
	`, string(payments.StatusPaid), externalcheckout.ParticipationProductSlug, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("select paid registration payments missing fiscal checks: %w", err)
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

func (DBStore) Upsert(ctx context.Context, paymentID, invoiceID string, checks []monobank.FiscalCheck) error {
	return fiscalchecks.UpsertForPayment(ctx, paymentID, invoiceID, checks)
}
