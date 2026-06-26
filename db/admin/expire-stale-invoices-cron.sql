do $$
declare
  configured_database text := current_setting('cron.database_name', true);
begin
  if configured_database is distinct from current_database() then
    raise exception 'pg_cron is configured for database %, but this script is connected to database %. Set cron.database_name to %, restart the Neon compute, then rerun this script.',
      coalesce(configured_database, '<unset>'),
      current_database(),
      current_database();
  end if;
end $$;

create extension if not exists pg_cron;

select cron.unschedule(jobid)
from cron.job
where jobname = 'expire-stale-invoices';

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
