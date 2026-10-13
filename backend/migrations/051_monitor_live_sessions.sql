-- Real, individually revocable login sessions and operation outcome tracking.
-- Existing unsigned login tokens are rejected by the matching API release;
-- users must sign in again after deployment.

use kol_admin;

create table if not exists sys_sessions (
  id bigint primary key auto_increment,
  session_hash char(64) not null,
  user_id bigint not null,
  username varchar(64) not null,
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  `system` varchar(64) not null default '',
  browser varchar(64) not null default '',
  started_at_unix bigint not null,
  last_seen_unix bigint not null,
  expires_at_unix bigint not null,
  revoked_at_unix bigint null,
  unique key uq_sys_sessions_hash (session_hash),
  key idx_sys_sessions_online (revoked_at_unix, expires_at_unix, last_seen_unix),
  key idx_sys_sessions_user (user_id, started_at_unix)
);

alter table sys_operation_logs
  add column status tinyint not null default 1 after method,
  add column audit_version tinyint not null default 0 after status;

create index idx_sys_operation_logs_filter
  on sys_operation_logs (audit_version, operation_time, status, module);

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (1106, 1101, 3, '强制下线', '', '', '', 1, '', 'monitor:online:offline', 0)
on duplicate key update
  parent_id = values(parent_id), title = values(title), auths = values(auths);

insert ignore into sys_role_menus (role_id, menu_id)
select id, 1106 from sys_roles where code = 'admin' and status = 1;

-- Remove only the exact rows seeded by the original example migration.
delete from sys_online_users
 where id = 1 and username = 'admin' and ip = '127.0.0.1'
   and address = '本机' and `system` = 'macOS' and browser = 'Chrome';
delete from sys_login_logs
 where id = 1 and username = 'admin' and ip = '127.0.0.1'
   and address = '本机' and `system` = 'macOS' and browser = 'Chrome'
   and behavior = '登录系统';
delete from sys_login_logs
 where id = 2 and username = 'common' and ip = '127.0.0.1'
   and address = '本机' and `system` = 'Windows' and browser = 'Edge'
   and behavior = '登录系统';
delete from sys_operation_logs
 where id = 1 and username = 'admin' and module = '系统管理'
   and summary = '查询用户列表' and method = 'POST' and ip = '127.0.0.1';
delete from sys_system_logs
 where id = 1 and module = '系统管理' and url = '/user'
   and method = 'POST' and ip = '127.0.0.1' and takes_time = 38;
