package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/externalcheckout"
	"github.com/apexwoot/lms-sls-go/internal/externalregistrations"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/products"
)

const (
	externalRegistrationSource     = "wnbf"
	externalTestRegistrationSource = "wnbf-test"
	participationFeeProductSlug    = "participation-fee"
	participationFeeDescription    = "Оплата участі в спортивному заході"
	duplicateParticipationMessage  = "This participant already has an active participation payment."
)

type externalCheckoutBody struct {
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}

type externalCheckoutMode struct {
	Client            *monobank.Client
	IdempotencyPrefix string
	Name              string
	Source            string
	Test              bool
}

func externalCheckoutDescription(product products.Row) string {
	if product.Slug == participationFeeProductSlug {
		return participationFeeDescription
	}
	description := product.NameUk
	if strings.TrimSpace(description) == "" {
		description = product.NameEn
	}
	return description
}

func externalCheckoutCustomerPhone(raw map[string]any) string {
	for _, key := range []string{"customerPhone", "phone", "billingPhone"} {
		value, ok := raw[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ExternalCheckout(c *gin.Context) {
	mode := externalCheckoutMode{
		IdempotencyPrefix: "wnbf:",
		Name:              "production",
		Source:            externalRegistrationSource,
	}
	logExternalCheckoutRequest(c, mode)
	handleExternalCheckout(c, mode)
}

func ExternalCheckoutTest(c *gin.Context) {
	mode := externalCheckoutMode{
		IdempotencyPrefix: "wnbf-test:",
		Name:              "test",
		Source:            externalTestRegistrationSource,
		Test:              true,
	}
	logExternalCheckoutRequest(c, mode)

	token, err := env.MonobankTestToken()
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "external checkout test token missing",
			"mode", mode.Name,
			"test", mode.Test,
			"path", c.Request.URL.Path,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	mode.Client = monobank.NewClientWithToken(token)
	handleExternalCheckout(c, mode)
}

func logExternalCheckoutRequest(c *gin.Context, mode externalCheckoutMode) {
	slog.InfoContext(c.Request.Context(), "external checkout request received",
		"mode", mode.Name,
		"test", mode.Test,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"route", c.FullPath(),
		"client_ip", c.ClientIP(),
		"host", c.Request.Host,
		"user_agent", c.Request.UserAgent(),
	)
}

func handleExternalCheckout(c *gin.Context, mode externalCheckoutMode) {
	ctx := c.Request.Context()

	var body externalCheckoutBody
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("external checkout invalid json", "mode", mode.Name, "error", err.Error())
		httpx.Error(c, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	secret, err := env.WnbfCheckoutSecret()
	if err != nil {
		slog.Error("external checkout secret missing", "mode", mode.Name, "error", err.Error())
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, rawPayload, err := externalcheckout.Verify(body.Payload, body.Sig, secret)
	if err != nil {
		slog.Warn("external checkout verification failed", "mode", mode.Name, "error", err.Error())
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	slog.Info("external checkout verified",
		"mode", mode.Name,
		"external_ref", payload.ExternalRef,
		"product_slug", payload.ProductSlug,
		"amount_minor", payload.AmountMinor,
	)

	product, err := products.SelectBySlug(ctx, payload.ProductSlug)
	if err != nil {
		slog.Error("external checkout product lookup failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if product == nil {
		slog.Warn("external checkout product not found",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
		)
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	if product.PricingType != products.PricingOnRequest {
		slog.Warn("external checkout product has invalid pricing",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
			"pricing_type", product.PricingType,
		)
		httpx.Error(c, http.StatusBadRequest, "External checkout requires an on_request product.")
		return
	}

	idempotencyKey := mode.IdempotencyPrefix + strings.TrimSpace(payload.ExternalRef)
	existing, err := invoicestore.FindPaymentByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		slog.Error("external checkout idempotency lookup failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if existing != nil && existing.InvoiceID != nil && existing.PageURL != nil {
		slog.Info("external checkout reused invoice",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"payment_id", existing.ID,
			"invoice_id", *existing.InvoiceID,
			"status", existing.Status,
		)
		c.JSON(http.StatusOK, gin.H{
			"expiresAt": existing.ExpiresAt,
			"invoiceId": *existing.InvoiceID,
			"pageUrl":   *existing.PageURL,
			"paymentId": existing.ID,
			"reused":    true,
			"test":      mode.Test,
		})
		return
	}
	if existing != nil {
		slog.Warn("external checkout existing payment without invoice",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"payment_id", existing.ID,
			"status", existing.Status,
		)
		httpx.Error(c, http.StatusConflict, "External checkout already exists but has no Monobank invoice yet.")
		return
	}

	customerEmail := strings.TrimSpace(payload.CustomerEmail)
	customerName := strings.TrimSpace(payload.CustomerName)
	description := externalCheckoutDescription(*product)
	productSlug := product.Slug

	var rawJSON map[string]any
	if err := json.Unmarshal(rawPayload, &rawJSON); err != nil {
		slog.Error("external checkout raw payload decode failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to decode external registration: "+err.Error())
		return
	}

	duplicate, err := externalregistrations.FindActiveDuplicate(ctx, externalregistrations.ActiveDuplicateInput{
		CustomerEmail: customerEmail,
		CustomerName:  customerName,
		CustomerPhone: externalCheckoutCustomerPhone(rawJSON),
		ExternalRef:   strings.TrimSpace(payload.ExternalRef),
		ProductSlug:   productSlug,
		Source:        mode.Source,
	})
	if err != nil {
		slog.Error("external checkout duplicate lookup failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if duplicate != nil {
		slog.Warn("external checkout duplicate participant blocked",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"duplicate_external_ref", duplicate.ExternalRef,
			"duplicate_payment_id", duplicate.PaymentID,
			"match_field", duplicate.MatchField,
			"status", duplicate.Status,
		)
		httpx.Error(c, http.StatusConflict, duplicateParticipationMessage)
		return
	}

	pending, err := invoicestore.CreatePendingInvoice(ctx, invoicestore.CreatePendingInvoiceInput{
		AmountMinor:    payload.AmountMinor,
		Currency:       monobank.CurrencyUAH,
		CustomerEmail:  &customerEmail,
		CustomerName:   customerName,
		Description:    description,
		IdempotencyKey: &idempotencyKey,
		ProductID:      &product.ID,
		ProductSlug:    &productSlug,
	})
	if err != nil {
		slog.Error("external checkout pending invoice insert failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	slog.Info("external checkout pending invoice created",
		"mode", mode.Name,
		"external_ref", payload.ExternalRef,
		"payment_id", pending.PaymentID,
	)

	if err := externalregistrations.Upsert(ctx, externalregistrations.UpsertInput{
		PaymentID:     pending.PaymentID,
		Source:        mode.Source,
		ExternalRef:   strings.TrimSpace(payload.ExternalRef),
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		RawPayload:    rawJSON,
	}); err != nil {
		slog.Error("external checkout registration upsert failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"payment_id", pending.PaymentID,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to store external registration: "+err.Error())
		return
	}
	slog.Info("external checkout registration stored",
		"mode", mode.Name,
		"external_ref", payload.ExternalRef,
		"payment_id", pending.PaymentID,
	)

	result := invoicestore.CreateStoredMonobankInvoice(ctx, invoicestore.CreateStoredMonobankInvoiceInput{
		AmountMinor:   payload.AmountMinor,
		Client:        mode.Client,
		Currency:      monobank.CurrencyUAH,
		CustomerEmail: &customerEmail,
		CustomerName:  customerName,
		Description:   description,
		FiscalItems: []monobank.FiscalizationItem{
			{
				Name:  description,
				Qty:   1,
				Sum:   payload.AmountMinor,
				Total: payload.AmountMinor,
				Unit:  "послуга",
				Code:  productSlug,
			},
		},
		PendingInvoice:  pending,
		RedirectURL:     strings.TrimSpace(payload.ReturnURL),
		RequestURL:      requestAbsoluteURL(c),
		ValiditySeconds: defaultInvoiceValiditySeconds,
	})
	if !result.OK {
		slog.Error("external checkout monobank invoice create failed",
			"mode", mode.Name,
			"external_ref", payload.ExternalRef,
			"payment_id", pending.PaymentID,
			"status", result.Status,
			"error", result.ErrorMessage,
		)
		httpx.Error(c, result.Status, result.ErrorMessage)
		return
	}
	slog.Info("external checkout monobank invoice created",
		"mode", mode.Name,
		"external_ref", payload.ExternalRef,
		"payment_id", result.Value.PaymentID,
		"invoice_id", result.Value.InvoiceID,
	)
	c.JSON(http.StatusOK, gin.H{
		"expiresAt": result.Value.ExpiresAt,
		"invoiceId": result.Value.InvoiceID,
		"pageUrl":   result.Value.PageURL,
		"paymentId": result.Value.PaymentID,
		"test":      mode.Test,
	})
}
