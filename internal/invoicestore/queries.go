package invoicestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/payments"
)

func toJSONB(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

func SelectAppUserIDByAuthUserID(ctx context.Context, authUserID string) (string, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `
		select id
		from app_users
		where auth_user_id = $1
		  and deleted_at is null
		limit 1
	`, authUserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func MirrorAuthUserToAppUsers(ctx context.Context, in MirrorAuthUserInput) (string, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `
		insert into app_users (auth_user_id, email, full_name, deleted_at)
		values ($1, $2, $3, null)
		on conflict (auth_user_id) do update
		set
			email = excluded.email,
			full_name = excluded.full_name,
			deleted_at = null,
			updated_at = timezone('utc', now())
		returning id
	`, in.AuthUserID, in.Email, in.FullName).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("Failed to resolve app user %s: %w", in.AuthUserID, err)
	}
	return id, nil
}

func SelectPaymentByIdempotencyKey(ctx context.Context, key string) (*IdempotencyPayment, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	var row IdempotencyPayment
	err = pool.QueryRow(ctx, `
		select id, invoice_id, page_url, expires_at, status
		from payments
		where idempotency_key = $1
		limit 1
	`, key).Scan(&row.ID, &row.InvoiceID, &row.PageURL, &row.ExpiresAt, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func InsertPendingInvoice(ctx context.Context, in CreatePendingInvoiceInput) (PendingInvoiceCreation, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return PendingInvoiceCreation{}, err
	}
	var out PendingInvoiceCreation
	err = pool.QueryRow(ctx, `
		insert into payments (
			id,
			user_id,
			created_by_admin_user_id,
			provider,
			reference,
			status,
			amount_minor,
			currency,
			customer_name,
			customer_email,
			description,
			idempotency_key,
			product_id,
			product_slug
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		returning id, reference
	`,
		in.PaymentID,
		in.UserID,
		in.CreatedByAdminUserID,
		"monobank",
		"mb-"+in.PaymentID,
		string(payments.StatusCreatingInvoice),
		in.AmountMinor,
		string(in.Currency),
		in.CustomerName,
		in.CustomerEmail,
		in.Description,
		in.IdempotencyKey,
		in.ProductID,
		in.ProductSlug,
	).Scan(&out.PaymentID, &out.Reference)
	if err != nil {
		return PendingInvoiceCreation{}, fmt.Errorf("Failed to create pending invoice record %s: %w", in.PaymentID, err)
	}
	return out, nil
}

func UpdateCreatedInvoice(ctx context.Context, in StoreCreatedInvoiceInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update payments
		set
			invoice_id = $1,
			page_url = $2,
			expires_at = $3,
			provider_status = $4,
			status = $5,
			failure_reason = null,
			provider_payload = coalesce($6::jsonb, provider_payload),
			updated_at = timezone('utc', now())
		where id = $7
	`,
		in.InvoiceID,
		in.PageURL,
		in.ExpiresAt,
		payments.PendingMonobankProviderStatuses[0],
		string(payments.StatusInvoiceCreated),
		toJSONB(in.ProviderPayload),
		in.PaymentID,
	)
	return err
}

func UpdateInvoiceCreationFailed(ctx context.Context, in MarkInvoiceCreationFailedInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update payments
		set
			status = case
				when payments.invoice_id is null then $1
				else payments.status
			end,
			failure_reason = case
				when payments.invoice_id is null then $2
				else payments.failure_reason
			end,
			provider_payload = coalesce($3::jsonb, provider_payload),
			updated_at = timezone('utc', now())
		where id = $4
	`,
		string(payments.StatusFailed),
		in.ErrorMessage,
		toJSONB(in.ProviderPayload),
		in.PaymentID,
	)
	return err
}

