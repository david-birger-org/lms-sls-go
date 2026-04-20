package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func init() { gin.SetMode(gin.TestMode) }

func makeWebhookRequest(t *testing.T, payload any, headers map[string]string) *http.Request {
	t.Helper()
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/monobank/webhook", bytes.NewReader(buf))
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func runWebhook(t *testing.T, deps WebhookDeps, req *http.Request) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := gin.New()
	r.POST("/api/monobank/webhook", func(c *gin.Context) { handleMonobankWebhook(c, deps) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestMonobankWebhook_RejectsMissingSignature(t *testing.T) {
	deps := WebhookDeps{
		GetPublicKey: func(context.Context, bool) (string, error) { return "public-key", nil },
		Sync:         func(context.Context, monobank.InvoiceStatusResponse) error { return nil },
		Verify:       func(monobank.VerifyWebhookInput) (bool, error) { return true, nil },
	}
	req := makeWebhookRequest(t, map[string]string{"invoiceId": "invoice_123"}, nil)
	rec, body := runWebhook(t, deps, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if body["error"] != "X-Sign header is required." {
		t.Fatalf("error: got %v", body["error"])
	}
}

func TestMonobankWebhook_ForcesRefreshOnFailure(t *testing.T) {
	var pubkeyCalls, verifyCalls, syncCalls int
	var syncedPayload monobank.InvoiceStatusResponse

	deps := WebhookDeps{
		GetPublicKey: func(_ context.Context, forceRefresh bool) (string, error) {
			pubkeyCalls++
			if forceRefresh {
				return "fresh-key", nil
			}
			return "cached-key", nil
		},
		Sync: func(_ context.Context, status monobank.InvoiceStatusResponse) error {
			syncCalls++
			syncedPayload = status
			return nil
		},
		Verify: func(in monobank.VerifyWebhookInput) (bool, error) {
			verifyCalls++
			return in.PublicKey == "fresh-key", nil
		},
	}

	payload := map[string]string{
		"invoiceId":    "invoice_123",
		"modifiedDate": "2026-03-30T12:00:00Z",
		"status":       "success",
	}
	req := makeWebhookRequest(t, payload, map[string]string{"x-sign": "valid-signature"})
	rec, _ := runWebhook(t, deps, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if pubkeyCalls != 2 {
		t.Fatalf("pubkey calls: got %d, want 2", pubkeyCalls)
	}
	if verifyCalls != 2 {
		t.Fatalf("verify calls: got %d, want 2", verifyCalls)
	}
	if syncCalls != 1 {
		t.Fatalf("sync calls: got %d, want 1", syncCalls)
	}
	if syncedPayload.InvoiceID != "invoice_123" {
		t.Fatalf("synced invoiceId: got %q, want invoice_123", syncedPayload.InvoiceID)
	}
	if syncedPayload.Status != "success" {
		t.Fatalf("synced status: got %q, want success", syncedPayload.Status)
	}
}

func TestMonobankWebhook_RejectsInvalidSignatureAfterRefresh(t *testing.T) {
	deps := WebhookDeps{
		GetPublicKey: func(context.Context, bool) (string, error) { return "public-key", nil },
		Sync:         func(context.Context, monobank.InvoiceStatusResponse) error { return nil },
		Verify:       func(monobank.VerifyWebhookInput) (bool, error) { return false, nil },
	}
	req := makeWebhookRequest(t,
		map[string]string{"invoiceId": "invoice_123", "status": "processing"},
		map[string]string{"x-sign": "bad-signature"},
	)
	rec, body := runWebhook(t, deps, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if body["error"] != "Invalid webhook signature." {
		t.Fatalf("error: got %v", body["error"])
	}
}
