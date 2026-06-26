-- +goose Up
alter table public.app_users
  drop constraint if exists app_users_auth_user_id_fkey;

alter table public.app_users
  add constraint app_users_auth_user_id_fkey
  foreign key (auth_user_id) references public.auth_users(id) on delete cascade;