func SelectPendingPaymentRows(ctx context.Context, limit int) ([]PendingPaymentRow, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select
			amount_minor,
			created_at,
			currency,
			customer_name,
			description,
			expires_at,
			failure_reason,
			invoice_id,
			page_url,
			product_slug,
			provider_status,
			reference,
			status
		from payments
		where invoice_id is not null
		  and (
			status in ($1, $2)
			or provider_status in ($3, $4, $5)
		  )
		order by created_at desc
		limit $6
	`,
		string(payments.PendingStatuses[0]),
		string(payments.PendingStatuses[1]),
		payments.PendingMonobankProviderStatuses[0],
		payments.PendingMonobankProviderStatuses[1],
		payments.PendingMonobankProviderStatuses[2],
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingPaymentRow
	for rows.Next() {
		var r PendingPaymentRow
		var currency string
		var status string
		if err := rows.Scan(
			&r.AmountMinor,
			&r.CreatedAt,
			&currency,
			&r.CustomerName,
			&r.Description,
			&r.ExpiresAt,
			&r.FailureReason,
			&r.InvoiceID,
			&r.PageURL,
			&r.ProductSlug,
			&r.ProviderStatus,
			&r.Reference,
			&status,
		); err != nil {
			return nil, err
		}
		r.Currency = monobank.SupportedCurrency(currency)
		r.Status = payments.Status(status)
		out = append(out, r)
	}
	return out, rows.Err()
}

const paymentHistoryColumns = `
	amount_minor,
	created_at,
	currency,
	customer_name,
	description,
	expires_at,
	failure_reason,
	profit_amount_minor,
	invoice_id,
	page_url,
	payment_info,
	product_slug,
	provider_modified_at,
	provider_status,
	reference,
	status
`

func scanPaymentHistoryRows(rows pgx.Rows) ([]PaymentHistoryRow, error) {
	defer rows.Close()
	var out []PaymentHistoryRow
	for rows.Next() {
		var r PaymentHistoryRow
		var currency string
		var status string
		if err := rows.Scan(
			&r.AmountMinor,
			&r.CreatedAt,
			&currency,
			&r.CustomerName,
			&r.Description,
			&r.ExpiresAt,
			&r.FailureReason,
			&r.ProfitAmountMinor,
			&r.InvoiceID,
			&r.PageURL,
			&r.PaymentInfoJSON,
			&r.ProductSlug,
			&r.ProviderModifiedAt,
			&r.ProviderStatus,
			&r.Reference,
			&status,
		); err != nil {
			return nil, err
		}
		r.Currency = monobank.SupportedCurrency(currency)
		r.Status = payments.Status(status)
		out = append(out, r)
	}
	return out, rows.Err()
}

func SelectPaymentHistoryRows(ctx context.Context, from, to time.Time) ([]PaymentHistoryRow, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select `+paymentHistoryColumns+`
		from payments
		where provider = 'monobank'
		  and invoice_id is not null
		  and coalesce(provider_modified_at, created_at) >= $1
		  and coalesce(provider_modified_at, created_at) <= $2
		order by coalesce(provider_modified_at, created_at) desc, created_at desc
	`, from, to)
	if err != nil {
		return nil, err
	}
	return scanPaymentHistoryRows(rows)
}

