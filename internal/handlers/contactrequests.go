package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/contactrequests"
	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
)

type contactRequestPayload struct {
	RequestType            any `json:"requestType"`
	FirstName              any `json:"firstName"`
	LastName               any `json:"lastName"`
	Email                  any `json:"email"`
	Country                any `json:"country"`
	Phone                  any `json:"phone"`
	PreferredContactMethod any `json:"preferredContactMethod"`
	Social                 any `json:"social"`
	Message                any `json:"message"`
	Service                any `json:"service"`
}

func normalizeOptionalString(v any) *string {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}

func parseContactRequestType(v any) (contactrequests.Type, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	switch contactrequests.Type(s) {
	case contactrequests.TypeContact, contactrequests.TypeService:
		return contactrequests.Type(s), true
	}
	return "", false
}

func toContactRequestRecord(r contactrequests.Row) gin.H {
	return gin.H{
		"id":                     r.ID,
		"requestType":            r.RequestType,
		"firstName":              r.FirstName,
		"lastName":               r.LastName,
		"email":                  r.Email,
		"country":                r.Country,
		"phone":                  r.Phone,
		"preferredContactMethod": r.PreferredContactMethod,
		"social":                 r.Social,
		"message":                r.Message,
		"service":                r.Service,
		"processed":              r.Processed,
		"processedAt":            r.ProcessedAt,
		"processedBy":            r.ProcessedBy,
		"createdAt":              r.CreatedAt,
		"updatedAt":              r.UpdatedAt,
	}
}

func ContactRequestsCreate(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader(auth.HeaderInternalAPIKey))
	expected, _ := env.InternalAPIKey()
	if key == "" || key != expected {
		httpx.Error(c, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var payload contactRequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid payload. requestType must be 'contact' or 'service'.")
		return
	}
	requestType, ok := parseContactRequestType(payload.RequestType)
	if !ok {
		httpx.Error(c, http.StatusBadRequest, "Invalid payload. requestType must be 'contact' or 'service'.")
		return
	}

	row, err := contactrequests.Insert(c.Request.Context(), contactrequests.CreateInput{
		RequestType:            requestType,
		FirstName:              normalizeOptionalString(payload.FirstName),
		LastName:               normalizeOptionalString(payload.LastName),
		Email:                  normalizeOptionalString(payload.Email),
		Country:                normalizeOptionalString(payload.Country),
		Phone:                  normalizeOptionalString(payload.Phone),
		PreferredContactMethod: normalizeOptionalString(payload.PreferredContactMethod),
		Social:                 normalizeOptionalString(payload.Social),
		Message:                normalizeOptionalString(payload.Message),
		Service:                normalizeOptionalString(payload.Service),
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to save contact request: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"request": toContactRequestRecord(*row)})
}

func ContactRequestsAdminList(c *gin.Context) {
	rows, err := contactrequests.SelectAll(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch contact requests: "+err.Error())
		return
	}
	records := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		records = append(records, toContactRequestRecord(r))
	}
	c.JSON(http.StatusOK, gin.H{"requests": records})
}

type contactRequestUpdatePayload struct {
	Processed *bool `json:"processed"`
}

func ContactRequestsAdminUpdate(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		httpx.Error(c, http.StatusBadRequest, "Missing contact request id.")
		return
	}

	var payload contactRequestUpdatePayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Processed == nil {
		httpx.Error(c, http.StatusBadRequest, "processed must be a boolean.")
		return
	}

	admin := auth.AdminFrom(c)
	var processedBy *string
	if *payload.Processed {
		id, err := invoicestore.GetAppUserIDByAuthUserID(c.Request.Context(), admin.UserID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to update contact request: "+err.Error())
			return
		}
		processedBy = &id
	}

	row, err := contactrequests.UpdateProcessed(c.Request.Context(), contactrequests.UpdateProcessedInput{
		ID:          id,
		Processed:   *payload.Processed,
		ProcessedBy: processedBy,
	})
	if errors.Is(err, nil) && row == nil {
		httpx.Error(c, http.StatusNotFound, "Contact request not found.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to update contact request: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"request": toContactRequestRecord(*row)})
}
