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

var TerminalStatuses = [6]Status{
	StatusCreationFailed,
	StatusPaid,
	StatusFailed,
	StatusExpired,
	StatusCancelled,
	StatusReversed,
}

const (
	MonobankProviderStatusCreated    = "created"
	MonobankProviderStatusProcessing = "processing"
	MonobankProviderStatusHold       = "hold"
	MonobankProviderStatusSuccess    = "success"
	MonobankProviderStatusFailure    = "failure"
	MonobankProviderStatusExpired    = "expired"
	MonobankProviderStatusCancelled  = "cancelled"
	MonobankProviderStatusReversed   = "reversed"
	MonobankProviderStatusRefunded   = "refunded"
)

var PendingMonobankProviderStatuses = [3]string{
	MonobankProviderStatusCreated,
	MonobankProviderStatusProcessing,
	MonobankProviderStatusHold,
}

var statusSet = map[Status]struct{}{
	StatusDraft:           {},
	StatusCreatingInvoice: {},
	StatusCreationFailed:  {},
	StatusInvoiceCreated:  {},
	StatusProcessing:      {},
	StatusPaid:            {},
	StatusFailed:          {},
	StatusExpired:         {},
	StatusCancelled:       {},
	StatusReversed:        {},
}

var pendingSet = map[Status]struct{}{
	StatusInvoiceCreated: {},
	StatusProcessing:     {},
}

var terminalSet = map[Status]struct{}{
	StatusCreationFailed: {},
	StatusPaid:           {},
	StatusFailed:         {},
	StatusExpired:        {},
	StatusCancelled:      {},
	StatusReversed:       {},
}

var monobankStatusMap = map[string]Status{
	MonobankProviderStatusCreated:    StatusInvoiceCreated,
	MonobankProviderStatusExpired:    StatusExpired,
	MonobankProviderStatusFailure:    StatusFailed,
	MonobankProviderStatusHold:       StatusProcessing,
	MonobankProviderStatusProcessing: StatusProcessing,
	MonobankProviderStatusRefunded:   StatusReversed,
	MonobankProviderStatusReversed:   StatusReversed,
	MonobankProviderStatusSuccess:    StatusPaid,
	MonobankProviderStatusCancelled:  StatusCancelled,
}

func NormalizeStatus(status Status) (Status, bool) {
	s := Status(strings.ToLower(strings.TrimSpace(string(status))))
	if s == "" {
		return "", false
	}
	_, ok := statusSet[s]
	if !ok {
		return "", false
	}
	return s, true
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
	stored, storedOK := NormalizeStatus(status)
	if v, ok := NormalizeMonobankStatus(providerStatus); ok {
		if storedOK && IsTerminalStatus(stored) && IsPendingStatus(v) {
			return stored, true
		}
		return v, true
	}
	if storedOK {
		return stored, true
	}
	return "", false
}

func IsPendingStatus(status Status) bool {
	normalized, ok := NormalizeStatus(status)
	if !ok {
		return false
	}
	_, pending := pendingSet[normalized]
	return pending
}

func IsTerminalStatus(status Status) bool {
	normalized, ok := NormalizeStatus(status)
	if !ok {
		return false
	}
	_, terminal := terminalSet[normalized]
	return terminal
}

func IsPendingMonobankPayment(status Status, providerStatus string) bool {
	resolved, ok := ResolveMonobankPaymentStatus(status, providerStatus)
	if !ok {
		return false
	}
	return IsPendingStatus(resolved)
}
