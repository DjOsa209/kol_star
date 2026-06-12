create database if not exists kol_admin default character set utf8mb4 collate utf8mb4_unicode_ci;
use kol_admin;

create table if not exists sys_departments (
  id bigint primary key auto_increment,
  parent_id bigint not null default 0,
  name varchar(64) not null,
  principal varchar(64) not null default '',
  phone varchar(32) not null default '',
  email varchar(128) not null default '',
  sort int not null default 0,
  status tinyint not null default 1,
  remark varchar(255) not null default '',
  create_time datetime not null default current_timestamp
);

create table if not exists sys_users (
  id bigint primary key auto_increment,
  dept_id bigint null,
  avatar varchar(255) not null default '',
  username varchar(64) not null unique,
  nickname varchar(64) not null default '',
  password_hash char(64) not null,
  phone varchar(32) not null default '',
  email varchar(128) not null default '',
  sex tinyint not null default 0,
  status tinyint not null default 1,
  remark varchar(255) not null default '',
  create_time datetime not null default current_timestamp,
  update_time datetime not null default current_timestamp on update current_timestamp
);

create table if not exists sys_roles (
  id bigint primary key auto_increment,
  name varchar(64) not null,
  code varchar(64) not null unique,
  status tinyint not null default 1,
  remark varchar(255) not null default '',
  create_time datetime not null default current_timestamp,
  update_time datetime not null default current_timestamp on update current_timestamp
);

create table if not exists sys_user_roles (
  user_id bigint not null,
  role_id bigint not null,
  primary key (user_id, role_id)
);

create table if not exists sys_menus (
  id bigint primary key,
  parent_id bigint not null default 0,
  menu_type tinyint not null default 0,
  title varchar(128) not null,
  path varchar(255) not null default '',
  name varchar(128) not null default '',
  component varchar(255) not null default '',
  `rank` int null,
  icon varchar(128) not null default '',
  auths varchar(255) not null default '',
  show_link tinyint not null default 1
);

create table if not exists sys_role_menus (
  role_id bigint not null,
  menu_id bigint not null,
  primary key (role_id, menu_id)
);

create table if not exists sys_online_users (
  id bigint primary key auto_increment,
  username varchar(64) not null,
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  system varchar(64) not null default '',
  browser varchar(64) not null default '',
  status tinyint not null default 1,
  login_time datetime not null default current_timestamp
);

create table if not exists sys_login_logs (
  id bigint primary key auto_increment,
  username varchar(64) not null,
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  system varchar(64) not null default '',
  browser varchar(64) not null default '',
  status tinyint not null default 1,
  behavior varchar(64) not null default '',
  login_time datetime not null default current_timestamp
);

create table if not exists sys_operation_logs (
  id bigint primary key auto_increment,
  username varchar(64) not null,
  module varchar(64) not null default '',
  summary varchar(255) not null default '',
  method varchar(16) not null default '',
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  system varchar(64) not null default '',
  browser varchar(64) not null default '',
  operation_time datetime not null default current_timestamp
);

create table if not exists sys_system_logs (
  id bigint primary key auto_increment,
  module varchar(64) not null default '',
  url varchar(255) not null default '',
  method varchar(16) not null default '',
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  system varchar(64) not null default '',
  browser varchar(64) not null default '',
  takes_time int not null default 0,
  request_body text null,
  response_body text null,
  request_time datetime not null default current_timestamp
);

insert ignore into sys_departments (id, parent_id, name, principal, phone, email, sort, status, remark) values
(100, 0, '总公司', '小铭', '15888886789', 'admin@example.com', 1, 1, '总部'),
(103, 100, '研发部门', '小铭', '15888886789', 'rd@example.com', 1, 1, '研发团队'),
(105, 100, '测试部门', '小林', '18288882345', 'qa@example.com', 2, 1, '测试团队');

insert ignore into sys_roles (id, name, code, status, remark) values
(1, '超级管理员', 'admin', 1, '超级管理员拥有最高权限'),
(2, '普通角色', 'common', 1, '普通角色拥有部分权限');

insert ignore into sys_users (id, dept_id, avatar, username, nickname, password_hash, phone, email, sex, status, remark) values
(1, 103, 'https://avatars.githubusercontent.com/u/44761321', 'admin', '小铭', '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', '15888886789', 'admin@example.com', 0, 1, '管理员'),
(2, 105, 'https://avatars.githubusercontent.com/u/52823142', 'common', '小林', '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', '18288882345', 'common@example.com', 1, 1, '普通用户');

insert ignore into sys_user_roles (user_id, role_id) values (1, 1), (2, 2);

insert ignore into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(1000, 0, 0, 'menus.pureSysManagement', '/system', '', '', 14, 'ri:settings-3-line', '', 1),
(1001, 1000, 0, 'menus.pureUser', '/system/user/index', 'SystemUser', '', null, 'ri:admin-line', '', 1),
(1002, 1000, 0, 'menus.pureRole', '/system/role/index', 'SystemRole', '', null, 'ri:admin-fill', '', 1),
(1003, 1000, 0, 'menus.pureSystemMenu', '/system/menu/index', 'SystemMenu', '', null, 'ep:menu', '', 1),
(1004, 1000, 0, 'menus.pureDept', '/system/dept/index', 'SystemDept', '', null, 'ri:git-branch-line', '', 1),
(1100, 0, 0, 'menus.pureSysMonitor', '/monitor', '', '', 15, 'ep:monitor', '', 1),
(1101, 1100, 0, 'menus.pureOnlineUser', '/monitor/online-user', 'OnlineUser', 'monitor/online/index', null, 'ri:user-voice-line', '', 1),
(1102, 1100, 0, 'menus.pureLoginLog', '/monitor/login-logs', 'LoginLog', 'monitor/logs/login/index', null, 'ri:window-line', '', 1),
(1103, 1100, 0, 'menus.pureOperationLog', '/monitor/operation-logs', 'OperationLog', 'monitor/logs/operation/index', null, 'ri:history-fill', '', 1),
(1104, 1100, 0, 'menus.pureSystemLog', '/monitor/system-logs', 'SystemLog', 'monitor/logs/system/index', null, 'ri:file-search-line', '', 1);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus;

insert ignore into sys_online_users (id, username, ip, address, system, browser, status) values
(1, 'admin', '127.0.0.1', '本机', 'macOS', 'Chrome', 1);

insert ignore into sys_login_logs (id, username, ip, address, system, browser, status, behavior) values
(1, 'admin', '127.0.0.1', '本机', 'macOS', 'Chrome', 1, '登录系统'),
(2, 'common', '127.0.0.1', '本机', 'Windows', 'Edge', 1, '登录系统');

insert ignore into sys_operation_logs (id, username, module, summary, method, ip, address, system, browser) values
(1, 'admin', '系统管理', '查询用户列表', 'POST', '127.0.0.1', '本机', 'macOS', 'Chrome');

insert ignore into sys_system_logs (id, module, url, method, ip, address, system, browser, takes_time, request_body, response_body) values
(1, '系统管理', '/user', 'POST', '127.0.0.1', '本机', 'macOS', 'Chrome', 38, '{}', '{"code":0}');
