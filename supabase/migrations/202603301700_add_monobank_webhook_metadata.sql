alter table public.payments
  add column if not exists provider_modified_at timestamptz;
