alter table public.lectures
  alter column pdf_data drop not null,
  add column if not exists video_id text;
