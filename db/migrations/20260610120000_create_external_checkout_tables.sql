-- +goose Up
create table if not exists external_registrations (
  id uuid primary key,
  payment_id uuid not null references payments(id) on delete cascade,
  source text not null,
  external_ref text not null,
  customer_name text not null,
  customer_email text not null,
  raw_payload jsonb not null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  unique (payment_id),
  unique (source, external_ref)
);

create index if not exists external_registrations_external_ref_idx
  on external_registrations (source, external_ref);

create table if not exists fiscal_checks (
  id uuid primary key,
  payment_id uuid not null references payments(id) on delete cascade,
  invoice_id text not null,
  check_id text not null unique,
  status text not null,
  type text not null,
  fiscalization_source text not null,
  status_description text,
  tax_url text,
  file text,
  payload jsonb not null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index if not exists fiscal_checks_payment_id_idx
  on fiscal_checks (payment_id);

create index if not exists fiscal_checks_invoice_id_idx
  on fiscal_checks (invoice_id);

create index if not exists fiscal_checks_status_idx
  on fiscal_checks (status);
