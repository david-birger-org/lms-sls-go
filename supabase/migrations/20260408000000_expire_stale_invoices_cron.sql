create extension if not exists pg_cron with schema extensions;

select cron.schedule(
  'expire-stale-invoices',
  '*/10 * * * *',
  $$
    update public.payments
    set status = 'expired'::public.payment_status,
        provider_status = 'expired',
        updated_at = timezone('utc', now())
    where status in ('invoice_created', 'processing')
      and (provider_status is null or provider_status in ('created', 'processing', 'hold'))
      and expires_at is not null
      and expires_at < timezone('utc', now())
  $$
);
