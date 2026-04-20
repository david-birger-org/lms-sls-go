package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
)

const (
	HeaderInternalAPIKey = "x-internal-api-key"
	HeaderAdminUserID    = "x-admin-user-id"
	HeaderAdminEmail     = "x-admin-email"
	HeaderAdminName      = "x-admin-name"
	HeaderUserID         = "x-user-id"
	HeaderUserEmail      = "x-user-email"
	HeaderUserName       = "x-user-name"
	HeaderUserRole       = "x-user-role"

	ctxAdmin = "lms.admin"
	ctxUser  = "lms.user"
)

type Admin struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Name   *string `json:"name"`
	Role   string  `json:"role"`
}

type User struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Name   *string `json:"name"`
	Role   string  `json:"role"`
}

func trimmedHeader(c *gin.Context, name string) string {
	return strings.TrimSpace(c.GetHeader(name))
}

func optionalHeader(c *gin.Context, name string) *string {
	v := trimmedHeader(c, name)
	if v == "" {
		return nil
	}
	return &v
}

type InternalKeyProvider func() string

func DefaultKeyProvider() string {
	key, _ := env.InternalAPIKey()
	return key
}

func RequireAdminWith(provider InternalKeyProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := trimmedHeader(c, HeaderInternalAPIKey)
		expected := provider()
		if key == "" || key != expected {
			httpx.Error(c, http.StatusUnauthorized, "Unauthorized.")
			c.Abort()
			return
		}
		userID := trimmedHeader(c, HeaderAdminUserID)
		if userID == "" {
			httpx.Error(c, http.StatusBadRequest, "Trusted admin headers are missing.")
			c.Abort()
			return
		}
		admin := Admin{
			UserID: userID,
			Email:  optionalHeader(c, HeaderAdminEmail),
			Name:   optionalHeader(c, HeaderAdminName),
			Role:   "admin",
		}
		c.Set(ctxAdmin, admin)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return RequireAdminWith(DefaultKeyProvider)
}

func RequireUserWith(provider InternalKeyProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := trimmedHeader(c, HeaderInternalAPIKey)
		expected := provider()
		if key == "" || key != expected {
			httpx.Error(c, http.StatusUnauthorized, "Unauthorized.")
			c.Abort()
			return
		}
		userID := trimmedHeader(c, HeaderUserID)
		if userID == "" {
			httpx.Error(c, http.StatusBadRequest, "Trusted user headers are missing.")
			c.Abort()
			return
		}
		role := trimmedHeader(c, HeaderUserRole)
		if role == "" {
			role = "user"
		}
		user := User{
			UserID: userID,
			Email:  optionalHeader(c, HeaderUserEmail),
			Name:   optionalHeader(c, HeaderUserName),
			Role:   role,
		}
		c.Set(ctxUser, user)
		c.Next()
	}
}

func RequireUser() gin.HandlerFunc {
	return RequireUserWith(DefaultKeyProvider)
}

func AdminFrom(c *gin.Context) Admin {
	v, _ := c.Get(ctxAdmin)
	a, _ := v.(Admin)
	return a
}

func UserFrom(c *gin.Context) User {
	v, _ := c.Get(ctxUser)
	u, _ := v.(User)
	return u
}
