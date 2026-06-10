package registrationpayments

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/payments"
)

type Row struct {
	ID                  string                     `json:"id"`
	PaymentID           string                     `json:"paymentId"`
	Source              string                     `json:"source"`
	ExternalRef         string                     `json:"externalRef"`
	CustomerName        string                     `json:"customerName"`
	CustomerEmail       string                     `json:"customerEmail"`
	AmountMinor         int64                      `json:"amountMinor"`
	Currency            monobank.SupportedCurrency `json:"currency"`
	Description         string                     `json:"description"`
	Status              payments.Status            `json:"status"`
	ProviderStatus      *string                    `json:"providerStatus,omitempty"`
	InvoiceID           *string                    `json:"invoiceId,omitempty"`
	PageURL             *string                    `json:"pageUrl,omitempty"`
	FailureReason       *string                    `json:"failureReason,omitempty"`
	ProductSlug         *string                    `json:"productSlug,omitempty"`
	PaymentCreatedAt    time.Time                  `json:"paymentCreatedAt"`
	PaymentUpdatedAt    time.Time                  `json:"paymentUpdatedAt"`
	ProviderModifiedAt  *time.Time                 `json:"providerModifiedAt,omitempty"`
	RegistrationCreated time.Time                  `json:"registrationCreatedAt"`
	CheckID             *string                    `json:"checkId,omitempty"`
	CheckStatus         *string                    `json:"checkStatus,omitempty"`
	CheckTaxURL         *string                    `json:"checkTaxUrl,omitempty"`
	CheckFile           *string                    `json:"checkFile,omitempty"`
	CheckUpdatedAt      *time.Time                 `json:"checkUpdatedAt,omitempty"`
}

func scanRows(rows pgx.Rows) ([]Row, error) {
	defer rows.Close()
	out := make([]Row, 0)
	for rows.Next() {
		var row Row
		var currency string
		var status string
		if err := rows.Scan(
			&row.ID,
			&row.PaymentID,
			&row.Source,
			&row.ExternalRef,
			&row.CustomerName,
			&row.CustomerEmail,
			&row.AmountMinor,
			&currency,
			&row.Description,
			&status,
			&row.ProviderStatus,
			&row.InvoiceID,
			&row.PageURL,
			&row.FailureReason,
			&row.ProductSlug,
			&row.PaymentCreatedAt,
			&row.PaymentUpdatedAt,
			&row.ProviderModifiedAt,
			&row.RegistrationCreated,
			&row.CheckID,
			&row.CheckStatus,
			&row.CheckTaxURL,
			&row.CheckFile,
			&row.CheckUpdatedAt,
		); err != nil {
			return nil, err
		}
		row.Currency = monobank.SupportedCurrency(currency)
		row.Status = payments.Status(status)
		out = append(out, row)
	}
	return out, rows.Err()
}

func SelectAll(ctx context.Context) ([]Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select
			er.id,
			er.payment_id,
			er.source,
			er.external_ref,
			er.customer_name,
			er.customer_email,
			p.amount_minor,
			p.currency,
			p.description,
			p.status,
			p.provider_status,
			p.invoice_id,
			p.page_url,
			p.failure_reason,
			p.product_slug,
			p.created_at,
			p.updated_at,
			p.provider_modified_at,
			er.created_at,
			fc.check_id,
			fc.status,
			fc.tax_url,
			fc.file,
			fc.updated_at
		from external_registrations er
		join payments p on p.id = er.payment_id
		left join lateral (
			select check_id, status, tax_url, file, updated_at
			from fiscal_checks
			where payment_id = p.id
			order by updated_at desc
			limit 1
		) fc on true
		order by er.created_at desc
		limit 500
	`)
	if err != nil {
		return nil, fmt.Errorf("select registration payments: %w", err)
	}
	return scanRows(rows)
}
