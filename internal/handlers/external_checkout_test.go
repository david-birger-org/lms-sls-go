package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func withHandlerTestLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func assertHandlerLogContains(t *testing.T, logs string, parts ...string) {
	t.Helper()

	for _, part := range parts {
		if !strings.Contains(logs, part) {
			t.Fatalf("logs missing %q:\n%s", part, logs)
		}
	}
}

func TestExternalCheckoutTestLogsRequestBeforeTokenLookup(t *testing.T) {
	t.Setenv("MONOBANK_TEST_TOKEN", "")
	logs := withHandlerTestLogger(t)

	r := gin.New()
	r.POST("/api/external/checkout/test", ExternalCheckoutTest)

	req := httptest.NewRequest(http.MethodPost, "/api/external/checkout/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != "MONOBANK_TEST_TOKEN is missing in environment variables." {
		t.Fatalf("error: got %q", body["error"])
	}

	assertHandlerLogContains(t, logs.String(),
		`"msg":"external checkout request received"`,
		`"mode":"test"`,
		`"test":true`,
		`"method":"POST"`,
		`"path":"/api/external/checkout/test"`,
		`"msg":"external checkout test token missing"`,
		`"error":"MONOBANK_TEST_TOKEN is missing in environment variables."`,
	)
}
