package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/userfeatures"
)

type internalAppUserBody struct {
	Action     string `json:"action"`
	AuthUserID string `json:"authUserId"`
	Email      string `json:"email"`
	Feature    string `json:"feature"`
	FullName   string `json:"fullName"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func InternalAppUsersUpsert(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader(auth.HeaderInternalAPIKey))
	expected, _ := env.InternalAPIKey()
	if key == "" || key != expected {
		httpx.Error(c, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var body internalAppUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid JSON payload.")
		return
	}
	action := strings.TrimSpace(body.Action)

	switch action {
	case "grant-feature", "revoke-feature":
		handleFeatureAction(c, body, action)
		return
	}
	handleUpsert(c, body)
}

func handleUpsert(c *gin.Context, body internalAppUserBody) {
	authUserID := strings.TrimSpace(body.AuthUserID)
	if authUserID == "" {
		httpx.Error(c, http.StatusBadRequest, "authUserId is required.")
		return
	}

	var email *string
	if v := strings.TrimSpace(body.Email); v != "" {
		email = &v
	}

	fullName := strings.TrimSpace(body.FullName)
	if fullName == "" {
		if email != nil {
			if at := strings.Index(*email, "@"); at > 0 {
				fullName = (*email)[:at]
			}
		}
	}
	if fullName == "" {
		fullName = authUserID
	}

	appUserID, err := invoicestore.MirrorAuthUser(c.Request.Context(), invoicestore.MirrorAuthUserInput{
		AuthUserID: authUserID,
		Email:      email,
		FullName:   fullName,
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process request: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"appUserId": appUserID})
}

func handleFeatureAction(c *gin.Context, body internalAppUserBody, action string) {
	adminUserID := strings.TrimSpace(c.GetHeader(auth.HeaderAdminUserID))
	if adminUserID == "" {
		httpx.Error(c, http.StatusBadRequest, "Trusted admin headers are missing.")
		return
	}

	authUserID := strings.TrimSpace(body.AuthUserID)
	if authUserID == "" {
		httpx.Error(c, http.StatusBadRequest, "authUserId is required.")
		return
	}
	feature := strings.TrimSpace(body.Feature)
	if feature == "" {
		httpx.Error(c, http.StatusBadRequest, "feature is required.")
		return
	}

	grantedBy, err := invoicestore.GetAppUserIDByAuthUserID(c.Request.Context(), adminUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process request: "+err.Error())
		return
	}

	if action == "grant-feature" {
		err = userfeatures.Grant(c.Request.Context(), userfeatures.GrantInput{
			AuthUserID:         authUserID,
			Feature:            feature,
			GrantedByAppUserID: &grantedBy,
		})
	} else {
		err = userfeatures.Revoke(c.Request.Context(), userfeatures.RevokeInput{
			AuthUserID: authUserID,
			Feature:    feature,
		})
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process request: "+err.Error())
		return
	}

	features, err := userfeatures.SelectActiveFeatures(c.Request.Context(), authUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process request: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"features": features})
}
