create table if not exists public.contact_requests (
  id uuid primary key default gen_random_uuid(),
  request_type text not null,
  first_name text,
  last_name text,
  email text,
  country text,
  phone text,
  preferred_contact_method text,
  social text,
  message text,
  service text,
  processed boolean not null default false,
  processed_at timestamptz,
  processed_by uuid references public.app_users(id) on delete set null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  constraint contact_requests_type_check check (request_type in ('contact', 'service'))
);

create index if not exists idx_contact_requests_created_at
  on public.contact_requests (created_at desc);

create index if not exists idx_contact_requests_unprocessed
  on public.contact_requests (created_at desc)
  where processed = false;
