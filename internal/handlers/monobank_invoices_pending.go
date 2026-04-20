package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func MonobankInvoicesPending(c *gin.Context) {
	ctx := c.Request.Context()
	invoiceID := strings.TrimSpace(c.Query("invoiceId"))

	if invoiceID != "" {
		client := monobank.NewClient()
		status, err := client.FetchInvoiceStatus(ctx, invoiceID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to load payment details: "+err.Error())
			return
		}
		if err := invoicestore.SyncMonobankPaymentStatus(ctx, status); err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to load payment details: "+err.Error())
			return
		}
		payment, err := invoicestore.GetPaymentDetailsByInvoiceID(ctx, invoiceID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to load payment details: "+err.Error())
			return
		}
		if payment != nil {
			c.JSON(http.StatusOK, payment)
			return
		}
		c.JSON(http.StatusOK, status)
		return
	}

	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = max(1, min(n, 100))
		}
	}
	list, err := invoicestore.ListPendingInvoices(ctx, limit)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to load pending invoices: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}
