create table if not exists public.user_features (
  id uuid primary key default gen_random_uuid(),
  app_user_id uuid not null references public.app_users(id) on delete cascade,
  feature text not null,
  granted_by uuid references public.app_users(id),
  payment_id uuid references public.payments(id) on delete set null,
  granted_at timestamptz not null default timezone('utc', now()),
  revoked_at timestamptz,
  constraint user_features_unique unique (app_user_id, feature)
);

create index if not exists idx_user_features_active
  on public.user_features (app_user_id, feature)
  where revoked_at is null;

create table if not exists public.lectures (
  id uuid primary key default gen_random_uuid(),
  slug text unique not null,
  title text not null,
  description text,
  pdf_data bytea not null,
  cover_image_url text,
  sort_order int not null default 0,
  active boolean not null default true,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index if not exists idx_lectures_slug
  on public.lectures (slug);

create index if not exists idx_lectures_active
  on public.lectures (active)
  where active = true;
