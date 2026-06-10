package fiscalchecks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func toJSONB(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal fiscal check payload: %w", err)
	}
	return string(b), nil
}

func UpsertForPayment(ctx context.Context, paymentID, invoiceID string, checks []monobank.FiscalCheck) error {
	if len(checks) == 0 {
		return nil
	}
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	for _, check := range checks {
		payload, err := toJSONB(check)
		if err != nil {
			return err
		}
		_, err = pool.Exec(ctx, `
			insert into fiscal_checks (
				id,
				payment_id,
				invoice_id,
				check_id,
				status,
				type,
				fiscalization_source,
				status_description,
				tax_url,
				file,
				payload
			)
			values ($1, $2, $3, $4, $5, $6, $7, nullif($8, ''), nullif($9, ''), nullif($10, ''), $11::jsonb)
			on conflict (check_id) do update
			set
				status = excluded.status,
				type = excluded.type,
				fiscalization_source = excluded.fiscalization_source,
				status_description = excluded.status_description,
				tax_url = excluded.tax_url,
				file = excluded.file,
				payload = excluded.payload,
				updated_at = timezone('utc', now())
		`,
			uuid.NewString(),
			paymentID,
			invoiceID,
			check.ID,
			check.Status,
			check.Type,
			check.FiscalizationSource,
			check.StatusDescription,
			check.TaxURL,
			check.File,
			payload,
		)
		if err != nil {
			return fmt.Errorf("upsert fiscal check %s: %w", check.ID, err)
		}
	}
	return nil
}
