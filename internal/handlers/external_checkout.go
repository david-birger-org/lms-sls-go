package handlers

import (
	"encoding/json"
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
		httpx.Error(c, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	secret, err := env.WnbfCheckoutSecret()
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload, rawPayload, err := externalcheckout.Verify(body.Payload, body.Sig, secret)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	product, err := products.SelectBySlug(ctx, payload.ProductSlug)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if product == nil {
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	if product.PricingType != products.PricingOnRequest {
		httpx.Error(c, http.StatusBadRequest, "External checkout requires an on_request product.")
		return
	}

	idempotencyKey := "wnbf:" + strings.TrimSpace(payload.ExternalRef)
	existing, err := invoicestore.FindPaymentByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if existing != nil && existing.InvoiceID != nil && existing.PageURL != nil {
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
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}

	var rawJSON map[string]any
	if err := json.Unmarshal(rawPayload, &rawJSON); err != nil {
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
		httpx.Error(c, http.StatusInternalServerError, "Failed to store external registration: "+err.Error())
		return
	}

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
