do $$
begin
  create type public.payment_status as enum (
    'draft',
    'creating_invoice',
    'creation_failed',
    'invoice_created',
    'processing',
    'paid',
    'failed',
    'expired',
    'reversed'
  );
exception
  when duplicate_object then null;
end $$;

alter table public.payments
  add column if not exists idempotency_key text,
  add column if not exists provider_status text;

alter table public.payments
  alter column status drop default;

alter table public.payments
  alter column status type public.payment_status
  using (
    case status::text
      when 'pending_creation' then 'draft'::public.payment_status
      when 'created' then 'invoice_created'::public.payment_status
      when 'creation_failed' then 'creation_failed'::public.payment_status
      when 'processing' then 'processing'::public.payment_status
      when 'success' then 'paid'::public.payment_status
      when 'failure' then 'failed'::public.payment_status
      when 'expired' then 'expired'::public.payment_status
      when 'reversed' then 'reversed'::public.payment_status
      when 'refunded' then 'reversed'::public.payment_status
      else 'draft'::public.payment_status
    end
  );

create unique index if not exists idx_payments_idempotency_key
  on public.payments (idempotency_key)
  where idempotency_key is not null;
