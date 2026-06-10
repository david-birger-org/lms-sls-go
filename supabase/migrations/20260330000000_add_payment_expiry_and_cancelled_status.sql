alter type public.payment_status add value if not exists 'cancelled';

alter table public.payments
  add column if not exists expires_at timestamptz;

create index if not exists idx_payments_expires_at
  on public.payments (expires_at);
