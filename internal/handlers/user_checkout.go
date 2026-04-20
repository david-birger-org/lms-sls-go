package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/products"
)

type checkoutBody struct {
	ProductSlug string `json:"productSlug"`
	Currency    string `json:"currency"`
	RedirectURL string `json:"redirectUrl"`
}

func UserCheckout(c *gin.Context) {
	user := auth.UserFrom(c)
	ctx := c.Request.Context()

	var body checkoutBody
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Request body must be valid JSON.")
		return
	}
	productSlug := strings.TrimSpace(body.ProductSlug)
	if productSlug == "" {
		httpx.Error(c, http.StatusBadRequest, "productSlug is required.")
		return
	}
	if body.Currency != string(monobank.CurrencyUAH) && body.Currency != string(monobank.CurrencyUSD) {
		httpx.Error(c, http.StatusBadRequest, "currency must be 'UAH' or 'USD'.")
		return
	}
	currency := monobank.SupportedCurrency(body.Currency)
	redirect := strings.TrimSpace(body.RedirectURL)

	product, err := products.SelectBySlug(ctx, productSlug)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}
	if product == nil || !product.Active {
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	if product.PricingType != products.PricingFixed {
		httpx.Error(c, http.StatusBadRequest, "This product is not available for direct checkout.")
		return
	}

	var amountPtr *int64
	switch currency {
	case monobank.CurrencyUAH:
		amountPtr = product.PriceUahMinor
	case monobank.CurrencyUSD:
		amountPtr = product.PriceUsdMinor
	}
	if amountPtr == nil || *amountPtr <= 0 {
		httpx.Error(c, http.StatusBadRequest, "Product has no price set for "+string(currency)+".")
		return
	}
	amountMinor := *amountPtr

	customerName := ""
	if user.Name != nil {
		customerName = strings.TrimSpace(*user.Name)
	}
	if customerName == "" && user.Email != nil {
		email := *user.Email
		if at := strings.Index(email, "@"); at > 0 {
			customerName = email[:at]
		}
	}
	if customerName == "" {
		customerName = "Customer"
	}
	var customerEmail *string
	if user.Email != nil {
		customerEmail = user.Email
	}

	appUserID, err := invoicestore.GetAppUserIDByAuthUserID(ctx, user.UserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}

	pending, err := invoicestore.CreatePendingInvoice(ctx, invoicestore.CreatePendingInvoiceInput{
		AmountMinor:   amountMinor,
		Currency:      currency,
		CustomerEmail: customerEmail,
		CustomerName:  customerName,
		Description:   product.NameEn,
		ProductID:     &product.ID,
		ProductSlug:   &product.Slug,
		UserID:        &appUserID,
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create checkout: "+err.Error())
		return
	}

	result := invoicestore.CreateStoredMonobankInvoice(ctx, invoicestore.CreateStoredMonobankInvoiceInput{
		AmountMinor:     amountMinor,
		Currency:        currency,
		CustomerName:    customerName,
		Description:     product.NameEn,
		PendingInvoice:  pending,
		RedirectURL:     redirect,
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
