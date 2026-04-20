package products

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type PricingType string

const (
	PricingFixed     PricingType = "fixed"
	PricingOnRequest PricingType = "on_request"
)

type Row struct {
	ID            string      `json:"id"`
	Slug          string      `json:"slug"`
	NameUk        string      `json:"nameUk"`
	NameEn        string      `json:"nameEn"`
	DescriptionUk *string     `json:"descriptionUk"`
	DescriptionEn *string     `json:"descriptionEn"`
	PricingType   PricingType `json:"pricingType"`
	PriceUahMinor *int64      `json:"priceUahMinor"`
	PriceUsdMinor *int64      `json:"priceUsdMinor"`
	ImageURL      *string     `json:"imageUrl"`
	Active        bool        `json:"active"`
	SortOrder     int         `json:"sortOrder"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

type CreateInput struct {
	Slug          string
	NameUk        string
	NameEn        string
	DescriptionUk *string
	DescriptionEn *string
	PricingType   PricingType
	PriceUahMinor *int64
	PriceUsdMinor *int64
	ImageURL      *string
	Active        bool
	SortOrder     int
}

type UpdateInput struct {
	Slug            *string
	NameUk          *string
	NameEn          *string
	DescriptionUk   *string
	HasDescUk       bool
	DescriptionEn   *string
	HasDescEn       bool
	PricingType     *PricingType
	PriceUahMinor   *int64
	HasPriceUah     bool
	PriceUsdMinor   *int64
	HasPriceUsd     bool
	ImageURL        *string
	HasImageURL     bool
	Active          *bool
	SortOrder       *int
}

const columns = `
	id,
	slug,
	name_uk,
	name_en,
	description_uk,
	description_en,
	pricing_type,
	price_uah_minor,
	price_usd_minor,
	image_url,
	active,
	sort_order,
	created_at,
	updated_at
`

const orderBy = `sort_order asc, created_at asc`

func scanOne(row pgx.Row) (*Row, error) {
	var r Row
	var pricing string
	err := row.Scan(
		&r.ID, &r.Slug, &r.NameUk, &r.NameEn,
		&r.DescriptionUk, &r.DescriptionEn,
		&pricing,
		&r.PriceUahMinor, &r.PriceUsdMinor,
		&r.ImageURL, &r.Active, &r.SortOrder,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.PricingType = PricingType(pricing)
	return &r, nil
}

func SelectActive(ctx context.Context) ([]Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select `+columns+` from products where active = true order by `+orderBy)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func SelectAll(ctx context.Context) ([]Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select `+columns+` from products order by `+orderBy)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func collect(rows pgx.Rows) ([]Row, error) {
	defer rows.Close()
	out := make([]Row, 0)
	for rows.Next() {
		r, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func SelectByID(ctx context.Context, id string) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	row := pool.QueryRow(ctx, `select `+columns+` from products where id = $1 limit 1`, id)
	out, err := scanOne(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func SelectBySlug(ctx context.Context, slug string) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	row := pool.QueryRow(ctx, `select `+columns+` from products where slug = $1 limit 1`, slug)
	out, err := scanOne(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func Insert(ctx context.Context, in CreateInput) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	row := pool.QueryRow(ctx, `
		insert into products (
			slug, name_uk, name_en, description_uk, description_en,
			pricing_type, price_uah_minor, price_usd_minor,
			image_url, active, sort_order
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		returning `+columns,
		in.Slug, in.NameUk, in.NameEn,
		in.DescriptionUk, in.DescriptionEn,
		string(in.PricingType),
		in.PriceUahMinor, in.PriceUsdMinor,
		in.ImageURL, in.Active, in.SortOrder,
	)
	out, err := scanOne(row)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert product: %w", err)
	}
	return out, nil
}

func Update(ctx context.Context, id string, in UpdateInput) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}

	var pricingStr *string
	if in.PricingType != nil {
		s := string(*in.PricingType)
		pricingStr = &s
	}

	row := pool.QueryRow(ctx, `
		update products
		set
			slug = coalesce($1, slug),
			name_uk = coalesce($2, name_uk),
			name_en = coalesce($3, name_en),
			description_uk = case when $4::boolean then $5 else description_uk end,
			description_en = case when $6::boolean then $7 else description_en end,
			pricing_type = coalesce($8, pricing_type),
			price_uah_minor = case when $9::boolean then $10::bigint else price_uah_minor end,
			price_usd_minor = case when $11::boolean then $12::bigint else price_usd_minor end,
			image_url = case when $13::boolean then $14 else image_url end,
			active = coalesce($15, active),
			sort_order = coalesce($16, sort_order),
			updated_at = timezone('utc', now())
		where id = $17
		returning `+columns,
		in.Slug, in.NameUk, in.NameEn,
		in.HasDescUk, in.DescriptionUk,
		in.HasDescEn, in.DescriptionEn,
		pricingStr,
		in.HasPriceUah, in.PriceUahMinor,
		in.HasPriceUsd, in.PriceUsdMinor,
		in.HasImageURL, in.ImageURL,
		in.Active, in.SortOrder,
		id,
	)
	out, err := scanOne(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func Delete(ctx context.Context, id string) (*string, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	var deletedID string
	err = pool.QueryRow(ctx, `delete from products where id = $1 returning id`, id).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &deletedID, nil
}
