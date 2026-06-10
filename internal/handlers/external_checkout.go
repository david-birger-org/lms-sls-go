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

const externalRegistrationSource = "wnbf"

type externalCheckoutBody struct {
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}

func ExternalCheckout(c *gin.Context) {
	ctx := c.Request.Context()

	var body externalCheckoutBody
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("external checkout invalid json", "error", err.Error())
		httpx.Error(c, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	secret, err := env.WnbfCheckoutSecret()
	if err != nil {
		slog.Error("external checkout secret missing", "error", err.Error())
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, rawPayload, err := externalcheckout.Verify(body.Payload, body.Sig, secret)
	if err != nil {
		slog.Warn("external checkout verification failed", "error", err.Error())
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	slog.Info("external checkout verified",
		"external_ref", payload.ExternalRef,
		"product_slug", payload.ProductSlug,
		"amount_minor", payload.AmountMinor,
	)

	product, err := products.SelectBySlug(ctx, payload.ProductSlug)
	if err != nil {
		slog.Error("external checkout product lookup failed",
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if product == nil {
		slog.Warn("external checkout product not found",
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
		)
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	if product.PricingType != products.PricingOnRequest {
		slog.Warn("external checkout product has invalid pricing",
			"external_ref", payload.ExternalRef,
			"product_slug", payload.ProductSlug,
			"pricing_type", product.PricingType,
		)
		httpx.Error(c, http.StatusBadRequest, "External checkout requires an on_request product.")
		return
	}

	idempotencyKey := "wnbf:" + strings.TrimSpace(payload.ExternalRef)
	existing, err := invoicestore.FindPaymentByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		slog.Error("external checkout idempotency lookup failed",
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if existing != nil && existing.InvoiceID != nil && existing.PageURL != nil {
		slog.Info("external checkout reused invoice",
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
		})
		return
	}
	if existing != nil {
		slog.Warn("external checkout existing payment without invoice",
			"external_ref", payload.ExternalRef,
			"payment_id", existing.ID,
			"status", existing.Status,
		)
		httpx.Error(c, http.StatusConflict, "External checkout already exists but has no Monobank invoice yet.")
		return
	}

	customerEmail := strings.TrimSpace(payload.CustomerEmail)
	customerName := strings.TrimSpace(payload.CustomerName)
	description := product.NameUk
	if strings.TrimSpace(description) == "" {
		description = product.NameEn
	}
	productSlug := product.Slug

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
			"external_ref", payload.ExternalRef,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	slog.Info("external checkout pending invoice created",
		"external_ref", payload.ExternalRef,
		"payment_id", pending.PaymentID,
	)

	var rawJSON map[string]any
	if err := json.Unmarshal(rawPayload, &rawJSON); err != nil {
		slog.Error("external checkout raw payload decode failed",
			"external_ref", payload.ExternalRef,
			"payment_id", pending.PaymentID,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to store external registration: "+err.Error())
		return
	}
	if err := externalregistrations.Upsert(ctx, externalregistrations.UpsertInput{
		PaymentID:     pending.PaymentID,
		Source:        externalRegistrationSource,
		ExternalRef:   strings.TrimSpace(payload.ExternalRef),
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		RawPayload:    rawJSON,
	}); err != nil {
		slog.Error("external checkout registration upsert failed",
			"external_ref", payload.ExternalRef,
			"payment_id", pending.PaymentID,
			"error", err.Error(),
		)
		httpx.Error(c, http.StatusInternalServerError, "Failed to store external registration: "+err.Error())
		return
	}
	slog.Info("external checkout registration stored",
		"external_ref", payload.ExternalRef,
		"payment_id", pending.PaymentID,
	)

	result := invoicestore.CreateStoredMonobankInvoice(ctx, invoicestore.CreateStoredMonobankInvoiceInput{
		AmountMinor:   payload.AmountMinor,
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
			"external_ref", payload.ExternalRef,
			"payment_id", pending.PaymentID,
			"status", result.Status,
			"error", result.ErrorMessage,
		)
		httpx.Error(c, result.Status, result.ErrorMessage)
		return
	}
	slog.Info("external checkout monobank invoice created",
		"external_ref", payload.ExternalRef,
		"payment_id", result.Value.PaymentID,
		"invoice_id", result.Value.InvoiceID,
	)
	c.JSON(http.StatusOK, gin.H{
		"expiresAt": result.Value.ExpiresAt,
		"invoiceId": result.Value.InvoiceID,
		"pageUrl":   result.Value.PageURL,
		"paymentId": result.Value.PaymentID,
	})
}
