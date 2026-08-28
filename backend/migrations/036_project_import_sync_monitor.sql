use kol_admin;

alter table biz_platform_sync_jobs
  add column created_by bigint null after message,
  add column created_by_name varchar(128) not null default '' after created_by,
  add index idx_biz_platform_sync_jobs_created_by (created_by, created_at);

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (1105, 1100, 0, '导入同步监控', '/monitor/import-sync', 'ProjectImportSyncMonitor', 'monitor/import-sync/index', null, 'ri:upload-cloud-2-line', '', 1)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  icon = values(icon),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id) values (1, 1105);
