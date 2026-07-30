use kol_admin;

insert ignore into biz_platform_sync_settings
  (platform, enabled, sync_profile, sync_posts, post_limit)
values
  ('Facebook', 0, 1, 1, 20),
  ('LinkedIn', 1, 1, 1, 20),
  ('Reddit', 1, 1, 1, 20);
