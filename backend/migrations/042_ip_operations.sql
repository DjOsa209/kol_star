use kol_admin;

create table if not exists biz_ip_resources (
  id bigint primary key auto_increment,
  name varchar(255) not null,
  ip_type varchar(64) not null,
  rights_owner varchar(255) not null default '',
  contact varchar(255) not null default '',
  markets text null,
  audience varchar(500) not null default '',
  summary text null,
  cooperation_status varchar(32) not null default '待评估',
  currency varchar(8) not null default 'CNY',
  price_min decimal(18,2) null,
  price_max decimal(18,2) null,
  license_notes text null,
  created_by bigint not null default 0,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_ip_name_owner (name, rights_owner),
  index idx_ip_status (cooperation_status),
  index idx_ip_type (ip_type)
);

create table if not exists biz_ip_cases (
  id bigint primary key auto_increment,
  ip_id bigint not null,
  title varchar(255) not null,
  summary text null,
  created_at datetime not null default current_timestamp,
  index idx_ip_cases_ip (ip_id)
);

create table if not exists biz_ip_files (
  id bigint primary key auto_increment,
  ip_id bigint not null,
  case_id bigint null,
  file_kind varchar(32) not null,
  original_name varchar(255) not null,
  storage_name varchar(80) not null,
  size_bytes bigint not null,
  uploaded_by bigint not null default 0,
  created_at datetime not null default current_timestamp,
  index idx_ip_files_ip (ip_id),
  index idx_ip_files_case (case_id)
);

create table if not exists biz_ip_requests (
  id bigint primary key auto_increment,
  project_name varchar(255) not null,
  department varchar(128) not null,
  markets text null,
  expected_launch date null,
  goal varchar(128) not null,
  description text not null,
  budget_currency varchar(8) not null default 'CNY',
  budget_min decimal(18,2) null,
  budget_max decimal(18,2) null,
  external_recommendation text null,
  status varchar(32) not null default 'draft',
  market_heat varchar(255) not null default '',
  fan_audience text null,
  commercial_value varchar(255) not null default '',
  marketing_risks text null,
  marketing_channels text null,
  marketing_comments text null,
  created_by bigint not null default 0,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_ip_request_status (status)
);

create table if not exists biz_ip_request_candidates (
  id bigint primary key auto_increment,
  request_id bigint not null,
  ip_id bigint not null,
  priority_order int not null default 0,
  feasibility varchar(32) not null default '',
  recommendation varchar(32) not null default '',
  reason text null,
  unique key uk_ip_request_candidate (request_id, ip_id),
  index idx_ip_candidate_ip (ip_id)
);

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(920, 0, 0, 'menus.ipOperations', '/business/ip', '', '', 3, 'ri:copyright-line', '', 1),
(921, 920, 0, 'menus.ipLibrary', '/business/ip/resources', 'IPResources', 'business/ip/resources/index', 1, 'ri:archive-line', '', 1),
(922, 920, 0, 'menus.ipRequests', '/business/ip/requests', 'IPRequests', 'business/ip/requests/index', 2, 'ri:file-add-line', '', 1),
(923, 920, 0, 'menus.ipFeedback', '/business/ip/feedback', 'IPFeedback', 'business/ip/requests/index', 3, 'ri:feedback-line', '', 1),
(924, 920, 0, 'menus.ipMarketing', '/business/ip/marketing', 'IPMarketing', 'business/ip/requests/index', 4, 'ri:chat-check-line', '', 1)
on duplicate key update parent_id = values(parent_id), title = values(title), path = values(path), name = values(name), component = values(component), `rank` = values(`rank`), icon = values(icon);

insert ignore into sys_role_menus (role_id, menu_id)
select r.id, m.id from sys_roles r join sys_menus m on m.id between 920 and 924
where r.code in ('admin', 'operation') and r.status = 1;
