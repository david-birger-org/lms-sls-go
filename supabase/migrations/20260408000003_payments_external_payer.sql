alter table public.payments
  alter column user_id drop not null,
  add column if not exists created_by_admin_user_id uuid references public.app_users(id);

create index if not exists idx_payments_created_by_admin_user_id
  on public.payments (created_by_admin_user_id)
  where created_by_admin_user_id is not null;
