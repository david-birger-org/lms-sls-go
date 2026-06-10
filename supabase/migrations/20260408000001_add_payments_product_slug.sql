alter table public.payments
  add column if not exists product_slug text;

create index if not exists idx_payments_product_slug
  on public.payments (product_slug)
  where product_slug is not null;