func SelectRecentPaymentsByCustomerName(ctx context.Context, name string) ([]PaymentHistoryRow, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select `+paymentHistoryColumns+`
		from payments
		where provider = 'monobank'
		  and invoice_id is not null
		  and lower(customer_name) = lower($1)
		  and created_at >= now() - interval '30 days'
		order by created_at desc
	`, name)
	if err != nil {
		return nil, err
	}
	return scanPaymentHistoryRows(rows)
}

func SelectPaymentHistoryRowByInvoiceID(ctx context.Context, invoiceID string) (*PaymentHistoryRow, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select `+paymentHistoryColumns+`
		from payments
		where provider = 'monobank'
		  and invoice_id = $1
		order by updated_at desc, created_at desc
		limit 1
	`, invoiceID)
	if err != nil {
		return nil, err
	}
	list, err := scanPaymentHistoryRows(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

func UpdateInvoiceCancelled(ctx context.Context, invoiceID string, providerPayload any) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update payments
		set
			provider_status = $1,
			status = $1,
			provider_payload = coalesce($2::jsonb, provider_payload),
			updated_at = timezone('utc', now())
		where invoice_id = $3
	`,
		string(payments.StatusCancelled),
		toJSONB(providerPayload),
		invoiceID,
	)
	return err
}

func SelectLatestProviderState(ctx context.Context, invoiceID, reference *string) (*time.Time, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	var modified *time.Time
	switch {
	case invoiceID != nil && reference != nil:
		err = pool.QueryRow(ctx, `
			select provider_modified_at
			from payments
			where invoice_id = $1
			   or reference = $2
			order by provider_modified_at desc nulls last, updated_at desc, created_at desc
			limit 1
		`, *invoiceID, *reference).Scan(&modified)
	case invoiceID != nil:
		err = pool.QueryRow(ctx, `
			select provider_modified_at
			from payments
			where invoice_id = $1
			order by provider_modified_at desc nulls last, updated_at desc, created_at desc
			limit 1
		`, *invoiceID).Scan(&modified)
	case reference != nil:
		err = pool.QueryRow(ctx, `
			select provider_modified_at
			from payments
			where reference = $1
			order by provider_modified_at desc nulls last, updated_at desc, created_at desc
			limit 1
		`, *reference).Scan(&modified)
	default:
		return nil, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return modified, nil
}

func UpdatePaymentProviderState(ctx context.Context, in ProviderStateUpdateInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}

	providerPayloadJSON, err := json.Marshal(in.ProviderPayload)
	if err != nil {
		return fmt.Errorf("marshal provider payload: %w", err)
	}

	var currencyStr *string
	if in.Currency != nil {
		s := string(*in.Currency)
		currencyStr = &s
	}
	var statusStr *string
	if in.Status != nil {
		s := string(*in.Status)
		statusStr = &s
	}

	args := []any{
		in.ProviderStatus,
		in.ProviderModifiedAt,
		statusStr,
		in.FailureReason,
		in.AmountMinor,
		in.ProfitAmountMinor,
		currencyStr,
		toJSONB(in.PaymentInfo),
		string(providerPayloadJSON),
	}

	switch {
	case in.InvoiceID != nil && in.Reference != nil:
		_, err = pool.Exec(ctx, `
			update payments
			set
				invoice_id = coalesce(payments.invoice_id, $10),
				provider_status = coalesce($1, payments.provider_status),
				provider_modified_at = coalesce($2, payments.provider_modified_at),
				status = coalesce($3, payments.status),
				failure_reason = coalesce($4, payments.failure_reason),
				amount_minor = coalesce($5, payments.amount_minor),
				profit_amount_minor = coalesce($6, payments.profit_amount_minor),
				currency = coalesce($7, payments.currency),
				payment_info = coalesce($8::jsonb, payments.payment_info),
				provider_payload = $9::jsonb,
				updated_at = timezone('utc', now())
			where payments.invoice_id = $10
			   or payments.reference = $11
		`, append(args, *in.InvoiceID, *in.Reference)...)
	case in.InvoiceID != nil:
		_, err = pool.Exec(ctx, `
			update payments
			set
				invoice_id = coalesce(payments.invoice_id, $10),
				provider_status = coalesce($1, payments.provider_status),
				provider_modified_at = coalesce($2, payments.provider_modified_at),
				status = coalesce($3, payments.status),
				failure_reason = coalesce($4, payments.failure_reason),
				amount_minor = coalesce($5, payments.amount_minor),
				profit_amount_minor = coalesce($6, payments.profit_amount_minor),
				currency = coalesce($7, payments.currency),
				payment_info = coalesce($8::jsonb, payments.payment_info),
				provider_payload = $9::jsonb,
				updated_at = timezone('utc', now())
			where payments.invoice_id = $10
		`, append(args, *in.InvoiceID)...)
	case in.Reference != nil:
		_, err = pool.Exec(ctx, `
			update payments
			set
				provider_status = coalesce($1, payments.provider_status),
				provider_modified_at = coalesce($2, payments.provider_modified_at),
				status = coalesce($3, payments.status),
				failure_reason = coalesce($4, payments.failure_reason),
				amount_minor = coalesce($5, payments.amount_minor),
				profit_amount_minor = coalesce($6, payments.profit_amount_minor),
				currency = coalesce($7, payments.currency),
				payment_info = coalesce($8::jsonb, payments.payment_info),
				provider_payload = $9::jsonb,
				updated_at = timezone('utc', now())
			where payments.reference = $10
		`, append(args, *in.Reference)...)
	default:
		return nil
	}
	return err
}

func SelectPaymentForFeatureGrant(ctx context.Context, invoiceID, reference *string) (*PaymentFeatureGrantRow, error) {
	if invoiceID == nil && reference == nil {
		return nil, nil
	}
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	var row PaymentFeatureGrantRow
	if invoiceID != nil {
		err = pool.QueryRow(ctx, `
			select id, user_id, product_slug
			from payments
			where invoice_id = $1
			limit 1
		`, *invoiceID).Scan(&row.ID, &row.UserID, &row.ProductSlug)
	} else {
		err = pool.QueryRow(ctx, `
			select id, user_id, product_slug
			from payments
			where reference = $1
			limit 1
		`, *reference).Scan(&row.ID, &row.UserID, &row.ProductSlug)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
