use kol_admin;

-- Products are curated only after a cooperation-specific suggestion is reviewed.
-- A creator account is always identified by resource_id, never by display name.
create table if not exists biz_discovery_brands (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  name_key varchar(128) not null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_discovery_brand_name (name_key)
);

create table if not exists biz_discovery_products (
  id bigint primary key auto_increment,
  brand_id bigint not null,
  name varchar(128) not null,
  name_key varchar(128) not null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_discovery_product_brand_name (brand_id, name_key),
  index idx_biz_discovery_product_brand (brand_id)
);

create table if not exists biz_cooperation_products (
  id bigint primary key auto_increment,
  cooperation_id bigint not null,
  product_id bigint null,
  candidate_name varchar(128) not null,
  candidate_key varchar(128) not null,
  source_kind varchar(32) not null default 'internal_sync',
  source_field varchar(32) not null,
  source_text varchar(512) not null default '',
  evidence_url varchar(1024) not null default '',
  status varchar(16) not null default 'pending',
  confirmed_by bigint null,
  confirmed_at datetime null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_cooperation_product_candidate (cooperation_id, candidate_key),
  unique key uk_biz_cooperation_product_confirmed (cooperation_id, product_id),
  index idx_biz_cooperation_product_confirmed (status, product_id, cooperation_id),
  index idx_biz_cooperation_product_review (status, created_at)
);
