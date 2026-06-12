use kol_admin;

alter table biz_resources
  add column platform_user_id varchar(128) not null default '' after platform_url,
  add column platform_handle varchar(128) not null default '' after platform_user_id,
  add column total_views bigint not null default 0 after platform_handle,
  add column video_count bigint not null default 0 after total_views,
  add column last_sync_status varchar(32) not null default '' after video_count,
  add column last_sync_error text null after last_sync_status,
  add column last_sync_at datetime null after last_sync_error;
