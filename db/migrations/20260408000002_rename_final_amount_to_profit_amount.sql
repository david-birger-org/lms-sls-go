-- +goose Up
-- +goose StatementBegin
do $$
begin
  if exists (
    select 1
    from information_schema.columns
    where table_schema = 'public'
      and table_name = 'payments'
      and column_name = 'final_amount_minor'
  ) and not exists (
    select 1
    from information_schema.columns
    where table_schema = 'public'
      and table_name = 'payments'
      and column_name = 'profit_amount_minor'
  ) then
    alter table public.payments
      rename column final_amount_minor to profit_amount_minor;
  end if;
end $$;
-- +goose StatementEnd
