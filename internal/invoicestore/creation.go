package invoicestore

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

type CreateStoredMonobankInvoiceInput struct {
	AmountMinor     int64
	Currency        monobank.SupportedCurrency
	CustomerName    string
	Description     string
	PendingInvoice  PendingInvoiceCreation
	RedirectURL     string
	RequestURL      string
	ValiditySeconds int64
	Client          *monobank.Client
}

type CreatedStoredMonobankInvoice struct {
	ExpiresAt time.Time
	InvoiceID string
	PageURL   string
	PaymentID string
}

type CreateStoredMonobankInvoiceResult struct {
	OK           bool
	Value        CreatedStoredMonobankInvoice
	ErrorMessage string
	Status       int
}

func webhookURLFromRequest(requestURL string) string {
	u, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	u.Path = "/api/monobank/webhook"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func CreateStoredMonobankInvoice(ctx context.Context, in CreateStoredMonobankInvoiceInput) CreateStoredMonobankInvoiceResult {
	client := in.Client
	if client == nil {
		client = monobank.NewClient()
	}

	invoice, err := client.CreateInvoice(ctx, monobank.CreateInvoiceInput{
		AmountMinor:     in.AmountMinor,
		Currency:        in.Currency,
		CustomerName:    in.CustomerName,
		Description:     in.Description,
		RedirectURL:     in.RedirectURL,
		Reference:       in.PendingInvoice.Reference,
		ValiditySeconds: in.ValiditySeconds,
		WebhookURL:      webhookURLFromRequest(in.RequestURL),
	})
	if err != nil {
		msg := err.Error()
		persistErr := MarkInvoiceCreationFailed(ctx, MarkInvoiceCreationFailedInput{
			ErrorMessage: msg,
			PaymentID:    in.PendingInvoice.PaymentID,
		})
		if persistErr != nil {
			slog.Warn("mark invoice creation failed", "error", persistErr.Error())
		}
		return CreateStoredMonobankInvoiceResult{
			OK:           false,
			ErrorMessage: msg,
			Status:       502,
		}
	}

	invoiceID := strings.TrimSpace(invoice.InvoiceID)
	pageURL := strings.TrimSpace(invoice.PageURL)
	if invoiceID == "" || pageURL == "" {
		msg := "Monobank response did not include invoiceId or pageUrl."
		persistErr := MarkInvoiceCreationFailed(ctx, MarkInvoiceCreationFailedInput{
			ErrorMessage:    msg,
			PaymentID:       in.PendingInvoice.PaymentID,
			ProviderPayload: invoice,
		})
		if persistErr != nil {
			slog.Warn("mark invoice creation failed", "error", persistErr.Error())
		}
		return CreateStoredMonobankInvoiceResult{
			OK:           false,
			ErrorMessage: msg,
			Status:       502,
		}
	}

	expiresAt := time.Now().UTC().Add(time.Duration(in.ValiditySeconds) * time.Second)
	if err := StoreCreatedInvoice(ctx, StoreCreatedInvoiceInput{
		ExpiresAt:       expiresAt,
		InvoiceID:       invoiceID,
		PageURL:         pageURL,
		PaymentID:       in.PendingInvoice.PaymentID,
		ProviderPayload: invoice,
	}); err != nil {
		return CreateStoredMonobankInvoiceResult{
			OK:           false,
			ErrorMessage: err.Error(),
			Status:       502,
		}
	}

	return CreateStoredMonobankInvoiceResult{
		OK: true,
		Value: CreatedStoredMonobankInvoice{
			ExpiresAt: expiresAt,
			InvoiceID: invoiceID,
			PageURL:   pageURL,
			PaymentID: in.PendingInvoice.PaymentID,
		},
	}
}
