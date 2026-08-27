use kol_admin;

create table if not exists biz_localized_texts (
  id bigint primary key auto_increment,
  entity_type varchar(64) not null,
  entity_id bigint not null,
  field_key varchar(128) not null,
  source_language varchar(16) not null default 'zh-CN',
  target_language varchar(16) not null,
  source_hash char(64) not null,
  source_text mediumtext not null,
  translated_text mediumtext not null,
  translation_status varchar(16) not null default 'completed',
  error_message varchar(512) not null default '',
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_localized_text (entity_type, entity_id, field_key, target_language),
  index idx_biz_localized_text_lookup (entity_type, entity_id, target_language)
);
