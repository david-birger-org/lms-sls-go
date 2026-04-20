package userfeatures

import (
	"context"
	"time"

	"github.com/apexwoot/lms-sls-go/internal/db"
)

type ActiveFeature struct {
	Feature   string    `json:"feature"`
	GrantedAt time.Time `json:"grantedAt"`
}

func SelectActiveFeatures(ctx context.Context, authUserID string) ([]ActiveFeature, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select uf.feature, uf.granted_at
		from user_features uf
		inner join app_users au on au.id = uf.app_user_id
		where au.auth_user_id = $1
		  and uf.revoked_at is null
		order by uf.granted_at asc
	`, authUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActiveFeature, 0)
	for rows.Next() {
		var f ActiveFeature
		if err := rows.Scan(&f.Feature, &f.GrantedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func SelectActiveFeaturesByAppUserID(ctx context.Context, appUserID string) ([]ActiveFeature, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		select feature, granted_at
		from user_features
		where app_user_id = $1
		  and revoked_at is null
		order by granted_at asc
	`, appUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActiveFeature, 0)
	for rows.Next() {
		var f ActiveFeature
		if err := rows.Scan(&f.Feature, &f.GrantedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func HasActiveFeature(ctx context.Context, authUserID, feature string) (bool, error) {
	pool, err := db.Pool(ctx)
	if err != nil {
		return false, err
	}
	var id string
	err = pool.QueryRow(ctx, `
		select uf.id
		from user_features uf
		inner join app_users au on au.id = uf.app_user_id
		where au.auth_user_id = $1
		  and uf.feature = $2
		  and uf.revoked_at is null
		limit 1
	`, authUserID, feature).Scan(&id)
	if err == nil {
		return true, nil
	}
	return false, nil
}

type GrantInput struct {
	AuthUserID         string
	Feature            string
	GrantedByAppUserID *string
	PaymentID          *string
}

func Grant(ctx context.Context, in GrantInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		insert into user_features (app_user_id, feature, granted_by, payment_id)
		select au.id, $2, $3, $4
		from app_users au
		where au.auth_user_id = $1
		on conflict (app_user_id, feature)
		do update set
			revoked_at = null,
			granted_at = timezone('utc', now()),
			granted_by = excluded.granted_by,
			payment_id = coalesce(excluded.payment_id, user_features.payment_id)
	`, in.AuthUserID, in.Feature, in.GrantedByAppUserID, in.PaymentID)
	return err
}

type GrantByAppUserIDInput struct {
	AppUserID          string
	Feature            string
	GrantedByAppUserID *string
	PaymentID          *string
}

func GrantByAppUserID(ctx context.Context, in GrantByAppUserIDInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		insert into user_features (app_user_id, feature, granted_by, payment_id)
		values ($1, $2, $3, $4)
		on conflict (app_user_id, feature)
		do update set
			revoked_at = null,
			granted_at = timezone('utc', now()),
			granted_by = excluded.granted_by,
			payment_id = coalesce(excluded.payment_id, user_features.payment_id)
	`, in.AppUserID, in.Feature, in.GrantedByAppUserID, in.PaymentID)
	return err
}

type RevokeInput struct {
	AuthUserID string
	Feature    string
}

func Revoke(ctx context.Context, in RevokeInput) error {
	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		update user_features uf
		set revoked_at = timezone('utc', now())
		from app_users au
		where au.id = uf.app_user_id
		  and au.auth_user_id = $1
		  and uf.feature = $2
		  and uf.revoked_at is null
	`, in.AuthUserID, in.Feature)
	return err
}
