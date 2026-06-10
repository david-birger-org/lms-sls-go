alter table public.lectures
  drop column if exists blob_path,
  add column if not exists pdf_data bytea;
