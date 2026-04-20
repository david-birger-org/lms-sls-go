package invoicestore

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apexwoot/lms-sls-go/internal/monobank"
	"github.com/apexwoot/lms-sls-go/internal/payments"
	"github.com/apexwoot/lms-sls-go/internal/userfeatures"
)

const LectureProductSlugPrefix = "lecture"

func CleanNullableText(value any) *string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		t := strings.TrimSpace(v)
		if t == "" {
			return nil
		}
		return &t
	case *string:
		if v == nil {
			return nil
		}
		return CleanNullableText(*v)
	case int:
		s := fmt.Sprintf("%d", v)
		return &s
	case int64:
		s := fmt.Sprintf("%d", v)
		return &s
	case float64:
		s := fmt.Sprintf("%v", v)
		return &s
	case bool:
		s := fmt.Sprintf("%v", v)
		return &s
	}
	return nil
}

func MirrorAuthUser(ctx context.Context, in MirrorAuthUserInput) (string, error) {
	email := CleanNullableText(stringOrNil(in.Email))
	fullName := CleanNullableText(in.FullName)
	var name string
	if fullName != nil {
		name = *fullName
	} else {
		name = in.AuthUserID
	}
	return MirrorAuthUserToAppUsers(ctx, MirrorAuthUserInput{
		AuthUserID: in.AuthUserID,
		Email:      email,
		FullName:   name,
	})
}

func stringOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func GetAppUserIDByAuthUserID(ctx context.Context, authUserID string) (string, error) {
	id, err := SelectAppUserIDByAuthUserID(ctx, authUserID)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("No app_users row for auth_user_id %s. Better Auth hook should have created it.", authUserID)
	}
	return id, nil
}

func FindPaymentByIdempotencyKey(ctx context.Context, key string) (*IdempotencyPayment, error) {
	return SelectPaymentByIdempotencyKey(ctx, key)
}

func CreatePendingInvoice(ctx context.Context, in CreatePendingInvoiceInput) (PendingInvoiceCreation, error) {
	if in.PaymentID == "" {
		in.PaymentID = uuid.NewString()
	}
	if trimmedName := CleanNullableText(in.CustomerName); trimmedName != nil {
		in.CustomerName = *trimmedName
	} else {
		return PendingInvoiceCreation{}, fmt.Errorf("Customer name is required to create an invoice record.")
	}
	in.CustomerEmail = CleanNullableText(stringOrNil(in.CustomerEmail))
	in.ProductSlug = CleanNullableText(stringOrNil(in.ProductSlug))
	return InsertPendingInvoice(ctx, in)
}

func StoreCreatedInvoice(ctx context.Context, in StoreCreatedInvoiceInput) error {
	return UpdateCreatedInvoice(ctx, in)
}

func MarkInvoiceCreationFailed(ctx context.Context, in MarkInvoiceCreationFailedInput) error {
	if trimmed := CleanNullableText(in.ErrorMessage); trimmed != nil {
		in.ErrorMessage = *trimmed
	}
	return UpdateInvoiceCreationFailed(ctx, in)
}

func ListPendingInvoices(ctx context.Context, limit int) ([]PendingInvoiceRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := SelectPendingPaymentRows(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]PendingInvoiceRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPendingInvoiceRecord(r))
	}
	return out, nil
}

func ListPaymentHistory(ctx context.Context, from, to int64) ([]PaymentHistoryRecord, error) {
	rows, err := SelectPaymentHistoryRows(ctx,
		time.Unix(from, 0).UTC(),
		time.Unix(to, 0).UTC(),
	)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentHistoryRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPaymentHistoryRecord(r))
	}
	return out, nil
}

func GetPaymentDetailsByInvoiceID(ctx context.Context, invoiceID string) (*PaymentDetailsRecord, error) {
	row, err := SelectPaymentHistoryRowByInvoiceID(ctx, invoiceID)
	if err != nil || row == nil {
		return nil, err
	}
	rec := toPaymentDetailsRecord(*row)
	return &rec, nil
}

func MarkInvoiceCancelled(ctx context.Context, invoiceID string, providerPayload any) error {
	return UpdateInvoiceCancelled(ctx, invoiceID, providerPayload)
}

func ListRecentPaymentsByCustomerName(ctx context.Context, name string) ([]PaymentHistoryRecord, error) {
	rows, err := SelectRecentPaymentsByCustomerName(ctx, name)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentHistoryRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPaymentHistoryRecord(r))
	}
	return out, nil
}

