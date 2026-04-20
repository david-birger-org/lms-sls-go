package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouterWith(provider InternalKeyProvider, captured *Admin) *gin.Engine {
	r := gin.New()
	r.GET("/test", RequireAdminWith(provider), func(c *gin.Context) {
		admin := AdminFrom(c)
		if captured != nil {
			*captured = admin
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRequireAdmin_AllowsTrustedAdmin(t *testing.T) {
	var captured Admin
	r := newRouterWith(func() string { return "internal-key" }, &captured)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderInternalAPIKey, "internal-key")
	req.Header.Set(HeaderAdminUserID, "user_123")
	req.Header.Set(HeaderAdminEmail, "admin@example.com")
	req.Header.Set(HeaderAdminName, "Admin")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if captured.UserID != "user_123" {
		t.Fatalf("userId: got %q, want user_123", captured.UserID)
	}
	if captured.Email == nil || *captured.Email != "admin@example.com" {
		t.Fatalf("email: got %v", captured.Email)
	}
	if captured.Name == nil || *captured.Name != "Admin" {
		t.Fatalf("name: got %v", captured.Name)
	}
	if captured.Role != "admin" {
		t.Fatalf("role: got %q, want admin", captured.Role)
	}
}

func TestRequireAdmin_RejectsMissingInternalKey(t *testing.T) {
	r := newRouterWith(func() string { return "internal-key" }, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderAdminUserID, "user_123")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != "Unauthorized." {
		t.Fatalf("error: got %q, want Unauthorized.", body["error"])
	}
}

func TestRequireAdmin_RejectsMissingAdminHeaders(t *testing.T) {
	r := newRouterWith(func() string { return "internal-key" }, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderInternalAPIKey, "internal-key")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != "Trusted admin headers are missing." {
		t.Fatalf("error: got %q", body["error"])
	}
}
