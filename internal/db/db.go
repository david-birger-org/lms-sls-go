package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apexwoot/lms-sls-go/internal/env"
)

const (
	poolMax               = 5
	idleTimeout           = 5 * time.Second
	connectTimeout        = 30 * time.Second
	supabasePoolerSuffix  = ".pooler.supabase.com"
	supabaseDirectPort    = "5432"
)

var (
	mu   sync.Mutex
	pool *pgxpool.Pool
)

func validateConnectionString(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("DATABASE_URL must be a valid Postgres connection string.")
	}
	if strings.HasSuffix(parsed.Hostname(), supabasePoolerSuffix) && parsed.Port() == supabaseDirectPort {
		return "", errors.New("DATABASE_URL must use the Supabase transaction pooler (port 6543) for lms-sls.")
	}
	return raw, nil
}

func buildConfig() (*pgxpool.Config, error) {
	raw, err := env.DatabaseURL()
	if err != nil {
		return nil, err
	}
	conn, err := validateConnectionString(raw)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(conn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = poolMax
	cfg.MaxConnIdleTime = idleTimeout
	cfg.ConnConfig.ConnectTimeout = connectTimeout
	return cfg, nil
}

func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	mu.Lock()
	defer mu.Unlock()
	if pool != nil {
		return pool, nil
	}
	cfg, err := buildConfig()
	if err != nil {
		return nil, err
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	pool = p
	return pool, nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
}
