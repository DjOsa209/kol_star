use kol_admin;

insert ignore into biz_platform_sync_settings
  (platform, enabled, sync_profile, sync_posts, post_limit)
values
  ('小红书', 1, 1, 1, 20);

insert into biz_standard_import_options
  (field_key, value, status, source, sort_order)
values
  ('platform', '小红书', '启用', '系统预置', 45)
on duplicate key update
  status = '启用',
  source = '系统预置',
  sort_order = values(sort_order);
