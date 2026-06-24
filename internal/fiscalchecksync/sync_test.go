package fiscalchecksync

import (
	"context"
	"errors"
	"testing"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

type fakeStore struct {
	missing      []MissingPayment
	listCalls    int
	upserts      []upsertCall
	upsertErrFor map[string]error
}

type upsertCall struct {
	paymentID string
	invoiceID string
	checks    []monobank.FiscalCheck
}

func (s *fakeStore) ListMissing(ctx context.Context, limit int) ([]MissingPayment, error) {
	s.listCalls++
	return append([]MissingPayment(nil), s.missing...), nil
}

func (s *fakeStore) Upsert(ctx context.Context, paymentID, invoiceID string, checks []monobank.FiscalCheck) error {
	s.upserts = append(s.upserts, upsertCall{
		paymentID: paymentID,
		invoiceID: invoiceID,
		checks:    append([]monobank.FiscalCheck(nil), checks...),
	})
	if s.upsertErrFor != nil {
		return s.upsertErrFor[paymentID]
	}
	return nil
}

type fakeClient struct {
	checks map[string][]monobank.FiscalCheck
	errs   map[string]error
	calls  []string
}

func (c *fakeClient) FetchFiscalChecks(ctx context.Context, invoiceID string) ([]monobank.FiscalCheck, error) {
	c.calls = append(c.calls, invoiceID)
	if c.errs != nil && c.errs[invoiceID] != nil {
		return nil, c.errs[invoiceID]
	}
	return append([]monobank.FiscalCheck(nil), c.checks[invoiceID]...), nil
}

func TestSyncMissingFetchesAndUpsertsChecks(t *testing.T) {
	store := &fakeStore{
		missing: []MissingPayment{
			{PaymentID: "payment-1", InvoiceID: "invoice-1"},
			{PaymentID: "payment-2", InvoiceID: "invoice-2"},
		},
	}
	client := &fakeClient{
		checks: map[string][]monobank.FiscalCheck{
			"invoice-1": {
				{ID: "check-1", Status: "done", Type: "receipt", FiscalizationSource: "checkbox"},
			},
			"invoice-2": {
				{ID: "check-2", Status: "done", Type: "receipt", FiscalizationSource: "checkbox"},
			},
		},
	}

	result, err := SyncMissing(context.Background(), store, client, 50)

	if err != nil {
		t.Fatalf("SyncMissing error: %v", err)
	}
	if result.Scanned != 2 || result.Synced != 2 || result.Empty != 0 || result.Failed != 0 {
		t.Fatalf("result: got %+v, want scanned=2 synced=2 empty=0 failed=0", result)
	}
	if store.listCalls != 1 {
		t.Fatalf("list calls: got %d, want 1", store.listCalls)
	}
	if len(client.calls) != 2 || client.calls[0] != "invoice-1" || client.calls[1] != "invoice-2" {
		t.Fatalf("client calls: got %v, want [invoice-1 invoice-2]", client.calls)
	}
	if len(store.upserts) != 2 {
		t.Fatalf("upserts: got %d, want 2", len(store.upserts))
	}
	if store.upserts[0].paymentID != "payment-1" || store.upserts[0].invoiceID != "invoice-1" {
		t.Fatalf("first upsert: got %+v", store.upserts[0])
	}
	if got := store.upserts[0].checks[0].ID; got != "check-1" {
		t.Fatalf("first check id: got %q, want check-1", got)
	}
}

func TestSyncMissingKeepsGoingWhenAReceiptIsNotReady(t *testing.T) {
	store := &fakeStore{
		missing: []MissingPayment{
			{PaymentID: "payment-empty", InvoiceID: "invoice-empty"},
			{PaymentID: "payment-ready", InvoiceID: "invoice-ready"},
		},
	}
	client := &fakeClient{
		checks: map[string][]monobank.FiscalCheck{
			"invoice-ready": {
				{ID: "check-ready", Status: "done", Type: "receipt", FiscalizationSource: "checkbox"},
			},
		},
	}

	result, err := SyncMissing(context.Background(), store, client, 50)

	if err != nil {
		t.Fatalf("SyncMissing error: %v", err)
	}
	if result.Scanned != 2 || result.Synced != 1 || result.Empty != 1 || result.Failed != 0 {
		t.Fatalf("result: got %+v, want scanned=2 synced=1 empty=1 failed=0", result)
	}
	if len(store.upserts) != 1 || store.upserts[0].paymentID != "payment-ready" {
		t.Fatalf("upserts: got %+v, want one upsert for payment-ready", store.upserts)
	}
}

func TestSyncMissingReportsFailuresAfterProcessingRemainingRows(t *testing.T) {
	fetchErr := errors.New("monobank unavailable")
	store := &fakeStore{
		missing: []MissingPayment{
			{PaymentID: "payment-fail", InvoiceID: "invoice-fail"},
			{PaymentID: "payment-ready", InvoiceID: "invoice-ready"},
		},
	}
	client := &fakeClient{
		errs: map[string]error{"invoice-fail": fetchErr},
		checks: map[string][]monobank.FiscalCheck{
			"invoice-ready": {
				{ID: "check-ready", Status: "done", Type: "receipt", FiscalizationSource: "checkbox"},
			},
		},
	}

	result, err := SyncMissing(context.Background(), store, client, 50)

	if !errors.Is(err, fetchErr) {
		t.Fatalf("error: got %v, want fetchErr", err)
	}
	if result.Scanned != 2 || result.Synced != 1 || result.Empty != 0 || result.Failed != 1 {
		t.Fatalf("result: got %+v, want scanned=2 synced=1 empty=0 failed=1", result)
	}
	if len(store.upserts) != 1 || store.upserts[0].paymentID != "payment-ready" {
		t.Fatalf("upserts: got %+v, want one upsert for payment-ready", store.upserts)
	}
}
