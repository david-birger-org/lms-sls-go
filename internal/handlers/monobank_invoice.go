package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

const defaultInvoiceValiditySeconds = 24 * 60 * 60

type createInvoiceBody struct {
	Amount          json.Number `json:"amount"`
	Currency        string      `json:"currency"`
	CustomerEmail   string      `json:"customerEmail"`
	CustomerName    string      `json:"customerName"`
	Description     string      `json:"description"`
	RedirectURL     string      `json:"redirectUrl"`
	ValiditySeconds json.Number `json:"validitySeconds"`
}

type parsedCreateInvoice struct {
	AmountMinor     int64
	Currency        monobank.SupportedCurrency
	CustomerEmail   string
	CustomerName    string
	Description     string
	RedirectURL     string
	ValiditySeconds int64
}

func parseCreateInvoiceBody(body createInvoiceBody) (parsedCreateInvoice, string) {
	amount, err := body.Amount.Float64()
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return parsedCreateInvoice{}, "Amount must be greater than 0."
	}
	if body.Currency != string(monobank.CurrencyUAH) && body.Currency != string(monobank.CurrencyUSD) {
		return parsedCreateInvoice{}, "Currency must be UAH or USD."
	}
	customerName := strings.TrimSpace(body.CustomerName)
	if customerName == "" {
		return parsedCreateInvoice{}, "Customer name is required."
	}
	description := strings.TrimSpace(body.Description)
	if description == "" {
		return parsedCreateInvoice{}, "Description is required."
	}
	validity := int64(defaultInvoiceValiditySeconds)
	if body.ValiditySeconds != "" {
		v, err := body.ValiditySeconds.Int64()
		if err != nil || v < 60 {
			return parsedCreateInvoice{}, "Expiration time must be at least 60 seconds."
		}
		validity = v
	}
	return parsedCreateInvoice{
		AmountMinor:     monobank.ToMinorUnits(amount),
		Currency:        monobank.SupportedCurrency(body.Currency),
		CustomerEmail:   strings.TrimSpace(body.CustomerEmail),
		CustomerName:    customerName,
		Description:     description,
		RedirectURL:     strings.TrimSpace(body.RedirectURL),
		ValiditySeconds: validity,
	}, ""
}

func MonobankInvoiceCreate(c *gin.Context) {
	admin := auth.AdminFrom(c)
	ctx := c.Request.Context()

	var body createInvoiceBody
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	parsed, badReason := parseCreateInvoiceBody(body)
	if badReason != "" {
		httpx.Error(c, http.StatusBadRequest, badReason)
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader("idempotency-key"))
	if idempotencyKey != "" {
		existing, err := invoicestore.FindPaymentByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to create invoice: "+err.Error())
			return
		}
		if existing != nil && existing.InvoiceID != nil && existing.PageURL != nil {
			c.JSON(http.StatusOK, gin.H{
				"expiresAt": existing.ExpiresAt,
				"invoiceId": *existing.InvoiceID,
				"pageUrl":   *existing.PageURL,
				"paymentId": existing.ID,
			})
			return
		}
	}

	createdBy, err := invoicestore.GetAppUserIDByAuthUserID(ctx, admin.UserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create invoice: "+err.Error())
		return
	}

	var customerEmail *string
	if parsed.CustomerEmail != "" {
		e := parsed.CustomerEmail
		customerEmail = &e
	}

	var idemKeyPtr *string
	if idempotencyKey != "" {
		idemKeyPtr = &idempotencyKey
	}

	pending, err := invoicestore.CreatePendingInvoice(ctx, invoicestore.CreatePendingInvoiceInput{
		AmountMinor:          parsed.AmountMinor,
		CreatedByAdminUserID: &createdBy,
		Currency:             parsed.Currency,
		CustomerEmail:        customerEmail,
		CustomerName:         parsed.CustomerName,
		Description:          parsed.Description,
		IdempotencyKey:       idemKeyPtr,
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create invoice: "+err.Error())
		return
	}

	result := invoicestore.CreateStoredMonobankInvoice(ctx, invoicestore.CreateStoredMonobankInvoiceInput{
		AmountMinor:     parsed.AmountMinor,
		Currency:        parsed.Currency,
		CustomerName:    parsed.CustomerName,
		Description:     parsed.Description,
		PendingInvoice:  pending,
		RedirectURL:     parsed.RedirectURL,
		RequestURL:      requestAbsoluteURL(c),
		ValiditySeconds: parsed.ValiditySeconds,
	})
	if !result.OK {
		httpx.Error(c, result.Status, result.ErrorMessage)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"expiresAt": result.Value.ExpiresAt,
		"invoiceId": result.Value.InvoiceID,
		"pageUrl":   result.Value.PageURL,
		"paymentId": result.Value.PaymentID,
	})
}

func requestAbsoluteURL(c *gin.Context) string {
	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("x-forwarded-proto") == "" {
		scheme = "http"
	}
	if proto := c.GetHeader("x-forwarded-proto"); proto != "" {
		scheme = proto
	}
	host := c.GetHeader("x-forwarded-host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + c.Request.URL.RequestURI()
}

type removeInvoiceBody struct {
	InvoiceID string `json:"invoiceId"`
}

var expiredInvoicePattern = regexp.MustCompile(`(?i)"errCode"\s*:\s*"INVOICE_EXPIRED"`)

func MonobankInvoiceDelete(c *gin.Context) {
	var body removeInvoiceBody
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to cancel invoice: "+err.Error())
		return
	}
	invoiceID := strings.TrimSpace(body.InvoiceID)
	if invoiceID == "" {
		httpx.Error(c, http.StatusBadRequest, "invoiceId is required.")
		return
	}

	ctx := c.Request.Context()
	client := monobank.NewClient()

	removal, err := client.RemoveInvoice(ctx, invoiceID)
	if err == nil {
		if persistErr := invoicestore.MarkInvoiceCancelled(ctx, invoiceID, removal); persistErr != nil {
			slog.Warn("mark invoice cancelled", "error", persistErr.Error())
		}
		c.JSON(http.StatusOK, removal)
		return
	}

	if !expiredInvoicePattern.MatchString(err.Error()) {
		httpx.Error(c, http.StatusInternalServerError, "Failed to cancel invoice: "+err.Error())
		return
	}

	status, statusErr := client.FetchInvoiceStatus(ctx, invoiceID)
	if statusErr != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to cancel invoice: "+errors.Join(err, statusErr).Error())
		return
	}
	if syncErr := invoicestore.SyncMonobankPaymentStatus(ctx, status); syncErr != nil {
		slog.Warn("sync monobank payment status", "error", syncErr.Error())
	}
	c.JSON(http.StatusOK, status)
}
