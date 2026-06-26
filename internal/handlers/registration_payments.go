package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/fiscalchecksync"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/registrationpayments"
)

type registrationPaymentDeps struct {
	DeletePayment     func(context.Context, string) error
	FindFiscalPayment func(context.Context, string) (fiscalchecksync.MissingPayment, error)
	SyncFiscalChecks  func(context.Context, fiscalchecksync.MissingPayment) (fiscalchecksync.Result, error)
}

func defaultRegistrationPaymentDeps() registrationPaymentDeps {
	store := fiscalchecksync.DBStore{}
	client := monobank.NewClient()
	return registrationPaymentDeps{
		DeletePayment:     registrationpayments.Delete,
		FindFiscalPayment: store.FindRegistrationPayment,
		SyncFiscalChecks: func(ctx context.Context, payment fiscalchecksync.MissingPayment) (fiscalchecksync.Result, error) {
			return fiscalchecksync.SyncPayment(ctx, store, client, payment)
		},
	}
}

func RegistrationPaymentsList(c *gin.Context) {
	rows, err := registrationpayments.SelectAll(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch registration payments: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": rows})
}

func RegistrationPaymentDelete(c *gin.Context) {
	handleRegistrationPaymentDelete(c, defaultRegistrationPaymentDeps())
}

func RegistrationPaymentFiscalCheckSync(c *gin.Context) {
	handleRegistrationPaymentFiscalCheckSync(c, defaultRegistrationPaymentDeps())
}

func handleRegistrationPaymentDelete(c *gin.Context, deps registrationPaymentDeps) {
	paymentID := strings.TrimSpace(c.Param("paymentID"))
	if paymentID == "" {
		httpx.Error(c, http.StatusBadRequest, "paymentId is required.")
		return
	}

	err := deps.DeletePayment(c.Request.Context(), paymentID)
	if errors.Is(err, registrationpayments.ErrNotFound) {
		httpx.Error(c, http.StatusNotFound, "Registration payment was not found.")
		return
	}
	if errors.Is(err, registrationpayments.ErrPaymentFinalized) {
		httpx.Error(c, http.StatusConflict, "Paid or processing registration payments cannot be deleted.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to delete registration payment: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func handleRegistrationPaymentFiscalCheckSync(c *gin.Context, deps registrationPaymentDeps) {
	paymentID := strings.TrimSpace(c.Param("paymentID"))
	if paymentID == "" {
		httpx.Error(c, http.StatusBadRequest, "paymentId is required.")
		return
	}

	ctx := c.Request.Context()
	payment, err := deps.FindFiscalPayment(ctx, paymentID)
	if errors.Is(err, fiscalchecksync.ErrNotFound) {
		httpx.Error(c, http.StatusNotFound, "Registration payment was not found.")
		return
	}
	if errors.Is(err, fiscalchecksync.ErrMissingInvoice) {
		httpx.Error(c, http.StatusBadRequest, "Registration payment does not have a Monobank invoice.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to load registration payment: "+err.Error())
		return
	}

	result, err := deps.SyncFiscalChecks(ctx, payment)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to sync fiscal checks: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}
