package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
)

func withTestLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, expected string) {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != expected {
		t.Fatalf("error: got %q, want %q", body["error"], expected)
	}
}

func assertLogContains(t *testing.T, logs string, parts ...string) {
	t.Helper()

	for _, part := range parts {
		if !strings.Contains(logs, part) {
			t.Fatalf("logs missing %q:\n%s", part, logs)
		}
	}
}

func TestNewRouterLogsMissingCheckoutRoute(t *testing.T) {
	logs := withTestLogger(t)
	router := newRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/checkout/external", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	assertJSONError(t, rec, "Not found.")
	assertLogContains(t, logs.String(),
		`"msg":"http request"`,
		`"method":"GET"`,
		`"path":"/api/checkout/external"`,
		`"status":404`,
		`"error":"Not found."`,
	)
}

func TestNewRouterLogsMethodMismatch(t *testing.T) {
	logs := withTestLogger(t)
	router := newRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/external/checkout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want 405", rec.Code)
	}
	assertJSONError(t, rec, "Method not allowed.")
	assertLogContains(t, logs.String(),
		`"msg":"http request"`,
		`"method":"GET"`,
		`"path":"/api/external/checkout"`,
		`"status":405`,
		`"error":"Method not allowed."`,
	)
}

func TestNewRouterLogsAuthRejection(t *testing.T) {
	logs := withTestLogger(t)
	router := newRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/external/checkout", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	assertJSONError(t, rec, "Unauthorized.")
	assertLogContains(t, logs.String(),
		`"msg":"request rejected by auth"`,
		`"auth":"internal_key"`,
		`"reason":"missing_or_invalid_internal_key"`,
		`"path":"/api/external/checkout"`,
		`"has_internal_key":false`,
		`"msg":"http request"`,
		`"status":401`,
		`"error":"Unauthorized."`,
	)
}

func TestRecoveryLogsPanicAndRequestError(t *testing.T) {
	logs := withTestLogger(t)
	router := gin.New()
	router.Use(requestLogger())
	router.Use(recoveryLogger())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	assertJSONError(t, rec, "Internal server error.")
	assertLogContains(t, logs.String(),
		`"msg":"panic recovered"`,
		`"panic":"boom"`,
		`"path":"/panic"`,
		`"stack":`,
		`"msg":"http request"`,
		`"status":500`,
		`"error":"Internal server error."`,
	)
}

func TestRequestLoggerIncludesHTTPXError(t *testing.T) {
	logs := withTestLogger(t)
	router := gin.New()
	router.Use(requestLogger())
	router.GET("/fail", func(c *gin.Context) {
		httpx.Error(c, http.StatusTeapot, "short and stout")
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d, want 418", rec.Code)
	}
	assertJSONError(t, rec, "short and stout")
	assertLogContains(t, logs.String(),
		`"msg":"http request"`,
		`"path":"/fail"`,
		`"status":418`,
		`"error":"short and stout"`,
	)
}
