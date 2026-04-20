package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

type PublicKeyFetcher func(ctx context.Context, forceRefresh bool) (string, error)
type SyncFn func(ctx context.Context, status monobank.InvoiceStatusResponse) error
type VerifyFn func(monobank.VerifyWebhookInput) (bool, error)

type WebhookDeps struct {
	GetPublicKey PublicKeyFetcher
	Sync         SyncFn
	Verify       VerifyFn
}

func defaultWebhookDeps() WebhookDeps {
	client := monobank.NewClient()
	return WebhookDeps{
		GetPublicKey: func(ctx context.Context, force bool) (string, error) {
			return client.PublicKey(ctx, monobank.PublicKeyOptions{ForceRefresh: force})
		},
		Sync:   invoicestore.SyncMonobankPaymentStatus,
		Verify: monobank.VerifyWebhookSignature,
	}
}

func MonobankWebhook(c *gin.Context) {
	handleMonobankWebhook(c, defaultWebhookDeps())
}

func handleMonobankWebhook(c *gin.Context, deps WebhookDeps) {
	signature := strings.TrimSpace(c.GetHeader("x-sign"))
	if signature == "" {
		httpx.Error(c, http.StatusUnauthorized, "X-Sign header is required.")
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
		return
	}
	body := string(raw)
	if strings.TrimSpace(body) == "" {
		httpx.Error(c, http.StatusBadRequest, "Webhook body is required.")
		return
	}

	ctx := c.Request.Context()

	publicKey, err := deps.GetPublicKey(ctx, false)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
		return
	}
	ok, err := deps.Verify(monobank.VerifyWebhookInput{Body: body, PublicKey: publicKey, Signature: signature})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
		return
	}
	if !ok {
		publicKey, err = deps.GetPublicKey(ctx, true)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
			return
		}
		ok, err = deps.Verify(monobank.VerifyWebhookInput{Body: body, PublicKey: publicKey, Signature: signature})
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
			return
		}
	}
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "Invalid webhook signature.")
		return
	}

	var payload monobank.InvoiceStatusResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
		return
	}
	if err := deps.Sync(ctx, payload); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to process Monobank webhook: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
