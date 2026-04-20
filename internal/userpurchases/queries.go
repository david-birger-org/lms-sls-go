package userpurchases

import (
	"context"
	"time"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type Row struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	AmountMinor       int64     `json:"amountMinor"`
	ProfitAmountMinor *int64    `json:"profitAmountMinor"`
	Currency          string    `json:"currency"`
	Description       string    `json:"description"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	ProductID         *string   `json:"productId"`
	ProductSlug       *string   `json:"productSlug"`
	ProductNameUk     *string   `json:"productNameUk"`
	ProductNameEn     *string   `json:"productNameEn"`
	ProductImageURL   *string   `json:"productImageUrl"`
}

type Query struct {
	From  time.Time
	Limit int
	To    time.Time
}

func scanRows(ctx context.Context, sql string, args ...any) ([]Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Row, 0)
	for rows.Next() {
		var r Row
		if err := rows.Scan(
			&r.ID,
			&r.Status,
			&r.AmountMinor,
			&r.ProfitAmountMinor,
			&r.Currency,
			&r.Description,
			&r.CreatedAt,
			&r.UpdatedAt,
			&r.ProductID,
			&r.ProductSlug,
			&r.ProductNameUk,
			&r.ProductNameEn,
			&r.ProductImageURL,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func SelectUserPurchases(ctx context.Context, authUserID string, q Query) ([]Row, error) {
	return scanRows(ctx, `
		select
			p.id,
			p.status,
			p.amount_minor,
			p.profit_amount_minor,
			p.currency,
			p.description,
			p.created_at,
			p.updated_at,
			p.product_id,
			pr.slug as product_slug,
			pr.name_uk as product_name_uk,
			pr.name_en as product_name_en,
			pr.image_url as product_image_url
		from payments p
		inner join app_users au on au.id = p.user_id
		left join products pr on pr.id = p.product_id
		where au.auth_user_id = $1
		  and p.created_at >= $2
		  and p.created_at <= $3
		order by p.created_at desc
		limit $4
	`, authUserID, q.From, q.To, q.Limit)
}

func SelectInvoicesCreatedByAdmin(ctx context.Context, authUserID string, q Query) ([]Row, error) {
	return scanRows(ctx, `
		select
			p.id,
			p.status,
			p.amount_minor,
			p.profit_amount_minor,
			p.currency,
			p.description,
			p.created_at,
			p.updated_at,
			p.product_id,
			pr.slug as product_slug,
			pr.name_uk as product_name_uk,
			pr.name_en as product_name_en,
			pr.image_url as product_image_url
		from payments p
		inner join app_users au on au.id = p.created_by_admin_user_id
		left join products pr on pr.id = p.product_id
		where au.auth_user_id = $1
		  and p.created_at >= $2
		  and p.created_at <= $3
		order by p.created_at desc
		limit $4
	`, authUserID, q.From, q.To, q.Limit)
}
