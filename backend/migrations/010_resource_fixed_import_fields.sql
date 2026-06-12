use kol_admin;

alter table biz_resources
  add column media_outlet varchar(128) not null default '' after resource_type,
  add column tier varchar(32) not null default '' after media_outlet,
  add column title varchar(128) not null default '' after category,
  add column reference_source varchar(255) not null default '' after region_team,
  add column shipping_address text null after reference_source,
  add column website varchar(512) not null default '' after platform_url,
  add column import_source_sheet varchar(128) not null default '' after website;
