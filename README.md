# lms-sls-go

Go backend for LMS payment, product, contact, and registration endpoints.

## Database

Runtime should use the Neon pooled Postgres connection string:

```sh
DATABASE_URL=postgresql://user:password@ep-example-pooler.region.aws.neon.tech/dbname?sslmode=require
```

Use the direct Neon connection string only for migrations and admin work.

## Migrations

The Neon database is restored from the latest Supabase public schema/data dump.
The unsupported Supabase-only extension `supabase_vault` is not used by this
service, and the `pg_cron` job is handled separately on Neon.

Validate and apply SQL migrations with Goose:

```sh
make migrate-validate

DATABASE_URL_DIRECT=postgresql://user:password@ep-example.region.aws.neon.tech/dbname?sslmode=require \
  make migrate-up
```

The Goose migration files live in `db/migrations/` and are applied in version order. Goose stores applied versions in the `goose_db_version` table.

The first migration is a no-op baseline that records the restored Neon schema as Goose version `20260626000000`. Historical Supabase migration files are intentionally not replayed against Neon; rebuild Neon from a fresh restore, then run Goose from this baseline forward.

`make migrate-down` rolls back one migration with Goose. The baseline rollback is intentionally a no-op; add explicit `-- +goose Down` sections to new migrations when rollback is required.

Before recreating the stale-invoice cron job on a fresh Neon project, enable `pg_cron` for the target database in Neon by setting `cron.database_name` to the restored database name, then restart the compute. For the default Neon database, this setting should be `neondb`, not `postgres`.

After `pg_cron` is enabled, recreate the scheduled job with:

```sh
psql "$DATABASE_URL_DIRECT" -v ON_ERROR_STOP=1 -f db/admin/expire-stale-invoices-cron.sql
```

## Validate

```sh
go test ./...
```
