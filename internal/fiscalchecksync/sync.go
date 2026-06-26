package fiscalchecksync

import (
	"context"
	"errors"
	"fmt"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

const DefaultBatchLimit = 100

var (
	ErrMissingInvoice = errors.New("registration payment invoice is missing")
	ErrNotFound       = errors.New("registration payment not found")
)

type MissingPayment struct {
	PaymentID string
	InvoiceID string
}

type Result struct {
	Scanned int `json:"scanned"`
	Synced  int `json:"synced"`
	Empty   int `json:"empty"`
	Failed  int `json:"failed"`
}

type Store interface {
	ListMissing(ctx context.Context, limit int) ([]MissingPayment, error)
	Upsert(ctx context.Context, paymentID, invoiceID string, checks []monobank.FiscalCheck) error
}

type Client interface {
	FetchFiscalChecks(ctx context.Context, invoiceID string) ([]monobank.FiscalCheck, error)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultBatchLimit
	}
	return limit
}

func SyncMissing(
	ctx context.Context,
	store Store,
	client Client,
	limit int,
) (Result, error) {
	payments, err := store.ListMissing(ctx, normalizeLimit(limit))
	if err != nil {
		return Result{}, err
	}
	return syncPayments(ctx, store, client, payments)
}

func SyncPayment(
	ctx context.Context,
	store Store,
	client Client,
	payment MissingPayment,
) (Result, error) {
	return syncPayments(ctx, store, client, []MissingPayment{payment})
}

func syncPayments(
	ctx context.Context,
	store Store,
	client Client,
	payments []MissingPayment,
) (Result, error) {
	result := Result{Scanned: len(payments)}
	errs := make([]error, 0)
	for _, payment := range payments {
		checks, err := client.FetchFiscalChecks(ctx, payment.InvoiceID)
		if err != nil {
			result.Failed++
			errs = append(errs, fmt.Errorf("fetch fiscal checks for invoice %s: %w", payment.InvoiceID, err))
			continue
		}
		if len(checks) == 0 {
			result.Empty++
			continue
		}
		if err := store.Upsert(ctx, payment.PaymentID, payment.InvoiceID, checks); err != nil {
			result.Failed++
			errs = append(errs, fmt.Errorf("upsert fiscal checks for payment %s: %w", payment.PaymentID, err))
			continue
		}
		result.Synced++
	}

	return result, errors.Join(errs...)
}
