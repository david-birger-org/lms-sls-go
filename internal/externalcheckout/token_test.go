package externalcheckout

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodePayload(t *testing.T, p Payload) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func validPayload() Payload {
	return Payload{
		ProductSlug:   ParticipationProductSlug,
		AmountMinor:   250000,
		Currency:      "UAH",
		CustomerName:  "Ivan Ivanov",
		CustomerEmail: "ivan@example.com",
		ExternalRef:   "wnbf-2026-123",
		ReturnURL:     "https://wnbfukraine.com.ua/payment-result",
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		Nonce:         "nonce-123",
	}
}

func TestVerifyAcceptsSignedPayload(t *testing.T) {
	secret := "secret"
	payload := encodePayload(t, validPayload())

	got, _, err := Verify(payload, sign(payload, secret), secret)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if got.ProductSlug != ParticipationProductSlug {
		t.Fatalf("productSlug: got %q", got.ProductSlug)
	}
	if got.AmountMinor != 250000 {
		t.Fatalf("amountMinor: got %d", got.AmountMinor)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	payload := encodePayload(t, validPayload())

	if _, _, err := Verify(payload, "bad", "secret"); err == nil {
		t.Fatal("Verify accepted a bad signature")
	}
}

func TestVerifyRejectsExpiredPayload(t *testing.T) {
	p := validPayload()
	p.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	payload := encodePayload(t, p)

	if _, _, err := Verify(payload, sign(payload, "secret"), "secret"); err == nil {
		t.Fatal("Verify accepted an expired payload")
	}
}
