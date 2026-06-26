# lms-sls-go

Go backend for LMS payment, product, contact, and registration endpoints.

## Database

Runtime should use the Neon pooled Postgres connection string:

```sh
DATABASE_URL=postgresql://user:password@ep-example-pooler.region.aws.neon.tech/dbname?sslmode=require
```

Use the direct Neon connection string only for migrations and admin work.

## Migrations

Validate and apply SQL migrations with Goose:

```sh
make migrate-validate

DATABASE_URL_DIRECT=postgresql://user:password@ep-example.region.aws.neon.tech/dbname?sslmode=require \
  make migrate-up
```

The migration files live in `db/migrations/` and are applied by Goose in version order. Goose stores applied versions in the `goose_db_version` table.

`make migrate-down` rolls back one migration with Goose. The imported legacy migrations are forward-only; add explicit `-- +goose Down` sections to new migrations when rollback is required.

Before running migrations on a fresh Neon project, enable `pg_cron` for the target database in Neon by setting `cron.database_name`, then restart the compute. The `20260408000000_expire_stale_invoices_cron.sql` migration creates the extension and schedules the stale-invoice expiry job after that Neon project setting is in place.

## Validate

```sh
go test ./...
```
