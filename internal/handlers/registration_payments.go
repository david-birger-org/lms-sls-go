package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/registrationpayments"
)

func RegistrationPaymentsList(c *gin.Context) {
	rows, err := registrationpayments.SelectAll(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch registration payments: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": rows})
}
