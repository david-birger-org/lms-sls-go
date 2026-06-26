-- +goose Up
update public.payments
set
  status = 'creation_failed'::public.payment_status,
  updated_at = timezone('utc', now())
where provider = 'monobank'
  and status = 'failed'::public.payment_status
  and invoice_id is null
  and page_url is null
  and provider_status is null;
