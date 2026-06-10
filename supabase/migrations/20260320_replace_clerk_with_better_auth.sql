create table if not exists public.auth_users (
  id text primary key,
  name text not null,
  email text not null unique,
  email_verified boolean not null default false,
  image text,
  role text default 'user',
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  constraint auth_users_role_check check (role in ('admin', 'user'))
);

create table if not exists public.auth_sessions (
  id text primary key,
  user_id text not null references public.auth_users(id) on delete cascade,
  token text not null unique,
  expires_at timestamptz not null,
  ip_address text,
  user_agent text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create table if not exists public.auth_accounts (
  id text primary key,
  user_id text not null references public.auth_users(id) on delete cascade,
  account_id text not null,
  provider_id text not null,
  access_token text,
  refresh_token text,
  access_token_expires_at timestamptz,
  refresh_token_expires_at timestamptz,
  scope text,
  id_token text,
  password text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  constraint auth_accounts_provider_account_unique unique (provider_id, account_id)
);

create table if not exists public.auth_verifications (
  id text primary key,
  identifier text not null,
  value text not null,
  expires_at timestamptz not null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

do $$
begin
  if exists (
    select 1
    from information_schema.columns
    where table_schema = 'public'
      and table_name = 'app_users'
      and column_name = 'clerk_user_id'
  ) then
    alter table public.app_users
      rename column clerk_user_id to auth_user_id;
  end if;
end $$;

alter table public.app_users
  drop column if exists clerk_created_at,
  drop column if exists clerk_updated_at,
  drop column if exists raw_clerk_data;

drop index if exists idx_app_users_clerk_user_id;

create index if not exists idx_app_users_auth_user_id
  on public.app_users (auth_user_id);

create index if not exists idx_auth_sessions_user_id
  on public.auth_sessions (user_id);

create index if not exists idx_auth_accounts_user_id
  on public.auth_accounts (user_id);

create unique index if not exists idx_auth_verifications_identifier_value
  on public.auth_verifications (identifier, value);
