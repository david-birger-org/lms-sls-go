package monobank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateInvoiceSendsFiscalBasketAndCustomerEmail(t *testing.T) {
	var got createInvoiceBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/merchant/invoice/create" {
			t.Fatalf("path: got %s, want /api/merchant/invoice/create", r.URL.Path)
		}
		if r.Header.Get("X-Token") != "token" {
			t.Fatalf("X-Token: got %q, want token", r.Header.Get("X-Token"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoiceId":"invoice-123","pageUrl":"https://pay.example/invoice-123"}`))
	}))
	defer server.Close()

	email := " customer@example.com "
	client := NewClientWithToken("token")
	client.baseURL = server.URL + "/api/merchant/"

	_, err := client.CreateInvoice(context.Background(), CreateInvoiceInput{
		AmountMinor:     12345,
		Currency:        CurrencyUAH,
		CustomerEmail:   &email,
		CustomerName:    "Customer",
		Description:     "Participation fee",
		Reference:       "payment-ref",
		ValiditySeconds: 24 * 60 * 60,
	})
	if err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}

	if got.Amount != 12345 {
		t.Fatalf("amount: got %d, want 12345", got.Amount)
	}
	if got.Ccy != 980 {
		t.Fatalf("ccy: got %d, want 980", got.Ccy)
	}
	if got.MerchantPaymInfo.Reference != "payment-ref" {
		t.Fatalf("reference: got %q, want payment-ref", got.MerchantPaymInfo.Reference)
	}
	if got.MerchantPaymInfo.Destination != "Participation fee" {
		t.Fatalf("destination: got %q, want Participation fee", got.MerchantPaymInfo.Destination)
	}
	if len(got.MerchantPaymInfo.CustomerEmails) != 1 || got.MerchantPaymInfo.CustomerEmails[0] != "customer@example.com" {
		t.Fatalf("customerEmails: got %#v, want customer@example.com", got.MerchantPaymInfo.CustomerEmails)
	}
	if len(got.MerchantPaymInfo.BasketOrder) != 1 {
		t.Fatalf("basketOrder length: got %d, want 1", len(got.MerchantPaymInfo.BasketOrder))
	}
	item := got.MerchantPaymInfo.BasketOrder[0]
	if item.Name != "Participation fee" || item.Qty != 1 || item.Sum != 12345 || item.Total != 12345 || item.Unit != "послуга" || item.Code != defaultFiscalItemCode {
		t.Fatalf("basketOrder item: got %#v", item)
	}
}

func TestCreateInvoiceOmitsInvalidCustomerEmail(t *testing.T) {
	var got createInvoiceBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoiceId":"invoice-123","pageUrl":"https://pay.example/invoice-123"}`))
	}))
	defer server.Close()

	email := "n/a"
	client := NewClientWithToken("token")
	client.baseURL = server.URL + "/api/merchant/"

	_, err := client.CreateInvoice(context.Background(), CreateInvoiceInput{
		AmountMinor:     12345,
		Currency:        CurrencyUAH,
		CustomerEmail:   &email,
		CustomerName:    "Customer",
		Description:     "Participation fee",
		Reference:       "payment-ref",
		ValiditySeconds: 24 * 60 * 60,
	})
	if err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}
	if len(got.MerchantPaymInfo.CustomerEmails) != 0 {
		t.Fatalf("customerEmails: got %#v, want none", got.MerchantPaymInfo.CustomerEmails)
	}
}
