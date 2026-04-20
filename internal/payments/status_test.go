package payments

import "testing"

func TestNormalizeMonobankStatus(t *testing.T) {
	cases := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"created", StatusInvoiceCreated, true},
		{"success", StatusPaid, true},
		{"failure", StatusFailed, true},
		{"expired", StatusExpired, true},
		{"refunded", StatusReversed, true},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := NormalizeMonobankStatus(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("NormalizeMonobankStatus(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestResolveMonobankPaymentStatus(t *testing.T) {
	cases := []struct {
		name     string
		status   Status
		provider string
		want     Status
	}{
		{"prefers recognized provider status", StatusCreationFailed, "created", StatusInvoiceCreated},
		{"maps hold to processing", StatusInvoiceCreated, "hold", StatusProcessing},
		{"falls back when provider missing", StatusInvoiceCreated, "", StatusInvoiceCreated},
		{"falls back when provider unknown", StatusProcessing, "unknown", StatusProcessing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ResolveMonobankPaymentStatus(tc.status, tc.provider)
			if !ok || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, true)", got, ok, tc.want)
			}
		})
	}
}

func TestIsPendingMonobankPayment(t *testing.T) {
	cases := []struct {
		name     string
		status   Status
		provider string
		want     bool
	}{
		{"provider created beats stored drift", StatusCreationFailed, "created", true},
		{"provider hold treated as pending", StatusDraft, "hold", true},
		{"paid is terminal", StatusPaid, "success", false},
		{"expired is terminal", StatusExpired, "expired", false},
		{"cancelled is terminal", StatusCancelled, "cancelled", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPendingMonobankPayment(tc.status, tc.provider); got != tc.want {
				t.Fatalf("IsPendingMonobankPayment(%q, %q) = %v, want %v", tc.status, tc.provider, got, tc.want)
			}
		})
	}
}
