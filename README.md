# lms-sls-go

Go backend for LMS payment, product, contact, and registration endpoints.

## Database

Runtime should use the Neon pooled Postgres connection string:

```sh
DATABASE_URL=postgresql://user:password@ep-example-pooler.region.aws.neon.tech/dbname?sslmode=require
```

Use the direct Neon connection string only for migrations and admin work.

## Migrations

Apply SQL migrations with:

```sh
DATABASE_URL_DIRECT=postgresql://user:password@ep-example.region.aws.neon.tech/dbname?sslmode=require \
  make migrate-up
```

The migration files live in `db/migrations/` and are applied in filename order.

`make migrate-down` applies SQL files from `db/migrations/down/` in reverse filename order. No down migrations are currently defined, so the target exits with a clear error until explicit rollback files are added.

Before running migrations on a fresh Neon project, enable `pg_cron` for the target database in Neon by setting `cron.database_name`, then restart the compute. The `20260408000000_expire_stale_invoices_cron.sql` migration creates the extension and schedules the stale-invoice expiry job after that Neon project setting is in place.

## Validate

```sh
go test ./...
```
