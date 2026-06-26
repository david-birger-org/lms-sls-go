-- products table for managing purchasable items with admin-controlled pricing
create table if not exists public.products (
  id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  name_uk text not null,
  name_en text not null,
  description_uk text not null,
  description_en text not null,
  price_minor bigint not null,
  currency text not null default 'UAH',
  image_url text,
  active boolean not null default true,
  sort_order int not null default 0,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  constraint products_currency_check check (currency in ('UAH', 'USD'))
);

create index if not exists idx_products_slug on public.products (slug);
create index if not exists idx_products_active on public.products (active) where active = true;

-- link payments to products
alter table public.payments add column if not exists product_id uuid references public.products(id);
create index if not exists idx_payments_product_id on public.payments (product_id);
