-- switch products to dual-currency pricing + pricing_type branching
alter table public.products
  drop constraint if exists products_currency_check;

alter table public.products drop column if exists price_minor;
alter table public.products drop column if exists currency;

alter table public.products
  alter column description_uk drop not null,
  alter column description_en drop not null;

alter table public.products
  add column if not exists pricing_type text not null default 'on_request';

alter table public.products
  add constraint products_pricing_type_check
    check (pricing_type in ('fixed', 'on_request'));

alter table public.products
  add column if not exists price_uah_minor bigint,
  add column if not exists price_usd_minor bigint;

alter table public.products
  add constraint products_fixed_prices_required
    check (
      pricing_type <> 'fixed'
      or (price_uah_minor is not null and price_usd_minor is not null)
    );

-- seed the 5 portfolio services; posing is fixed-price, others are on_request
insert into public.products (slug, name_uk, name_en, pricing_type, price_uah_minor, price_usd_minor, image_url, sort_order)
values
  ('online-coaching',
   'ОНЛАЙН-ВЕДЕННЯ',
   'ONLINE COACHING',
   'on_request', null, null,
   '/images/{locale}/coworking-1.jpg', 1),
  ('personal-training-lviv',
   'ПЕРСОНАЛЬНІ ТРЕНУВАННЯ У ЛЬВОВІ',
   'PERSONAL TRAINING - LVIV',
   'on_request', null, null,
   '/images/{locale}/coworking-2.jpg', 2),
  ('posing-lessons',
   'ІНДИВІДУАЛЬНІ УРОКИ ПОЗУВАННЯ ДЛЯ КАТЕГОРІЙ «БОДІБІЛДИНГ» ТА «КЛАСІК ФІЗІК»',
   'INDIVIDUAL POSING LESSONS - BODYBUILDING & CLASSIC',
   'fixed', 200000, 5000,
   '/images/{locale}/coworking-3.jpg', 3),
  ('consultation',
   'ІНДИВІДУАЛЬНА КОНСУЛЬТАЦІЯ',
   'INDIVIDUAL CONSULTATION Online - video call',
   'on_request', null, null,
   '/images/{locale}/coworking-4.jpg', 4),
  ('training-nutrition-plan',
   'ІНДИВІДУАЛЬНИЙ ТРЕНУВАЛЬНИЙ ПЛАН / ПЛАН ХАРЧУВАННЯ (АБО КОМПЛЕКС)',
   'INDIVIDUAL TRAINING PLAN /NUTRITION PLAN (OR COMBINED PACKAGE)',
   'on_request', null, null,
   '/images/{locale}/coworking-5.jpg', 5)
on conflict (slug) do nothing;
