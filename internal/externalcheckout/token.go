package externalcheckout

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

const ParticipationProductSlug = "participation-fee"

type Payload struct {
	ProductSlug   string `json:"productSlug"`
	AmountMinor   int64  `json:"amountMinor"`
	Currency      string `json:"currency"`
	CustomerName  string `json:"customerName"`
	CustomerEmail string `json:"customerEmail"`
	ExternalRef   string `json:"externalRef"`
	ReturnURL     string `json:"returnUrl"`
	ExpiresAt     int64  `json:"exp"`
	Nonce         string `json:"nonce"`
}

func Verify(payloadValue, signature, secret string) (Payload, []byte, error) {
	payloadValue = strings.TrimSpace(payloadValue)
	signature = strings.TrimSpace(signature)
	if payloadValue == "" || signature == "" {
		return Payload{}, nil, errors.New("payload and sig are required")
	}
	if strings.TrimSpace(secret) == "" {
		return Payload{}, nil, errors.New("WNBF_CHECKOUT_SECRET is not configured")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadValue))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return Payload{}, nil, errors.New("invalid checkout signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(payloadValue)
	if err != nil {
		return Payload{}, nil, fmt.Errorf("decode payload: %w", err)
	}
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Payload{}, nil, fmt.Errorf("decode payload json: %w", err)
	}
	if err := payload.Validate(time.Now()); err != nil {
		return Payload{}, nil, err
	}
	return payload, raw, nil
}

func (p Payload) Validate(now time.Time) error {
	if strings.TrimSpace(p.ProductSlug) != ParticipationProductSlug {
		return fmt.Errorf("productSlug must be %s", ParticipationProductSlug)
	}
	if p.AmountMinor <= 0 {
		return errors.New("amountMinor must be greater than 0")
	}
	if strings.TrimSpace(p.Currency) != string(monobank.CurrencyUAH) {
		return errors.New("currency must be UAH")
	}
	if strings.TrimSpace(p.CustomerName) == "" {
		return errors.New("customerName is required")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(p.CustomerEmail)); err != nil {
		return errors.New("customerEmail must be a valid email")
	}
	if strings.TrimSpace(p.ExternalRef) == "" {
		return errors.New("externalRef is required")
	}
	if strings.TrimSpace(p.Nonce) == "" {
		return errors.New("nonce is required")
	}
	if p.ExpiresAt <= now.Unix() {
		return errors.New("checkout payload expired")
	}
	return validateReturnURL(p.ReturnURL)
}

func validateReturnURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return errors.New("returnUrl must be an absolute URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("returnUrl must use http or https")
	}
	return nil
}
