use kol_admin;

create table if not exists biz_platform_sync_settings (
  platform varchar(64) primary key,
  enabled tinyint not null default 1,
  sync_profile tinyint not null default 1,
  sync_posts tinyint not null default 1,
  post_limit int not null default 25,
  updated_at datetime not null default current_timestamp on update current_timestamp
);

insert ignore into biz_platform_sync_settings
  (platform, enabled, sync_profile, sync_posts, post_limit)
values
  ('YouTube', 1, 1, 1, 25),
  ('Instagram', 1, 1, 1, 25),
  ('TikTok', 1, 1, 1, 20);

insert into biz_governance_rules (rule_type, name, content, enabled)
values ('platform_api', '平台 API 配置', json_object(
  'youtubeApiKeyConfigured', false,
  'youtubeApiKeyLast4', '',
  'metaGraphApiVersion', 'v21.0',
  'instagramAccessTokenConfigured', false,
  'instagramAccessTokenLast4', '',
  'instagramUserId', '',
  'tiktokAccessTokenConfigured', false,
  'tiktokAccessTokenLast4', '',
  'tikhubApiKeyConfigured', false,
  'tikhubApiKeyLast4', ''
), 1)
on duplicate key update
  name = values(name),
  enabled = values(enabled);

create table if not exists biz_platform_sync_jobs (
  id bigint primary key auto_increment,
  job_type varchar(64) not null default 'resource_sync_all',
  status varchar(32) not null default '运行中',
  total_count int not null default 0,
  success_count int not null default 0,
  failed_count int not null default 0,
  skipped_count int not null default 0,
  current_resource_id bigint null,
  current_resource_name varchar(128) not null default '',
  message text null,
  started_at datetime null,
  finished_at datetime null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_biz_platform_sync_jobs_status (status, created_at),
  index idx_biz_platform_sync_jobs_created (created_at)
);

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (1005, 1000, 0, '抓取控制', '/system/platform-sync-control', 'SystemPlatformSyncControl', 'system/platform-sync-control/index', null, 'ri:cloud-line', '', 1)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  icon = values(icon),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id = 1005;
