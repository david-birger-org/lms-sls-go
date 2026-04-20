package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/invoicestore"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func PaymentsHistory(c *gin.Context) {
	ctx := c.Request.Context()
	invoiceID := strings.TrimSpace(c.Query("invoiceId"))
	if invoiceID != "" {
		details, err := invoicestore.GetPaymentDetailsByInvoiceID(ctx, invoiceID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to load payment details: "+err.Error())
			return
		}
		if details == nil {
			httpx.Error(c, http.StatusNotFound, "Payment not found.")
			return
		}
		c.JSON(http.StatusOK, details)
		return
	}

	customer := strings.TrimSpace(c.Query("customerName"))
	if customer != "" {
		list, err := invoicestore.ListRecentPaymentsByCustomerName(ctx, customer)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to load payment history: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"list": list})
		return
	}

	r, err := monobank.ParseStatementRange(c.Request.URL.Query())
	if err != nil {
		var rangeErr *monobank.InvalidStatementRangeError
		if errors.As(err, &rangeErr) {
			httpx.Error(c, http.StatusBadRequest, rangeErr.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "Failed to load payment history: "+err.Error())
		return
	}
	list, err := invoicestore.ListPaymentHistory(ctx, r.From, r.To)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to load payment history: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}
