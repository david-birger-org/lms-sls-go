package payments

import "testing"

func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		in   Status
		want Status
		ok   bool
	}{
		{StatusDraft, StatusDraft, true},
		{StatusCreatingInvoice, StatusCreatingInvoice, true},
		{StatusCreationFailed, StatusCreationFailed, true},
		{StatusInvoiceCreated, StatusInvoiceCreated, true},
		{StatusProcessing, StatusProcessing, true},
		{StatusPaid, StatusPaid, true},
		{StatusFailed, StatusFailed, true},
		{StatusExpired, StatusExpired, true},
		{StatusCancelled, StatusCancelled, true},
		{StatusReversed, StatusReversed, true},
		{" PAID ", StatusPaid, true},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			got, ok := NormalizeStatus(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("NormalizeStatus(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestNormalizeMonobankStatus(t *testing.T) {
	cases := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"created", StatusInvoiceCreated, true},
		{"processing", StatusProcessing, true},
		{"hold", StatusProcessing, true},
		{"success", StatusPaid, true},
		{"failure", StatusFailed, true},
		{"expired", StatusExpired, true},
		{"refunded", StatusReversed, true},
		{"reversed", StatusReversed, true},
		{"cancelled", StatusCancelled, true},
		{" SUCCESS ", StatusPaid, true},
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
		{"terminal local beats stale created provider", StatusCreationFailed, "created", StatusCreationFailed},
		{"maps hold to processing", StatusInvoiceCreated, "hold", StatusProcessing},
		{"terminal local beats stale pending provider", StatusCancelled, "hold", StatusCancelled},
		{"terminal provider beats stale local pending", StatusProcessing, "success", StatusPaid},
		{"terminal provider can reverse paid local status", StatusPaid, "reversed", StatusReversed},
		{"falls back when provider missing", StatusInvoiceCreated, "", StatusInvoiceCreated},
		{"falls back when provider unknown", StatusProcessing, "unknown", StatusProcessing},
		{"rejects unknown local and provider", "unknown", "unknown", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ResolveMonobankPaymentStatus(tc.status, tc.provider)
			wantOK := tc.want != ""
			if ok != wantOK || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, ok, tc.want, wantOK)
			}
		})
	}
}

func TestStatusGroups(t *testing.T) {
	for _, status := range PendingStatuses {
		if !IsPendingStatus(status) {
			t.Fatalf("%q should be pending", status)
		}
		if IsTerminalStatus(status) {
			t.Fatalf("%q should not be terminal", status)
		}
	}
	for _, status := range TerminalStatuses {
		if !IsTerminalStatus(status) {
			t.Fatalf("%q should be terminal", status)
		}
		if IsPendingStatus(status) {
			t.Fatalf("%q should not be pending", status)
		}
	}
}

func TestIsPendingMonobankPayment(t *testing.T) {
	cases := []struct {
		name     string
		status   Status
		provider string
		want     bool
	}{
		{"creation failed beats stale created provider", StatusCreationFailed, "created", false},
		{"provider hold treated as pending", StatusDraft, "hold", true},
		{"cancelled local beats stale hold provider", StatusCancelled, "hold", false},
		{"paid is terminal", StatusPaid, "success", false},
		{"expired is terminal", StatusExpired, "expired", false},
		{"cancelled is terminal", StatusCancelled, "cancelled", false},
		{"creation failed is terminal", StatusCreationFailed, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPendingMonobankPayment(tc.status, tc.provider); got != tc.want {
				t.Fatalf("IsPendingMonobankPayment(%q, %q) = %v, want %v", tc.status, tc.provider, got, tc.want)
			}
		})
	}
}
