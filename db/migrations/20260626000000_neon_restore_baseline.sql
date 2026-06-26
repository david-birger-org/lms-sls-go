-- +goose Up
-- The production Neon database was restored from Supabase with pg_dump/pg_restore.
-- This baseline records that restored schema as the starting point for Goose.
select 1;

-- +goose Down
-- Baseline rollback is intentionally a no-op. Drop/recreate the database from a
-- fresh restore instead of trying to roll back the imported production schema.
select 1;
