package contactrequests

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type Type string

const (
	TypeContact Type = "contact"
	TypeService Type = "service"
)

type Row struct {
	ID                     string     `json:"id"`
	RequestType            Type       `json:"requestType"`
	FirstName              *string    `json:"firstName"`
	LastName               *string    `json:"lastName"`
	Email                  *string    `json:"email"`
	Country                *string    `json:"country"`
	Phone                  *string    `json:"phone"`
	PreferredContactMethod *string    `json:"preferredContactMethod"`
	Social                 *string    `json:"social"`
	Message                *string    `json:"message"`
	Service                *string    `json:"service"`
	Processed              bool       `json:"processed"`
	ProcessedAt            *time.Time `json:"processedAt"`
	ProcessedBy            *string    `json:"processedBy"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type CreateInput struct {
	RequestType            Type
	FirstName              *string
	LastName               *string
	Email                  *string
	Country                *string
	Phone                  *string
	PreferredContactMethod *string
	Social                 *string
	Message                *string
	Service                *string
}

const columns = `
	id,
	request_type,
	first_name,
	last_name,
	email,
	country,
	phone,
	preferred_contact_method,
	social,
	message,
	service,
	processed,
	processed_at,
	processed_by,
	created_at,
	updated_at
`

func scanOne(row pgx.Row) (*Row, error) {
	var r Row
	var requestType string
	err := row.Scan(
		&r.ID,
		&requestType,
		&r.FirstName,
		&r.LastName,
		&r.Email,
		&r.Country,
		&r.Phone,
		&r.PreferredContactMethod,
		&r.Social,
		&r.Message,
		&r.Service,
		&r.Processed,
		&r.ProcessedAt,
		&r.ProcessedBy,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.RequestType = Type(requestType)
	return &r, nil
}

func Insert(ctx context.Context, in CreateInput) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	row := pool.QueryRow(ctx, `
		insert into contact_requests (
			request_type, first_name, last_name, email, country, phone,
			preferred_contact_method, social, message, service
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		returning `+columns,
		string(in.RequestType),
		in.FirstName, in.LastName, in.Email, in.Country, in.Phone,
		in.PreferredContactMethod, in.Social, in.Message, in.Service,
	)
	out, err := scanOne(row)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert contact request: %w", err)
	}
	return out, nil
}

func SelectAll(ctx context.Context) ([]Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select `+columns+` from contact_requests order by created_at desc`)
	if err != nil {
		return nil, err
	}
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

type UpdateProcessedInput struct {
	ID          string
	Processed   bool
	ProcessedBy *string
}

func UpdateProcessed(ctx context.Context, in UpdateProcessedInput) (*Row, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	row := pool.QueryRow(ctx, `
		update contact_requests
		set
			processed = $2,
			processed_at = case when $2 then timezone('utc', now()) else null end,
			processed_by = case when $2 then $3::uuid else null end,
			updated_at = timezone('utc', now())
		where id = $1
		returning `+columns,
		in.ID, in.Processed, in.ProcessedBy,
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
