package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/fiscalchecksync"
)

func runRegistrationPaymentFiscalCheckSync(
	t *testing.T,
	deps registrationPaymentDeps,
	paymentID string,
) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := gin.New()
	r.POST("/api/registration-payments/:paymentID/fiscal-checks/sync", func(c *gin.Context) {
		handleRegistrationPaymentFiscalCheckSync(c, deps)
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/registration-payments/"+paymentID+"/fiscal-checks/sync",
		nil,
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestRegistrationPaymentFiscalCheckSyncFetchesChecks(t *testing.T) {
	var foundPaymentID string
	var syncedPayment fiscalchecksync.MissingPayment
	deps := registrationPaymentDeps{
		FindFiscalPayment: func(_ context.Context, paymentID string) (fiscalchecksync.MissingPayment, error) {
			foundPaymentID = paymentID
			return fiscalchecksync.MissingPayment{
				PaymentID: paymentID,
				InvoiceID: "invoice-123",
			}, nil
		},
		SyncFiscalChecks: func(_ context.Context, payment fiscalchecksync.MissingPayment) (fiscalchecksync.Result, error) {
			syncedPayment = payment
			return fiscalchecksync.Result{Scanned: 1, Synced: 1}, nil
		},
	}

	rec, body := runRegistrationPaymentFiscalCheckSync(t, deps, "payment-123")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if foundPaymentID != "payment-123" {
		t.Fatalf("found payment id: got %q, want payment-123", foundPaymentID)
	}
	if syncedPayment.InvoiceID != "invoice-123" {
		t.Fatalf("synced invoice id: got %q, want invoice-123", syncedPayment.InvoiceID)
	}
	result, ok := body["result"].(map[string]any)
	if !ok {
		t.Fatalf("result: got %#v", body["result"])
	}
	if result["synced"] != float64(1) {
		t.Fatalf("synced: got %#v, want 1", result["synced"])
	}
}

func TestRegistrationPaymentFiscalCheckSyncRejectsRowsWithoutInvoice(t *testing.T) {
	deps := registrationPaymentDeps{
		FindFiscalPayment: func(context.Context, string) (fiscalchecksync.MissingPayment, error) {
			return fiscalchecksync.MissingPayment{}, fiscalchecksync.ErrMissingInvoice
		},
		SyncFiscalChecks: func(context.Context, fiscalchecksync.MissingPayment) (fiscalchecksync.Result, error) {
			t.Fatal("sync should not run when invoice is missing")
			return fiscalchecksync.Result{}, nil
		},
	}

	rec, body := runRegistrationPaymentFiscalCheckSync(t, deps, "payment-no-invoice")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if body["error"] != "Registration payment does not have a Monobank invoice." {
		t.Fatalf("error: got %#v", body["error"])
	}
}

func TestRegistrationPaymentFiscalCheckSyncReturnsProviderErrors(t *testing.T) {
	providerErr := errors.New("monobank unavailable")
	deps := registrationPaymentDeps{
		FindFiscalPayment: func(_ context.Context, paymentID string) (fiscalchecksync.MissingPayment, error) {
			return fiscalchecksync.MissingPayment{PaymentID: paymentID, InvoiceID: "invoice-123"}, nil
		},
		SyncFiscalChecks: func(context.Context, fiscalchecksync.MissingPayment) (fiscalchecksync.Result, error) {
			return fiscalchecksync.Result{Scanned: 1, Failed: 1}, providerErr
		},
	}

	rec, body := runRegistrationPaymentFiscalCheckSync(t, deps, "payment-123")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	if body["error"] != "Failed to sync fiscal checks: monobank unavailable" {
		t.Fatalf("error: got %#v", body["error"])
	}
}
