package lectures

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type Admin struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	Description   *string   `json:"description"`
	CoverImageURL *string   `json:"coverImageUrl"`
	SortOrder     int       `json:"sortOrder"`
	Active        bool      `json:"active"`
	HasNotes      bool      `json:"hasNotes"`
	NotesSize     int64     `json:"notesSize"`
	VideoID       *string   `json:"videoId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Input is the admin create/replace body. Active defaults to true when absent.
type Input struct {
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	CoverImageURL *string `json:"coverImageUrl"`
	SortOrder     int     `json:"sortOrder"`
	Active        *bool   `json:"active"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func ValidSlug(slug string) bool { return slugPattern.MatchString(slug) }

const maxVideoIDLength = 1024

// NormalizeVideoID trims an opaque video provider reference and reports
// whether it is non-empty and at most 1024 characters.
func NormalizeVideoID(raw string) (string, bool) {
	id := strings.TrimSpace(raw)
	return id, id != "" && utf8.RuneCountInString(id) <= maxVideoIDLength
}

func IsPDF(data []byte) bool { return bytes.HasPrefix(data, []byte("%PDF-")) }

func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// Normalize trims fields and reports whether the input is valid.
func (in *Input) Normalize() bool {
	in.Slug = strings.TrimSpace(in.Slug)
	in.Title = strings.TrimSpace(in.Title)
	in.Description = trimOptional(in.Description)
	in.CoverImageURL = trimOptional(in.CoverImageURL)
	if in.Active == nil {
		active := true
		in.Active = &active
	}
	return ValidSlug(in.Slug) && in.Title != ""
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const adminColumns = `
	id,
	slug,
	title,
	description,
	cover_image_url,
	sort_order,
	active,
	pdf_data is not null,
	coalesce(octet_length(pdf_data), 0),
	video_id,
	created_at,
	updated_at
`

const touch = `updated_at = timezone('utc', now())`

func scanAdmin(row pgx.Row) (*Admin, error) {
	var a Admin
	err := row.Scan(
		&a.ID, &a.Slug, &a.Title, &a.Description, &a.CoverImageURL,
		&a.SortOrder, &a.Active, &a.HasNotes, &a.NotesSize, &a.VideoID,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func queryAdmin(ctx context.Context, sql string, args ...any) (*Admin, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	return scanAdmin(pool.QueryRow(ctx, sql, args...))
}

func SelectAllAdmin(ctx context.Context) ([]Admin, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select `+adminColumns+` from lectures order by sort_order asc, created_at asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Admin, 0)
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func SelectAdminByID(ctx context.Context, id string) (*Admin, error) {
	return queryAdmin(ctx, `select `+adminColumns+` from lectures where id = $1`, id)
}

func Insert(ctx context.Context, in Input) (*Admin, error) {
	return queryAdmin(ctx, `
		insert into lectures (slug, title, description, cover_image_url, sort_order, active)
		values ($1, $2, $3, $4, $5, $6)
		returning `+adminColumns,
		in.Slug, in.Title, in.Description, in.CoverImageURL, in.SortOrder, *in.Active,
	)
}

func Update(ctx context.Context, id string, in Input) (*Admin, error) {
	return queryAdmin(ctx, `
		update lectures
		set slug = $1, title = $2, description = $3, cover_image_url = $4,
			sort_order = $5, active = $6, `+touch+`
		where id = $7
		returning `+adminColumns,
		in.Slug, in.Title, in.Description, in.CoverImageURL, in.SortOrder, *in.Active, id,
	)
}

// Delete returns false when no row matched, plus the deleted row's video id.
func Delete(ctx context.Context, id string) (bool, *string, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return false, nil, err
	}
	var videoID *string
	err = pool.QueryRow(ctx, `delete from lectures where id = $1 returning video_id`, id).Scan(&videoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, videoID, nil
}

// SetNotes stores the PDF bytes; nil clears them.
func SetNotes(ctx context.Context, id string, pdf []byte) (*Admin, error) {
	return queryAdmin(ctx, `
		update lectures set pdf_data = $1, `+touch+`
		where id = $2
		returning `+adminColumns,
		pdf, id,
	)
}

// SetVideoID stores videoID (nil clears it) and returns the previous value
// alongside the updated row. The row lock keeps the previous value correct
// under concurrent calls.
func SetVideoID(ctx context.Context, id string, videoID *string) (*Admin, *string, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var previous *string
	err = tx.QueryRow(ctx, `select video_id from lectures where id = $1 for update`, id).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	a, err := scanAdmin(tx.QueryRow(ctx, `
		update lectures set video_id = $1, `+touch+`
		where id = $2
		returning `+adminColumns,
		videoID, id,
	))
	if err != nil || a == nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return a, previous, nil
}
