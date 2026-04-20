package invoicestore

import "testing"

func TestCleanNullableText(t *testing.T) {
	t.Run("trims string values", func(t *testing.T) {
		got := CleanNullableText("  invoice_123  ")
		if got == nil || *got != "invoice_123" {
			t.Fatalf("got %v, want invoice_123", got)
		}
	})

	t.Run("stringifies primitives", func(t *testing.T) {
		if got := CleanNullableText(101); got == nil || *got != "101" {
			t.Fatalf("int: got %v, want 101", got)
		}
		if got := CleanNullableText(false); got == nil || *got != "false" {
			t.Fatalf("bool: got %v, want false", got)
		}
	})

	t.Run("nil for empty and unsupported", func(t *testing.T) {
		if got := CleanNullableText("   "); got != nil {
			t.Fatalf("empty string: got %v, want nil", *got)
		}
		if got := CleanNullableText(map[string]string{"errCode": "INVOICE_EXPIRED"}); got != nil {
			t.Fatalf("map: got %v, want nil", *got)
		}
		if got := CleanNullableText(nil); got != nil {
			t.Fatalf("nil: got %v, want nil", *got)
		}
	})
}
