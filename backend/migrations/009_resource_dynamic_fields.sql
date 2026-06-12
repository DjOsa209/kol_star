use kol_admin;

create table if not exists biz_resource_extra_fields (
  id bigint primary key auto_increment,
  field_key varchar(128) not null,
  label varchar(128) not null,
  source_header varchar(128) not null default '',
  status varchar(16) not null default '启用',
  created_at datetime not null default current_timestamp,
  unique key uk_biz_resource_extra_fields_key (field_key)
);

create table if not exists biz_resource_extra_values (
  resource_id bigint not null,
  field_id bigint not null,
  value text null,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  primary key (resource_id, field_id),
  index idx_biz_resource_extra_values_field (field_id)
);
