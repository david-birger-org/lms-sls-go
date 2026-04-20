package invoicestore

import (
	"time"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/payments"
)

type PendingPaymentRow struct {
	AmountMinor    int64
	CreatedAt      time.Time
	Currency       monobank.SupportedCurrency
	CustomerName   string
	Description    string
	ExpiresAt      *time.Time
	FailureReason  *string
	InvoiceID      string
	PageURL        *string
	ProductSlug    *string
	ProviderStatus *string
	Reference      string
	Status         payments.Status
}

type PaymentHistoryRow struct {
	AmountMinor       int64
	CreatedAt         time.Time
	Currency          monobank.SupportedCurrency
	CustomerName      string
	Description       string
	ExpiresAt         *time.Time
	FailureReason     *string
	ProfitAmountMinor *int64
	InvoiceID         *string
	PageURL           *string
	PaymentInfoJSON   []byte
	ProductSlug       *string
	ProviderModifiedAt *time.Time
	ProviderStatus    *string
	Reference         string
	Status            payments.Status
}

type PendingInvoiceCreation struct {
	PaymentID string `json:"paymentId"`
	Reference string `json:"reference"`
}

type PendingInvoiceRecord struct {
	Amount       int64                      `json:"amount"`
	CreatedDate  time.Time                  `json:"createdDate"`
	Currency     monobank.SupportedCurrency `json:"currency"`
	CustomerName string                     `json:"customerName"`
	Description  string                     `json:"description"`
	Error        *string                    `json:"error,omitempty"`
	ExpiresAt    *time.Time                 `json:"expiresAt,omitempty"`
	InvoiceID    string                     `json:"invoiceId"`
	PageURL      *string                    `json:"pageUrl,omitempty"`
	ProductSlug  *string                    `json:"productSlug,omitempty"`
	Reference    string                     `json:"reference"`
	Status       payments.Status            `json:"status"`
}

type PaymentHistoryRecord struct {
	Amount       int64                      `json:"amount"`
	Ccy          monobank.SupportedCurrency `json:"ccy"`
	CustomerName string                     `json:"customerName"`
	Date         time.Time                  `json:"date"`
	Destination  string                     `json:"destination"`
	Error        *string                    `json:"error,omitempty"`
	ExpiresAt    *time.Time                 `json:"expiresAt,omitempty"`
	InvoiceID    *string                    `json:"invoiceId,omitempty"`
	MaskedPan    *string                    `json:"maskedPan,omitempty"`
	PageURL      *string                    `json:"pageUrl,omitempty"`
	ProductSlug  *string                    `json:"productSlug,omitempty"`
	Reference    string                     `json:"reference"`
	Status       *payments.Status           `json:"status,omitempty"`
}

type PaymentDetailsRecord struct {
	Amount        int64                      `json:"amount"`
	CreatedDate   time.Time                  `json:"createdDate"`
	Ccy           monobank.SupportedCurrency `json:"ccy"`
	CustomerName  string                     `json:"customerName"`
	Destination   string                     `json:"destination"`
	ExpiresAt     *time.Time                 `json:"expiresAt,omitempty"`
	FailureReason *string                    `json:"failureReason,omitempty"`
	ProfitAmount  *int64                     `json:"profitAmount,omitempty"`
	InvoiceID     *string                    `json:"invoiceId,omitempty"`
	ModifiedDate  *time.Time                 `json:"modifiedDate,omitempty"`
	PageURL       *string                    `json:"pageUrl,omitempty"`
	PaymentInfo   *monobank.PaymentInfo      `json:"paymentInfo,omitempty"`
	ProductSlug   *string                    `json:"productSlug,omitempty"`
	Reference     string                     `json:"reference"`
	Status        *payments.Status           `json:"status,omitempty"`
}

type MirrorAuthUserInput struct {
	AuthUserID string
	Email      *string
	FullName   string
}

type CreatePendingInvoiceInput struct {
	AmountMinor          int64
	CreatedByAdminUserID *string
	Currency             monobank.SupportedCurrency
	CustomerEmail        *string
	CustomerName         string
	Description          string
	PaymentID            string
	ProductID            *string
	ProductSlug          *string
	UserID               *string
	IdempotencyKey       *string
}

type StoreCreatedInvoiceInput struct {
	ExpiresAt       time.Time
	InvoiceID       string
	PageURL         string
	PaymentID       string
	ProviderPayload any
}

type MarkInvoiceCreationFailedInput struct {
	ErrorMessage    string
	PaymentID       string
	ProviderPayload any
}

type ProviderStateUpdateInput struct {
	AmountMinor       *int64
	Currency          *monobank.SupportedCurrency
	FailureReason     *string
	ProfitAmountMinor *int64
	InvoiceID         *string
	PaymentInfo       *monobank.PaymentInfo
	ProviderModifiedAt *time.Time
	ProviderPayload   any
	ProviderStatus    *string
	Reference         *string
	Status            *payments.Status
}

type PaymentFeatureGrantRow struct {
	ID          string
	UserID      string
	ProductSlug *string
}

type IdempotencyPayment struct {
	ID        string
	InvoiceID *string
	PageURL   *string
	ExpiresAt *time.Time
	Status    payments.Status
}
