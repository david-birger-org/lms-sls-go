package monobank

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apexwoot/lms-sls-go/internal/env"
)

const (
	baseURL                    = "https://api.monobank.ua/api/merchant/"
	maxRangeSeconds            = 31 * 24 * 60 * 60
	MaxStatementRangeSeconds   = maxRangeSeconds
	defaultStatementLookback   = 30 * 24 * 60 * 60
)

type SupportedCurrency string

const (
	CurrencyUAH SupportedCurrency = "UAH"
	CurrencyUSD SupportedCurrency = "USD"
)

var currencyCode = map[SupportedCurrency]int{
	CurrencyUAH: 980,
	CurrencyUSD: 840,
}

var currencyByCode = map[int]SupportedCurrency{
	980: CurrencyUAH,
	840: CurrencyUSD,
}

func CurrencyCode(c SupportedCurrency) int { return currencyCode[c] }

func CurrencyFromCode(code *int) *SupportedCurrency {
	if code == nil {
		return nil
	}
	if v, ok := currencyByCode[*code]; ok {
		return &v
	}
	return nil
}

func ToMinorUnits(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

type InvalidStatementRangeError struct{ Message string }

func (e *InvalidStatementRangeError) Error() string { return e.Message }

type StatementRange struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

func ParseStatementRange(q url.Values) (StatementRange, error) {
	now := time.Now().Unix()
	fromParam, errFrom := strconv.ParseInt(q.Get("from"), 10, 64)
	toParam, errTo := strconv.ParseInt(q.Get("to"), 10, 64)
	if errFrom == nil && errTo == nil && fromParam < toParam {
		if toParam-fromParam > maxRangeSeconds {
			return StatementRange{}, &InvalidStatementRangeError{
				Message: fmt.Sprintf("Statement range cannot exceed %d days.", maxRangeSeconds/(24*60*60)),
			}
		}
		return StatementRange{From: fromParam, To: toParam}, nil
	}
	return StatementRange{From: now - defaultStatementLookback, To: now}, nil
}

type StatementItem struct {
	InvoiceID     string `json:"invoiceId,omitempty"`
	Status        string `json:"status,omitempty"`
	MaskedPan     string `json:"maskedPan,omitempty"`
	Date          any    `json:"date,omitempty"`
	PaymentScheme string `json:"paymentScheme,omitempty"`
	Amount        *int64 `json:"amount,omitempty"`
	ProfitAmount  *int64 `json:"profitAmount,omitempty"`
	Ccy           *int   `json:"ccy,omitempty"`
	Rrn           string `json:"rrn,omitempty"`
	Reference     string `json:"reference,omitempty"`
	Destination   string `json:"destination,omitempty"`
}

type statementResponse struct {
	List []StatementItem `json:"list"`
}

type PaymentInfo struct {
	MaskedPan     string  `json:"maskedPan,omitempty"`
	ApprovalCode  string  `json:"approvalCode,omitempty"`
	Rrn           string  `json:"rrn,omitempty"`
	TranID        string  `json:"tranId,omitempty"`
	Terminal      string  `json:"terminal,omitempty"`
	Bank          string  `json:"bank,omitempty"`
	PaymentSystem string  `json:"paymentSystem,omitempty"`
	PaymentMethod string  `json:"paymentMethod,omitempty"`
	Fee           *int64  `json:"fee,omitempty"`
	Country       string  `json:"country,omitempty"`
	AgentFee      *int64  `json:"agentFee,omitempty"`
}

type CancelItem struct {
	Amount       *int64 `json:"amount,omitempty"`
	Ccy          *int   `json:"ccy,omitempty"`
	Date         string `json:"date,omitempty"`
	ApprovalCode string `json:"approvalCode,omitempty"`
	Rrn          string `json:"rrn,omitempty"`
	MaskedPan    string `json:"maskedPan,omitempty"`
}

type InvoiceStatusResponse struct {
	InvoiceID     string       `json:"invoiceId,omitempty"`
	Status        string       `json:"status,omitempty"`
	FailureReason any          `json:"failureReason,omitempty"`
	ErrCode       any          `json:"errCode,omitempty"`
	Amount        *int64       `json:"amount,omitempty"`
	Ccy           *int         `json:"ccy,omitempty"`
	FinalAmount   *int64       `json:"finalAmount,omitempty"`
	CreatedDate   string       `json:"createdDate,omitempty"`
	ModifiedDate  string       `json:"modifiedDate,omitempty"`
	Reference     string       `json:"reference,omitempty"`
	Destination   string       `json:"destination,omitempty"`
	PaymentInfo   *PaymentInfo `json:"paymentInfo,omitempty"`
	CancelList    []CancelItem `json:"cancelList,omitempty"`
}

type InvoiceResponse struct {
	InvoiceID string `json:"invoiceId,omitempty"`
	PageURL   string `json:"pageUrl,omitempty"`
}

type InvoiceRemovalResponse struct {
	InvoiceID string `json:"invoiceId"`
	Status    string `json:"status"`
}

type pubkeyResponse struct {
	Key string `json:"key"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFn    func() (string, error)

	pkMu       sync.Mutex
	cachedKey  string
}

func defaultTokenFn() (string, error) { return env.MonobankToken() }

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		tokenFn:    defaultTokenFn,
	}
}

type requestInput struct {
	Method       string
	Path         string
	SearchParams map[string]string
	Body         any
	Token        string
}

func (c *Client) do(ctx context.Context, in requestInput, out any) error {
	token := in.Token
	if token == "" {
		t, err := c.tokenFn()
		if err != nil {
			return err
		}
		token = t
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	u = u.ResolveReference(&url.URL{Path: in.Path})
	if len(in.SearchParams) > 0 {
		q := u.Query()
		for k, v := range in.SearchParams {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	var body io.Reader
	hasBody := in.Body != nil
	if hasBody {
		buf, err := json.Marshal(in.Body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, in.Method, u.String(), body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Token", token)
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Cache-Control", "no-store")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("monobank request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read monobank response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Monobank API error: %s", string(respBody))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode monobank response: %w", err)
	}
	return nil
}

func (c *Client) FetchStatementChunk(ctx context.Context, token string, from, to int64) ([]StatementItem, error) {
	var resp statementResponse
	err := c.do(ctx, requestInput{
		Method: http.MethodGet,
		Path:   "statement",
		SearchParams: map[string]string{
			"from": strconv.FormatInt(from, 10),
			"to":   strconv.FormatInt(to, 10),
		},
		Token: token,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return resp.List, nil
}

func (c *Client) FetchStatement(ctx context.Context, r StatementRange) ([]StatementItem, error) {
	token, err := c.tokenFn()
	if err != nil {
		return nil, err
	}
	items := make([]StatementItem, 0)
	chunkFrom := r.From
	for chunkFrom < r.To {
		chunkTo := chunkFrom + maxRangeSeconds - 1
		if chunkTo > r.To {
			chunkTo = r.To
		}
		chunk, err := c.FetchStatementChunk(ctx, token, chunkFrom, chunkTo)
		if err != nil {
			return nil, err
		}
		items = append(items, chunk...)
		chunkFrom = chunkTo + 1
	}
	return items, nil
}

type CreateInvoiceInput struct {
	AmountMinor     int64
	Currency        SupportedCurrency
	CustomerName    string
	Description     string
	RedirectURL     string
	Reference       string
	WebhookURL      string
	ValiditySeconds int64
}

type createInvoiceBody struct {
	Amount           int64              `json:"amount"`
	Ccy              int                `json:"ccy"`
	Validity         int64              `json:"validity"`
	MerchantPaymInfo merchantPaymInfo   `json:"merchantPaymInfo"`
	RedirectURL      string             `json:"redirectUrl,omitempty"`
	WebHookURL       string             `json:"webHookUrl,omitempty"`
}

type merchantPaymInfo struct {
	Reference   string `json:"reference"`
	Destination string `json:"destination"`
	Comment     string `json:"comment"`
}

func (c *Client) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (InvoiceResponse, error) {
	body := createInvoiceBody{
		Amount:   in.AmountMinor,
		Ccy:      CurrencyCode(in.Currency),
		Validity: in.ValiditySeconds,
		MerchantPaymInfo: merchantPaymInfo{
			Reference:   in.Reference,
			Destination: in.Description,
			Comment:     fmt.Sprintf("%s: %s", in.CustomerName, in.Description),
		},
		RedirectURL: in.RedirectURL,
		WebHookURL:  in.WebhookURL,
	}
	var resp InvoiceResponse
	if err := c.do(ctx, requestInput{
		Method: http.MethodPost,
		Path:   "invoice/create",
		Body:   body,
	}, &resp); err != nil {
		return InvoiceResponse{}, err
	}
	return resp, nil
}

func (c *Client) RemoveInvoice(ctx context.Context, invoiceID string) (InvoiceRemovalResponse, error) {
	var empty map[string]any
	if err := c.do(ctx, requestInput{
		Method: http.MethodPost,
		Path:   "invoice/remove",
		Body:   map[string]string{"invoiceId": invoiceID},
	}, &empty); err != nil {
		return InvoiceRemovalResponse{}, err
	}
	return InvoiceRemovalResponse{InvoiceID: invoiceID, Status: "cancelled"}, nil
}

func (c *Client) FetchInvoiceStatus(ctx context.Context, invoiceID string) (InvoiceStatusResponse, error) {
	var resp InvoiceStatusResponse
	if err := c.do(ctx, requestInput{
		Method:       http.MethodGet,
		Path:         "invoice/status",
		SearchParams: map[string]string{"invoiceId": invoiceID},
	}, &resp); err != nil {
		return InvoiceStatusResponse{}, err
	}
	return resp, nil
}

type PublicKeyOptions struct {
	ForceRefresh bool
}

func (c *Client) PublicKey(ctx context.Context, opts PublicKeyOptions) (string, error) {
	c.pkMu.Lock()
	cached := c.cachedKey
	c.pkMu.Unlock()
	if !opts.ForceRefresh && cached != "" {
		return cached, nil
	}

	var resp pubkeyResponse
	if err := c.do(ctx, requestInput{
		Method: http.MethodGet,
		Path:   "pubkey",
	}, &resp); err != nil {
		return "", err
	}
	key := strings.TrimSpace(resp.Key)
	if key == "" {
		return "", errors.New("Monobank public key response did not include a key.")
	}
	c.pkMu.Lock()
	c.cachedKey = key
	c.pkMu.Unlock()
	return key, nil
}

type VerifyWebhookInput struct {
	Body      string
	PublicKey string
	Signature string
}

func VerifyWebhookSignature(in VerifyWebhookInput) (bool, error) {
	pkBytes, err := base64.StdEncoding.DecodeString(in.PublicKey)
	if err != nil {
		return false, fmt.Errorf("decode public key: %w", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(in.Signature)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}

	pub, err := parsePublicKey(pkBytes)
	if err != nil {
		return false, err
	}

	digest := sha256.Sum256([]byte(in.Body))

	switch pk := pub.(type) {
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(pk, digest[:], sigBytes), nil
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(pk, 0, digest[:], sigBytes); err != nil {
			return false, nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported public key type %T", pub)
	}
}

func parsePublicKey(raw []byte) (any, error) {
	if block, _ := pem.Decode(raw); block != nil {
		return x509.ParsePKIXPublicKey(block.Bytes)
	}
	return x509.ParsePKIXPublicKey(raw)
}
