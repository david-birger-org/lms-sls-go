package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/env"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/mailer"
)

type mailPayload struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	ReplyTo string `json:"replyTo"`
}

func MailTransactional(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader(auth.HeaderInternalAPIKey))
	expected, _ := env.InternalAPIKey()
	if key == "" || key != expected {
		httpx.Error(c, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var body mailPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid payload.")
		return
	}
	subject := strings.TrimSpace(body.Subject)
	text := strings.TrimSpace(body.Text)
	if subject == "" || text == "" {
		httpx.Error(c, http.StatusBadRequest, "Invalid payload.")
		return
	}

	result := mailer.SendTransactional(mailer.Input{
		Subject: subject,
		Text:    text,
		ReplyTo: strings.TrimSpace(body.ReplyTo),
	})

	if result.OK {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	switch result.Reason {
	case mailer.ReasonMissingConfig, mailer.ReasonMissingDestination:
		httpx.Error(c, http.StatusInternalServerError, "Email provider is not configured.")
		return
	}
	slog.Error("transactional mail send failed", "error", result.Err)
	httpx.Error(c, http.StatusBadGateway, "Failed to send mail.")
}
