package payments

import "strings"

type Status string

const (
	StatusDraft           Status = "draft"
	StatusCreatingInvoice Status = "creating_invoice"
	StatusCreationFailed  Status = "creation_failed"
	StatusInvoiceCreated  Status = "invoice_created"
	StatusProcessing      Status = "processing"
	StatusPaid            Status = "paid"
	StatusFailed          Status = "failed"
	StatusExpired         Status = "expired"
	StatusCancelled       Status = "cancelled"
	StatusReversed        Status = "reversed"
)

var PendingStatuses = [2]Status{StatusInvoiceCreated, StatusProcessing}

var PendingMonobankProviderStatuses = [3]string{"created", "processing", "hold"}

var pendingSet = map[Status]struct{}{
	StatusInvoiceCreated: {},
	StatusProcessing:     {},
}

var monobankStatusMap = map[string]Status{
	"created":   StatusInvoiceCreated,
	"expired":   StatusExpired,
	"failure":   StatusFailed,
	"hold":      StatusProcessing,
	"processing": StatusProcessing,
	"refunded":  StatusReversed,
	"reversed":  StatusReversed,
	"success":   StatusPaid,
	"cancelled": StatusCancelled,
}

func NormalizeMonobankStatus(status string) (Status, bool) {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return "", false
	}
	v, ok := monobankStatusMap[s]
	return v, ok
}

func ResolveMonobankPaymentStatus(status Status, providerStatus string) (Status, bool) {
	if v, ok := NormalizeMonobankStatus(providerStatus); ok {
		return v, true
	}
	if status != "" {
		return status, true
	}
	return "", false
}

func IsPendingMonobankPayment(status Status, providerStatus string) bool {
	resolved, ok := ResolveMonobankPaymentStatus(status, providerStatus)
	if !ok {
		return false
	}
	_, pending := pendingSet[resolved]
	return pending
}
