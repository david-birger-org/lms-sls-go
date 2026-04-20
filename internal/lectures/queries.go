package lectures

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type Summary struct {
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	CoverImageURL *string `json:"coverImageUrl"`
}

type Detail struct {
	Slug          string
	Title         string
	Description   *string
	PDFData       []byte
	CoverImageURL *string
}

func SelectActive(ctx context.Context) ([]Summary, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select slug, title, description, cover_image_url
		from lectures
		where active = true
		order by sort_order asc, created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Summary, 0)
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.Slug, &s.Title, &s.Description, &s.CoverImageURL); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func SelectBySlug(ctx context.Context, slug string) (*Detail, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	var d Detail
	err = pool.QueryRow(ctx, `
		select slug, title, description, pdf_data, cover_image_url
		from lectures
		where slug = $1
		  and active = true
		limit 1
	`, slug).Scan(&d.Slug, &d.Title, &d.Description, &d.PDFData, &d.CoverImageURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}