func SyncMonobankPaymentStatus(ctx context.Context, status monobank.InvoiceStatusResponse) error {
	invoiceID := CleanNullableText(status.InvoiceID)
	reference := CleanNullableText(status.Reference)
	if invoiceID == nil && reference == nil {
		return nil
	}

	providerStatus := CleanNullableText(status.Status)
	var normalizedStatus *payments.Status
	if providerStatus != nil {
		if v, ok := payments.NormalizeMonobankStatus(*providerStatus); ok {
			normalizedStatus = &v
		}
	}

	providerModifiedAt := parseProviderTimestamp(status.ModifiedDate)
	existingModifiedAt, err := SelectLatestProviderState(ctx, invoiceID, reference)
	if err != nil {
		return err
	}
	if providerModifiedAt != nil && existingModifiedAt != nil && providerModifiedAt.Before(*existingModifiedAt) {
		return nil
	}

	var fee int64
	if status.PaymentInfo != nil && status.PaymentInfo.Fee != nil {
		fee = *status.PaymentInfo.Fee
	}
	var profitAmount *int64
	if status.Amount != nil {
		v := *status.Amount - fee
		profitAmount = &v
	}

	failure := CleanNullableText(status.FailureReason)
	if failure == nil {
		failure = CleanNullableText(status.ErrCode)
	}

	currencyPtr := monobank.CurrencyFromCode(status.Ccy)

	update := ProviderStateUpdateInput{
		AmountMinor:        status.Amount,
		Currency:           currencyPtr,
		FailureReason:      failure,
		ProfitAmountMinor:  profitAmount,
		InvoiceID:          invoiceID,
		PaymentInfo:        status.PaymentInfo,
		ProviderModifiedAt: providerModifiedAt,
		ProviderPayload:    status,
		ProviderStatus:     providerStatus,
		Reference:          reference,
		Status:             normalizedStatus,
	}
	if err := UpdatePaymentProviderState(ctx, update); err != nil {
		return err
	}

	if normalizedStatus != nil && *normalizedStatus == payments.StatusPaid {
		if err := maybeGrantProductFeatures(ctx, invoiceID, reference); err != nil {
			slog.Warn("grant product features failed", "error", err.Error())
		}
	}
	return nil
}

func maybeGrantProductFeatures(ctx context.Context, invoiceID, reference *string) error {
	payment, err := SelectPaymentForFeatureGrant(ctx, invoiceID, reference)
	if err != nil || payment == nil {
		return err
	}
	if payment.ProductSlug == nil || !strings.HasPrefix(*payment.ProductSlug, LectureProductSlugPrefix) {
		return nil
	}
	return userfeatures.GrantByAppUserID(ctx, userfeatures.GrantByAppUserIDInput{
		AppUserID: payment.UserID,
		Feature:   "lectures",
		PaymentID: &payment.ID,
	})
}

func parseProviderTimestamp(value string) *time.Time {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

func toPendingInvoiceRecord(row PendingPaymentRow) PendingInvoiceRecord {
	status := row.Status
	if row.ProviderStatus != nil {
		if v, ok := payments.ResolveMonobankPaymentStatus(row.Status, *row.ProviderStatus); ok {
			status = v
		}
	}
	rec := PendingInvoiceRecord{
		Amount:       row.AmountMinor,
		CreatedDate:  row.CreatedAt,
		Currency:     row.Currency,
		CustomerName: row.CustomerName,
		Description:  row.Description,
		Error:        row.FailureReason,
		ExpiresAt:    row.ExpiresAt,
		InvoiceID:    row.InvoiceID,
		PageURL:      row.PageURL,
		ProductSlug:  row.ProductSlug,
		Reference:    row.Reference,
		Status:       status,
	}
	return rec
}

func toPaymentHistoryRecord(row PaymentHistoryRow) PaymentHistoryRecord {
	var info *monobank.PaymentInfo
	if len(row.PaymentInfoJSON) > 0 {
		var pi monobank.PaymentInfo
		if err := json.Unmarshal(row.PaymentInfoJSON, &pi); err == nil {
			info = &pi
		}
	}
	status := row.Status
	if row.ProviderStatus != nil {
		if v, ok := payments.ResolveMonobankPaymentStatus(row.Status, *row.ProviderStatus); ok {
			status = v
		}
	}
	date := row.CreatedAt
	if row.ProviderModifiedAt != nil {
		date = *row.ProviderModifiedAt
	}
	rec := PaymentHistoryRecord{
		Amount:       row.AmountMinor,
		Ccy:          row.Currency,
		CustomerName: row.CustomerName,
		Date:         date,
		Destination:  row.Description,
		Error:        row.FailureReason,
		ExpiresAt:    row.ExpiresAt,
		InvoiceID:    row.InvoiceID,
		PageURL:      row.PageURL,
		ProductSlug:  row.ProductSlug,
		Reference:    row.Reference,
		Status:       &status,
	}
	if info != nil && info.MaskedPan != "" {
		mp := info.MaskedPan
		rec.MaskedPan = &mp
	}
	return rec
}

func toPaymentDetailsRecord(row PaymentHistoryRow) PaymentDetailsRecord {
	var info *monobank.PaymentInfo
	if len(row.PaymentInfoJSON) > 0 {
		var pi monobank.PaymentInfo
		if err := json.Unmarshal(row.PaymentInfoJSON, &pi); err == nil {
			info = &pi
		}
	}
	status := row.Status
	if row.ProviderStatus != nil {
		if v, ok := payments.ResolveMonobankPaymentStatus(row.Status, *row.ProviderStatus); ok {
			status = v
		}
	}
	rec := PaymentDetailsRecord{
		Amount:        row.AmountMinor,
		CreatedDate:   row.CreatedAt,
		Ccy:           row.Currency,
		CustomerName:  row.CustomerName,
		Destination:   row.Description,
		ExpiresAt:     row.ExpiresAt,
		FailureReason: row.FailureReason,
		ProfitAmount:  row.ProfitAmountMinor,
		InvoiceID:     row.InvoiceID,
		ModifiedDate:  row.ProviderModifiedAt,
		PageURL:       row.PageURL,
		PaymentInfo:   info,
		ProductSlug:   row.ProductSlug,
		Reference:     row.Reference,
		Status:        &status,
	}
	return rec
}
