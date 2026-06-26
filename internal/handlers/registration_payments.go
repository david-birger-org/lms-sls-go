package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/registrationpayments"
)

type registrationPaymentDeps struct {
	DeletePayment func(context.Context, string) error
}

func defaultRegistrationPaymentDeps() registrationPaymentDeps {
	return registrationPaymentDeps{
		DeletePayment: registrationpayments.Delete,
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
