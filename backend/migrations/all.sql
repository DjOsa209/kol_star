-- kol-admin database bootstrap bundle
-- Generated from backend/migrations/001_init.sql through 033_sso_default_role.sql.
-- Execute with: mysql -uroot -p < migrations/all.sql


-- ============================================================
-- Source: migrations/001_init.sql
-- ============================================================

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
  `system` varchar(64) not null default '',
  browser varchar(64) not null default '',
  status tinyint not null default 1,
  login_time datetime not null default current_timestamp
);

create table if not exists sys_login_logs (
  id bigint primary key auto_increment,
  username varchar(64) not null,
  ip varchar(64) not null default '',
  address varchar(128) not null default '',
  `system` varchar(64) not null default '',
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
  `system` varchar(64) not null default '',
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
  `system` varchar(64) not null default '',
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

insert ignore into sys_online_users (id, username, ip, address, `system`, browser, status) values
(1, 'admin', '127.0.0.1', '本机', 'macOS', 'Chrome', 1);

insert ignore into sys_login_logs (id, username, ip, address, `system`, browser, status, behavior) values
(1, 'admin', '127.0.0.1', '本机', 'macOS', 'Chrome', 1, '登录系统'),
(2, 'common', '127.0.0.1', '本机', 'Windows', 'Edge', 1, '登录系统');

insert ignore into sys_operation_logs (id, username, module, summary, method, ip, address, `system`, browser) values
(1, 'admin', '系统管理', '查询用户列表', 'POST', '127.0.0.1', '本机', 'macOS', 'Chrome');

insert ignore into sys_system_logs (id, module, url, method, ip, address, `system`, browser, takes_time, request_body, response_body) values
(1, '系统管理', '/user', 'POST', '127.0.0.1', '本机', 'macOS', 'Chrome', 38, '{}', '{"code":0}');

-- ============================================================
-- Source: migrations/002_business.sql
-- ============================================================

use kol_admin;

create table if not exists biz_resources (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  resource_type varchar(32) not null default 'KOL',
  country varchar(64) not null default '',
  region varchar(64) not null default '',
  city varchar(64) not null default '',
  language varchar(64) not null default '',
  platform varchar(64) not null default '',
  industry varchar(128) not null default '',
  category varchar(128) not null default '',
  contact varchar(255) not null default '',
  owner varchar(64) not null default '',
  region_team varchar(64) not null default '',
  status varchar(32) not null default '可合作',
  followers bigint not null default 0,
  engagement_rate decimal(8, 4) not null default 0,
  avg_views bigint not null default 0,
  post_frequency varchar(64) not null default '',
  active_30d int not null default 0,
  active_90d int not null default 0,
  audience_profile text null,
  content_types varchar(255) not null default '',
  platform_url varchar(512) not null default '',
  score int not null default 60,
  level varchar(8) not null default 'B',
  risk_level varchar(16) not null default '低',
  notes text null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_biz_resources_filter (country, platform, status, level),
  index idx_biz_resources_score (score)
);

create table if not exists biz_tags (
  id bigint primary key auto_increment,
  name varchar(64) not null,
  category varchar(32) not null,
  color varchar(32) not null default '#409EFF',
  status varchar(16) not null default '启用',
  created_at datetime not null default current_timestamp,
  unique key uk_biz_tags_category_name (category, name)
);

create table if not exists biz_resource_tags (
  resource_id bigint not null,
  tag_id bigint not null,
  primary key (resource_id, tag_id)
);

create table if not exists biz_projects (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  target_market text null,
  language varchar(64) not null default '',
  platform varchar(64) not null default '',
  campaign_type varchar(64) not null default '',
  budget decimal(14, 2) not null default 0,
  currency varchar(16) not null default 'USD',
  status varchar(32) not null default '未开始',
  owner varchar(64) not null default '',
  brief text null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp
);

create table if not exists biz_cooperations (
  id bigint primary key auto_increment,
  project_id bigint not null,
  resource_id bigint not null,
  cooperation_type varchar(64) not null default '',
  quote_amount decimal(14, 2) not null default 0,
  currency varchar(16) not null default 'USD',
  status varchar(32) not null default '邀约中',
  deliverable_status varchar(32) not null default '未开始',
  review_pass_rate decimal(8, 4) not null default 0,
  publish_on_time_rate decimal(8, 4) not null default 0,
  impressions bigint not null default 0,
  clicks bigint not null default 0,
  conversions bigint not null default 0,
  roi decimal(10, 4) not null default 0,
  team_rating int not null default 0,
  notes text null,
  due_date date null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_biz_cooperations_project (project_id),
  index idx_biz_cooperations_resource (resource_id)
);

create table if not exists biz_brief_templates (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  platform varchar(64) not null default '',
  market varchar(128) not null default '',
  content_type varchar(64) not null default '',
  language varchar(64) not null default '',
  status varchar(16) not null default '启用',
  owner varchar(64) not null default '',
  template text null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp
);

insert ignore into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(900, 0, 0, '资源运营', '/business', '', '', 2, 'ri:global-line', '', 1),
(901, 900, 0, '全球资源库', '/business/resources', 'BusinessResources', 'business/resources/index', null, 'ri:contacts-book-3-line', '', 1),
(902, 900, 0, '标签体系', '/business/tags', 'BusinessTags', 'business/tags/index', null, 'ri:price-tag-3-line', '', 1),
(903, 900, 0, '项目合作', '/business/projects', 'BusinessProjects', 'business/projects/index', null, 'ri:briefcase-4-line', '', 1),
(904, 900, 0, 'Brief模板库', '/business/briefs', 'BusinessBriefs', 'business/briefs/index', null, 'ri:file-list-3-line', '', 1),
(905, 900, 0, '数据看板', '/business/dashboard', 'BusinessDashboard', 'business/dashboard/index', null, 'ri:bar-chart-box-line', '', 1);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id between 900 and 905;

insert ignore into biz_tags (id, name, category, color, status) values
(1, '美国', '基础标签', '#409EFF', '启用'),
(2, '英语', '基础标签', '#409EFF', '启用'),
(3, 'YouTube', '基础标签', '#67C23A', '启用'),
(4, 'TikTok', '基础标签', '#67C23A', '启用'),
(5, 'AI', '内容标签', '#E6A23C', '启用'),
(6, '消费电子', '内容标签', '#E6A23C', '启用'),
(7, '测评', '能力标签', '#909399', '启用'),
(8, '开箱', '能力标签', '#909399', '启用'),
(9, '交付稳定', '合作标签', '#67C23A', '启用'),
(10, '数据异常', '风险标签', '#F56C6C', '启用');

insert ignore into biz_resources
(id, name, resource_type, country, region, city, language, platform, industry, category, contact, owner, region_team, status, followers, engagement_rate, avg_views, content_types, platform_url, score, level, risk_level, notes)
values
(1, 'Tech Review Daily', 'YouTuber', '美国', '北美', 'San Francisco', '英语', 'YouTube', 'AI/消费电子', '科技测评', 'creator@example.com', 'Alice', '北美市场', '可合作', 860000, 0.0480, 125000, '长视频,测评,开箱', 'https://youtube.com/@techreviewdaily', 91, 'S', '低', '适合新品发布和深度评测'),
(2, 'AI Gadget Lab', 'KOL', '德国', '欧洲', 'Berlin', '德语', 'YouTube', 'AI硬件', '垂直科技', 'lab@example.com', 'Bob', '欧洲市场', '观察中', 210000, 0.0360, 42000, '长视频,教程', 'https://youtube.com/@aigadgetlab', 78, 'B', '中', '过往转化稳定，报价偏高'),
(3, 'Daily Byte News', '媒体', '英国', '欧洲', 'London', '英语', 'Newsletter', '科技媒体', '行业媒体', 'editor@example.com', 'Cindy', '欧洲市场', '可合作', 120000, 0.0220, 35000, '图文,新闻报道', 'https://example.com/newsletter', 84, 'A', '低', '适合新闻稿分发');

insert ignore into biz_resource_tags (resource_id, tag_id) values
(1, 1), (1, 2), (1, 3), (1, 5), (1, 7), (1, 9),
(2, 3), (2, 5), (2, 7),
(3, 2), (3, 6), (3, 9);

insert ignore into biz_projects
(id, name, target_market, language, platform, campaign_type, budget, currency, status, owner, brief)
values
(1, 'AI硬件新品发布', '德国', '德语', 'YouTube', '新品发布', 20000, 'USD', '资源筛选', 'Bob', '寻找德国市场 AI 硬件测评类创作者，重点强调性能、易用性和开发者场景。'),
(2, '北美消费电子种草', '美国', '英语', 'TikTok/YouTube', '新品种草', 35000, 'USD', '执行中', 'Alice', '面向北美年轻科技用户，产出短视频种草和深度评测内容。');

insert ignore into biz_cooperations
(id, project_id, resource_id, cooperation_type, quote_amount, currency, status, deliverable_status, impressions, clicks, conversions, roi, team_rating, notes)
values
(1, 1, 2, '深度测评', 8500, 'USD', '确认合作', '脚本审核', 0, 0, 0, 0, 4, '等待第一版脚本'),
(2, 2, 1, '开箱测评', 12000, 'USD', '已发布', '已完成', 310000, 18200, 420, 2.8, 5, '交付准时，互动质量较好');

insert ignore into biz_brief_templates
(id, name, platform, market, content_type, language, status, owner, template)
values
(1, 'YouTube 深度测评 Brief', 'YouTube', '全球', '长视频测评', '英语', '启用', 'HQ', '目标：说明产品核心卖点。\n交付：1条8-12分钟长视频，含开箱、场景测试、结论。\n必须包含：产品定位、3个核心卖点、CTA、合规声明。'),
(2, 'TikTok 新品种草 Brief', 'TikTok', '北美', '短视频', '英语', '启用', 'HQ', '目标：用生活化场景表达产品价值。\n交付：2条30-60秒短视频。\n风格：真实、轻快、避免硬广。');

-- ============================================================
-- Source: migrations/003_system_admin.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_sys_menus_column;
delimiter //
create procedure add_sys_menus_column(
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'sys_menus'
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table sys_menus add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end//
delimiter ;

call add_sys_menus_column('redirect', 'redirect varchar(255) not null default ''''');
call add_sys_menus_column('extra_icon', 'extra_icon varchar(128) not null default ''''');
call add_sys_menus_column('enter_transition', 'enter_transition varchar(128) not null default ''''');
call add_sys_menus_column('leave_transition', 'leave_transition varchar(128) not null default ''''');
call add_sys_menus_column('active_path', 'active_path varchar(255) not null default ''''');
call add_sys_menus_column('frame_src', 'frame_src varchar(512) not null default ''''');
call add_sys_menus_column('frame_loading', 'frame_loading tinyint not null default 1');
call add_sys_menus_column('keep_alive', 'keep_alive tinyint not null default 0');
call add_sys_menus_column('hidden_tag', 'hidden_tag tinyint not null default 0');
call add_sys_menus_column('fixed_tag', 'fixed_tag tinyint not null default 0');
call add_sys_menus_column('show_parent', 'show_parent tinyint not null default 0');

drop procedure if exists add_sys_menus_column;

-- ============================================================
-- Source: migrations/004_default_admin.sql
-- ============================================================

use kol_admin;

insert into sys_departments (id, parent_id, name, principal, phone, email, sort, status, remark)
values (103, 100, '研发部门', '小铭', '15888886789', 'rd@example.com', 1, 1, '研发团队')
on duplicate key update
  name = values(name),
  status = 1;

insert into sys_roles (id, name, code, status, remark)
values (1, '超级管理员', 'admin', 1, '超级管理员拥有最高权限')
on duplicate key update
  name = values(name),
  code = values(code),
  status = 1,
  remark = values(remark);

insert into sys_users
(id, dept_id, avatar, username, nickname, password_hash, phone, email, sex, status, remark)
values
(1, 103, 'https://avatars.githubusercontent.com/u/44761321', 'admin', '小铭', '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', '15888886789', 'admin@example.com', 0, 1, '管理员')
on duplicate key update
  dept_id = values(dept_id),
  avatar = values(avatar),
  username = values(username),
  nickname = values(nickname),
  password_hash = values(password_hash),
  phone = values(phone),
  email = values(email),
  sex = values(sex),
  status = 1,
  remark = values(remark);

insert ignore into sys_user_roles (user_id, role_id) values (1, 1);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus;

-- ============================================================
-- Source: migrations/005_system_permissions.sql
-- ============================================================

use kol_admin;

insert ignore into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(100101, 1001, 3, '新增用户', '', '', '', 1, '', 'system:user:add', 0),
(100102, 1001, 3, '修改用户', '', '', '', 2, '', 'system:user:edit', 0),
(100103, 1001, 3, '删除用户', '', '', '', 3, '', 'system:user:delete', 0),
(100104, 1001, 3, '上传头像', '', '', '', 4, '', 'system:user:upload', 0),
(100105, 1001, 3, '重置密码', '', '', '', 5, '', 'system:user:reset-password', 0),
(100106, 1001, 3, '分配角色', '', '', '', 6, '', 'system:user:assign-role', 0),
(100201, 1002, 3, '新增角色', '', '', '', 1, '', 'system:role:add', 0),
(100202, 1002, 3, '修改角色', '', '', '', 2, '', 'system:role:edit', 0),
(100203, 1002, 3, '删除角色', '', '', '', 3, '', 'system:role:delete', 0),
(100204, 1002, 3, '分配菜单权限', '', '', '', 4, '', 'system:role:menu', 0),
(100301, 1003, 3, '新增菜单', '', '', '', 1, '', 'system:menu:add', 0),
(100302, 1003, 3, '修改菜单', '', '', '', 2, '', 'system:menu:edit', 0),
(100303, 1003, 3, '删除菜单', '', '', '', 3, '', 'system:menu:delete', 0),
(100401, 1004, 3, '新增部门', '', '', '', 1, '', 'system:dept:add', 0),
(100402, 1004, 3, '修改部门', '', '', '', 2, '', 'system:dept:edit', 0),
(100403, 1004, 3, '删除部门', '', '', '', 3, '', 'system:dept:delete', 0);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id in (
  100101, 100102, 100103, 100104, 100105, 100106,
  100201, 100202, 100203, 100204,
  100301, 100302, 100303,
  100401, 100402, 100403
);

-- ============================================================
-- Source: migrations/006_sync_current_menus.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_sys_menus_column;
delimiter //
create procedure add_sys_menus_column(
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'sys_menus'
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table sys_menus add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end//
delimiter ;

call add_sys_menus_column('redirect', 'redirect varchar(255) not null default ''''');
call add_sys_menus_column('extra_icon', 'extra_icon varchar(128) not null default ''''');
call add_sys_menus_column('enter_transition', 'enter_transition varchar(128) not null default ''''');
call add_sys_menus_column('leave_transition', 'leave_transition varchar(128) not null default ''''');
call add_sys_menus_column('active_path', 'active_path varchar(255) not null default ''''');
call add_sys_menus_column('frame_src', 'frame_src varchar(512) not null default ''''');
call add_sys_menus_column('frame_loading', 'frame_loading tinyint not null default 1');
call add_sys_menus_column('keep_alive', 'keep_alive tinyint not null default 0');
call add_sys_menus_column('hidden_tag', 'hidden_tag tinyint not null default 0');
call add_sys_menus_column('fixed_tag', 'fixed_tag tinyint not null default 0');
call add_sys_menus_column('show_parent', 'show_parent tinyint not null default 0');

drop procedure if exists add_sys_menus_column;

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(900, 0, 0, '资源运营', '/business', '', '', 2, 'ri:global-line', '', 1),
(901, 900, 0, '全球资源库', '/business/resources', 'BusinessResources', 'business/resources/index', null, 'ri:contacts-book-3-line', '', 1),
(902, 900, 0, '标签体系', '/business/tags', 'BusinessTags', 'business/tags/index', null, 'ri:price-tag-3-line', '', 1),
(903, 900, 0, '项目合作', '/business/projects', 'BusinessProjects', 'business/projects/index', null, 'ri:briefcase-4-line', '', 1),
(904, 900, 0, 'Brief模板库', '/business/briefs', 'BusinessBriefs', 'business/briefs/index', null, 'ri:file-list-3-line', '', 1),
(905, 900, 0, '数据看板', '/business/dashboard', 'BusinessDashboard', 'business/dashboard/index', null, 'ri:bar-chart-box-line', '', 1),
(1000, 0, 0, 'menus.pureSysManagement', '/system', '', '', 14, 'ri:settings-3-line', '', 1),
(1001, 1000, 0, 'menus.pureUser', '/system/user/index', 'SystemUser', '', null, 'ri:admin-line', '', 1),
(1002, 1000, 0, 'menus.pureRole', '/system/role/index', 'SystemRole', '', null, 'ri:admin-fill', '', 1),
(1003, 1000, 0, 'menus.pureSystemMenu', '/system/menu/index', 'SystemMenu', '', null, 'ep:menu', '', 1),
(1004, 1000, 0, 'menus.pureDept', '/system/dept/index', 'SystemDept', '', null, 'ri:git-branch-line', '', 1),
(1100, 0, 0, 'menus.pureSysMonitor', '/monitor', '', '', 15, 'ep:monitor', '', 1),
(1101, 1100, 0, 'menus.pureOnlineUser', '/monitor/online-user', 'OnlineUser', 'monitor/online/index', null, 'ri:user-voice-line', '', 1),
(1102, 1100, 0, 'menus.pureLoginLog', '/monitor/login-logs', 'LoginLog', 'monitor/logs/login/index', null, 'ri:window-line', '', 1),
(1103, 1100, 0, 'menus.pureOperationLog', '/monitor/operation-logs', 'OperationLog', 'monitor/logs/operation/index', null, 'ri:history-fill', '', 1),
(1104, 1100, 0, 'menus.pureSystemLog', '/monitor/system-logs', 'SystemLog', 'monitor/logs/system/index', null, 'ri:file-search-line', '', 1),
(100101, 1001, 3, '新增用户', '', '', '', 1, '', 'system:user:add', 0),
(100102, 1001, 3, '修改用户', '', '', '', 2, '', 'system:user:edit', 0),
(100103, 1001, 3, '删除用户', '', '', '', 3, '', 'system:user:delete', 0),
(100104, 1001, 3, '上传头像', '', '', '', 4, '', 'system:user:upload', 0),
(100105, 1001, 3, '重置密码', '', '', '', 5, '', 'system:user:reset-password', 0),
(100106, 1001, 3, '分配角色', '', '', '', 6, '', 'system:user:assign-role', 0),
(100201, 1002, 3, '新增角色', '', '', '', 1, '', 'system:role:add', 0),
(100202, 1002, 3, '修改角色', '', '', '', 2, '', 'system:role:edit', 0),
(100203, 1002, 3, '删除角色', '', '', '', 3, '', 'system:role:delete', 0),
(100204, 1002, 3, '分配菜单权限', '', '', '', 4, '', 'system:role:menu', 0),
(100301, 1003, 3, '新增菜单', '', '', '', 1, '', 'system:menu:add', 0),
(100302, 1003, 3, '修改菜单', '', '', '', 2, '', 'system:menu:edit', 0),
(100303, 1003, 3, '删除菜单', '', '', '', 3, '', 'system:menu:delete', 0),
(100401, 1004, 3, '新增部门', '', '', '', 1, '', 'system:dept:add', 0),
(100402, 1004, 3, '修改部门', '', '', '', 2, '', 'system:dept:edit', 0),
(100403, 1004, 3, '删除部门', '', '', '', 3, '', 'system:dept:delete', 0)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  `rank` = values(`rank`),
  icon = values(icon),
  auths = values(auths),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus;

-- ============================================================
-- Source: migrations/007_business_import_fields.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_bundle_column;

delimiter $$

create procedure add_bundle_column(
  in p_table_name varchar(64),
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = p_table_name
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table ', p_table_name, ' add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

call add_bundle_column('biz_cooperations', 'views', 'views bigint not null default 0 after impressions');
call add_bundle_column('biz_cooperations', 'engagement_count', 'engagement_count bigint not null default 0 after conversions');
call add_bundle_column('biz_cooperations', 'comments_count', 'comments_count bigint not null default 0 after engagement_count');
call add_bundle_column('biz_cooperations', 'release_date', 'release_date date null after team_rating');
call add_bundle_column('biz_cooperations', 'deliverable_links', 'deliverable_links text null after release_date');
call add_bundle_column('biz_cooperations', 'import_batch_id', 'import_batch_id varchar(64) not null default '''' after deliverable_links');

-- ============================================================
-- Source: migrations/008_resource_platform_sync.sql
-- ============================================================

use kol_admin;

call add_bundle_column('biz_resources', 'platform_user_id', 'platform_user_id varchar(128) not null default '''' after platform_url');
call add_bundle_column('biz_resources', 'platform_handle', 'platform_handle varchar(128) not null default '''' after platform_user_id');
call add_bundle_column('biz_resources', 'total_views', 'total_views bigint not null default 0 after platform_handle');
call add_bundle_column('biz_resources', 'video_count', 'video_count bigint not null default 0 after total_views');
call add_bundle_column('biz_resources', 'last_sync_status', 'last_sync_status varchar(32) not null default '''' after video_count');
call add_bundle_column('biz_resources', 'last_sync_error', 'last_sync_error text null after last_sync_status');
call add_bundle_column('biz_resources', 'last_sync_at', 'last_sync_at datetime null after last_sync_error');

-- ============================================================
-- Source: migrations/009_resource_dynamic_fields.sql
-- ============================================================

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

-- ============================================================
-- Source: migrations/010_resource_fixed_import_fields.sql
-- ============================================================

use kol_admin;

call add_bundle_column('biz_resources', 'media_outlet', 'media_outlet varchar(128) not null default '''' after resource_type');
call add_bundle_column('biz_resources', 'tier', 'tier varchar(32) not null default '''' after media_outlet');
call add_bundle_column('biz_resources', 'title', 'title varchar(128) not null default '''' after category');
call add_bundle_column('biz_resources', 'reference_source', 'reference_source varchar(255) not null default '''' after region_team');
call add_bundle_column('biz_resources', 'shipping_address', 'shipping_address text null after reference_source');
call add_bundle_column('biz_resources', 'website', 'website varchar(512) not null default '''' after platform_url');
call add_bundle_column('biz_resources', 'import_source_sheet', 'import_source_sheet varchar(128) not null default '''' after website');

drop procedure if exists add_bundle_column;

-- ============================================================
-- Source: migrations/011_prd_v2_assistant_governance.sql
-- ============================================================

use kol_admin;

create table if not exists biz_project_resources (
  id bigint primary key auto_increment,
  project_id bigint not null,
  resource_id bigint not null,
  status varchar(32) not null default '候选',
  source varchar(32) not null default '智能助手',
  recommend_reason text null,
  priority varchar(16) not null default '',
  estimated_cost decimal(14, 2) not null default 0,
  risk_tip varchar(255) not null default '',
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_project_resources_project_resource (project_id, resource_id),
  index idx_biz_project_resources_project (project_id),
  index idx_biz_project_resources_resource (resource_id)
);

create table if not exists biz_governance_rules (
  id bigint primary key auto_increment,
  rule_type varchar(64) not null,
  name varchar(128) not null,
  content json not null,
  enabled tinyint not null default 1,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_governance_rules_type (rule_type)
);

create table if not exists biz_rule_versions (
  id bigint primary key auto_increment,
  rule_id bigint not null,
  rule_type varchar(64) not null,
  version_no varchar(32) not null,
  content json not null,
  effective_mode varchar(32) not null default '立即生效',
  effective_at datetime null,
  impact_summary varchar(255) not null default '',
  created_by varchar(64) not null default '',
  created_at datetime not null default current_timestamp,
  index idx_biz_rule_versions_rule (rule_id),
  index idx_biz_rule_versions_type (rule_type)
);

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(906, 900, 0, '智能资源助手', '/business/assistant', 'BusinessAssistant', 'business/assistant/index', null, 'ri:chat-search-line', '', 1),
(907, 900, 0, '治理规则', '/business/governance', 'BusinessGovernance', 'business/governance/index', null, 'ri:settings-4-line', '', 1)
on duplicate key update
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  icon = values(icon),
  show_link = values(show_link);

update sys_menus set show_link = 0 where id = 904;

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id in (906, 907);

insert into biz_governance_rules (rule_type, name, content, enabled) values
('ai_model', 'AI模型配置', json_object(
  'provider', '',
  'model', '',
  'baseUrl', '',
  'apiKeyConfigured', false,
  'fallbackStrategy', 'AI不可用时走本地规则推荐'
), 1),
('scoring_model', '评分模型配置', json_object(
  'influence', 20,
  'activity', 15,
  'interactionQuality', 20,
  'brandFit', 15,
  'deliveryPerformance', 20,
  'conversionEffect', 10
), 1),
('level_threshold', '等级阈值配置', json_object(
  'S', 90,
  'A', 80,
  'B', 65,
  'C', 50
), 1),
('required_fields', '必填数据规则', json_object(
  'creator', json_array('Profile URL', '平台', '国家', '语言', '粉丝数', '负责人'),
  'media', json_array('官网URL', '国家', '语言', '行业', '联系人', '负责人'),
  'agency', json_array('公司名称', '国家', '联系人', '合作范围', '负责人')
), 1),
('update_frequency', '更新频率规则', json_object(
  'SA', 30,
  'BC', 90,
  'D', 180
), 1),
('data_trust', '数据可信度规则', json_object(
  'A', json_object('source', '官方API或授权后台数据', 'factor', 1),
  'B', json_object('source', '创作者后台截图或录屏', 'factor', 0.9),
  'C', json_object('source', '第三方工具估算', 'factor', 0.8),
  'D', json_object('source', '人工公开页面采集', 'factor', 0.7)
), 1),
('recommendation', '智能推荐规则', json_object(
  'minimumLevel', 'B',
  'excludeBlacklisted', true,
  'includeWatchingByDefault', false,
  'minimumCompleteness', 80,
  'maxDaysSinceUpdate', 180,
  'overBudgetPolicy', 'filter',
  'highRiskPolicy', 'downgrade_or_filter'
), 1),
('warning', '预警规则', json_object(
  'scoreDrop', 10,
  'costAbovePeerAveragePercent', 50,
  'deliveryDelayTimes', 2,
  'staleContact', true
), 1)
on duplicate key update
  name = values(name),
  content = values(content),
  enabled = values(enabled);

-- ============================================================
-- Source: migrations/012_ai_model_config_menu.sql
-- ============================================================

use kol_admin;

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(908, 900, 0, 'AI模型配置', '/business/ai-model', 'BusinessAIModel', 'business/ai-model/index', null, 'ri:sparkling-2-line', '', 1)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  `rank` = values(`rank`),
  icon = values(icon),
  auths = values(auths),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id = 908;

-- ============================================================
-- Source: migrations/013_platform_sync_engine.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_biz_resources_platform_column;

delimiter $$

create procedure add_biz_resources_platform_column(
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_resources'
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table biz_resources add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

call add_biz_resources_platform_column('avatar_url', 'avatar_url varchar(1024) not null default '''' after platform_handle');

drop procedure if exists add_biz_resources_platform_column;

create table if not exists biz_resource_platform_posts (
  id bigint primary key auto_increment,
  resource_id bigint not null,
  platform varchar(64) not null default '',
  platform_post_id varchar(128) not null default '',
  title varchar(255) not null default '',
  description text null,
  post_url varchar(1024) not null default '',
  cover_url varchar(1024) not null default '',
  media_type varchar(64) not null default '',
  published_at datetime null,
  duration_seconds int not null default 0,
  view_count bigint not null default 0,
  like_count bigint not null default 0,
  comment_count bigint not null default 0,
  share_count bigint not null default 0,
  save_count bigint not null default 0,
  raw_json json null,
  synced_at datetime not null default current_timestamp,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_resource_platform_post (resource_id, platform, platform_post_id),
  index idx_biz_resource_platform_posts_resource (resource_id),
  index idx_biz_resource_platform_posts_published (published_at),
  index idx_biz_resource_platform_posts_metrics (view_count, like_count)
);

-- ============================================================
-- Source: migrations/014_platform_sync_control.sql
-- ============================================================

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

-- ============================================================
-- Source: migrations/015_resource_platform_posts_menu.sql
-- ============================================================

use kol_admin;

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (909, 900, 0, '作品数据', '/business/resource-posts', 'BusinessResourcePosts', 'business/resource-posts/index', null, 'ri:video-line', '', 1)
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
select 1, id from sys_menus where id = 909;

-- ============================================================
-- Source: migrations/016_market_options.sql
-- ============================================================

use kol_admin;

create table if not exists biz_market_options (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  region_group varchar(64) not null default '',
  status varchar(16) not null default '启用',
  source varchar(32) not null default '系统预置',
  sort_order int not null default 100,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_market_options_name (name),
  index idx_biz_market_options_status_sort (status, sort_order)
);

insert into biz_market_options
  (name, region_group, status, source, sort_order)
values
  ('美国', '欧美', '启用', '系统预置', 10),
  ('英国', '欧美', '启用', '系统预置', 20),
  ('欧洲', '欧美', '启用', '系统预置', 30),
  ('德国', '欧美', '启用', '系统预置', 40),
  ('日本', '亚太', '启用', '系统预置', 50),
  ('中东北非', 'MENA', '启用', '系统预置', 60),
  ('东非', '非洲', '启用', '系统预置', 70),
  ('西非', '非洲', '启用', '系统预置', 80),
  ('东南亚', '亚太', '启用', '系统预置', 90),
  ('拉美', '拉美', '启用', '系统预置', 100)
on duplicate key update
  region_group = values(region_group),
  sort_order = values(sort_order);

-- ============================================================
-- Source: migrations/017_campaign_execution_center.sql
-- ============================================================

use kol_admin;

update sys_menus
set
  title = '项目管理',
  icon = 'ri:flow-chart'
where id = 903 or path = '/business/projects';

-- ============================================================
-- Source: migrations/018_dashboard_first_menu.sql
-- ============================================================

use kol_admin;

update sys_menus set
  parent_id = 900,
  title = '看板',
  path = '/business/dashboard',
  name = 'BusinessDashboard',
  component = 'business/dashboard/index',
  `rank` = 1,
  icon = 'ri:bar-chart-box-line',
  show_link = 1
where id = 905 or path = '/business/dashboard';

update sys_menus set
  parent_id = 900,
  title = '项目管理',
  path = '/business/projects',
  name = 'BusinessProjects',
  component = 'business/projects/index',
  `rank` = 4,
  icon = 'ri:flow-chart',
  show_link = 1
where id = 903 or path = '/business/projects';

update sys_menus set
  parent_id = 900,
  title = 'AI模型配置',
  path = '/business/ai-model',
  name = 'BusinessAIModel',
  component = 'business/ai-model/index',
  `rank` = 9,
  icon = 'ri:sparkling-2-line',
  show_link = 1
where id = 908 or path = '/business/ai-model';

update sys_menus
set `rank` = case path
  when '/business/resources' then 2
  when '/business/tags' then 3
  when '/business/projects' then 4
  when '/business/briefs' then 5
  when '/business/resource-posts' then 6
  when '/business/assistant' then 7
  when '/business/governance' then 8
  when '/business/ai-model' then 9
  else `rank`
end
where parent_id = 900;

-- ============================================================
-- Source: migrations/019_campaign_detail_features.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_campaign_column;

delimiter $$

create procedure add_campaign_column(
  in p_table_name varchar(64),
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = p_table_name
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table ', p_table_name, ' add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

call add_campaign_column('biz_projects', 'cycle_start_date', 'cycle_start_date date null after brief');
call add_campaign_column('biz_projects', 'cycle_end_date', 'cycle_end_date date null after cycle_start_date');
call add_campaign_column('biz_projects', 'report_update_date', 'report_update_date date null after cycle_end_date');
call add_campaign_column('biz_projects', 'paused_at', 'paused_at datetime null after report_update_date');

call add_campaign_column('biz_cooperations', 'audience_segment', 'audience_segment varchar(128) not null default '''' after cooperation_type');
call add_campaign_column('biz_cooperations', 'creative_name', 'creative_name varchar(128) not null default '''' after audience_segment');
call add_campaign_column('biz_cooperations', 'final_link', 'final_link varchar(1024) not null default '''' after deliverable_links');
call add_campaign_column('biz_cooperations', 'top_geographies', 'top_geographies varchar(255) not null default '''' after final_link');
call add_campaign_column('biz_cooperations', 'publish_time', 'publish_time datetime null after top_geographies');
call add_campaign_column('biz_cooperations', 'tracking_link', 'tracking_link varchar(1024) not null default '''' after publish_time');
call add_campaign_column('biz_cooperations', 'ad_authorization_code', 'ad_authorization_code varchar(255) not null default '''' after tracking_link');

drop procedure if exists add_campaign_column;

create table if not exists biz_campaign_deliverables (
  id bigint primary key auto_increment,
  project_id bigint not null,
  cooperation_id bigint not null,
  stage_key varchar(64) not null default '',
  title varchar(128) not null default '',
  status varchar(32) not null default '',
  submitted_at datetime null,
  link varchar(1024) not null default '',
  caption text null,
  note text null,
  rejection_reason text null,
  sort_order int not null default 100,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_campaign_deliverables_project (project_id),
  index idx_campaign_deliverables_cooperation (cooperation_id),
  index idx_campaign_deliverables_stage (stage_key)
);

create table if not exists biz_campaign_report_segments (
  id bigint primary key auto_increment,
  project_id bigint not null,
  audience_segment varchar(128) not null default '',
  platform varchar(64) not null default '',
  creative_name varchar(128) not null default '',
  forecast_views bigint not null default 0,
  actual_views bigint not null default 0,
  forecast_clicks bigint not null default 0,
  actual_clicks bigint not null default 0,
  forecast_cost decimal(14, 2) not null default 0,
  actual_cost decimal(14, 2) not null default 0,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_campaign_report_segment (project_id, audience_segment, platform, creative_name),
  index idx_campaign_report_segments_project (project_id)
);

create table if not exists biz_campaign_billing_events (
  id bigint primary key auto_increment,
  project_id bigint not null,
  event_type varchar(64) not null default '',
  amount decimal(14, 2) not null default 0,
  currency varchar(16) not null default 'USD',
  description varchar(255) not null default '',
  occurred_at datetime not null default current_timestamp,
  created_at datetime not null default current_timestamp,
  index idx_campaign_billing_events_project (project_id),
  index idx_campaign_billing_events_time (occurred_at)
);

create table if not exists biz_campaign_influencer_reports (
  id bigint primary key auto_increment,
  project_id bigint not null,
  cooperation_id bigint not null,
  resource_id bigint not null,
  reason varchar(128) not null default '',
  detail text null,
  status varchar(32) not null default '待处理',
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_campaign_influencer_reports_project (project_id),
  index idx_campaign_influencer_reports_cooperation (cooperation_id)
);

update biz_projects
   set cycle_start_date = coalesce(cycle_start_date, date(created_at)),
       cycle_end_date = coalesce(cycle_end_date, date_add(date(created_at), interval 30 day)),
       report_update_date = coalesce(report_update_date, current_date())
 where cycle_start_date is null
    or cycle_end_date is null
    or report_update_date is null;

update biz_cooperations c
left join biz_resources r on r.id = c.resource_id
   set c.audience_segment = if(c.audience_segment = '', coalesce(nullif(r.category, ''), nullif(r.industry, ''), 'All audiences'), c.audience_segment),
       c.creative_name = if(c.creative_name = '', coalesce(nullif(c.cooperation_type, ''), 'Default creative'), c.creative_name),
       c.final_link = if(c.final_link = '', coalesce(c.deliverable_links, ''), c.final_link),
       c.top_geographies = if(c.top_geographies = '', coalesce(nullif(r.country, ''), 'United States'), c.top_geographies),
       c.publish_time = coalesce(c.publish_time, if(c.release_date is null, null, cast(c.release_date as datetime))),
       c.tracking_link = if(c.tracking_link = '', concat('https://AhaCreator.tryit.cc/', c.id, '-', c.project_id), c.tracking_link)
 where c.audience_segment = ''
    or c.creative_name = ''
    or c.final_link = ''
    or c.top_geographies = ''
    or c.publish_time is null
    or c.tracking_link = '';

insert ignore into biz_campaign_report_segments
  (project_id, audience_segment, platform, creative_name, forecast_views, actual_views, forecast_clicks, actual_clicks, forecast_cost, actual_cost)
select c.project_id,
       coalesce(nullif(c.audience_segment, ''), 'All audiences'),
       coalesce(nullif(r.platform, ''), nullif(c.creative_name, ''), 'All platforms'),
       coalesce(nullif(c.creative_name, ''), 'Default creative'),
       greatest(sum(greatest(c.impressions, c.views)) * 105 div 100, 1000),
       sum(greatest(c.impressions, c.views)),
       greatest(sum(c.clicks) * 105 div 100, 100),
       sum(c.clicks),
       coalesce(sum(c.quote_amount) * 1.03, 0),
       coalesce(sum(c.quote_amount), 0)
  from biz_cooperations c
  left join biz_resources r on r.id = c.resource_id
 group by c.project_id, c.audience_segment, r.platform, c.creative_name;

insert into biz_campaign_billing_events
  (project_id, event_type, amount, currency, description, occurred_at)
select c.project_id, '合作费用', c.quote_amount, c.currency,
       concat('Influencer cooperation: ', coalesce(r.name, concat('#', c.resource_id))),
       coalesce(c.publish_time, c.updated_at)
  from biz_cooperations c
  left join biz_resources r on r.id = c.resource_id
 where c.quote_amount > 0
   and not exists (
     select 1
       from biz_campaign_billing_events b
      where b.project_id = c.project_id
        and b.description = concat('Influencer cooperation: ', coalesce(r.name, concat('#', c.resource_id)))
        and b.amount = c.quote_amount
   );

insert into biz_campaign_deliverables
  (project_id, cooperation_id, stage_key, title, status, submitted_at, link, caption, note, sort_order)
select c.project_id, c.id, seed.stage_key, seed.title, seed.status,
       case seed.stage_key
         when 'influencer_applied' then c.created_at
         when 'deal_confirmed' then c.created_at
         when 'kickoff_production' then c.updated_at
         when 'idea_script' then c.updated_at
         when 'video_draft' then c.updated_at
         when 'final_link' then coalesce(c.publish_time, c.updated_at)
       end,
       case seed.stage_key
         when 'final_link' then coalesce(nullif(c.final_link, ''), c.deliverable_links, '')
         when 'video_draft' then coalesce(c.deliverable_links, '')
         else ''
       end,
       case seed.stage_key
         when 'video_draft' then c.notes
         else null
       end,
       seed.note,
       seed.sort_order
  from biz_cooperations c
  join (
    select 'final_link' as stage_key, 'Final link' as title, 'Completed' as status, '最终发布链接已回收。' as note, 10 as sort_order
    union all select 'video_draft', 'Video draft 1', 'Approved', '内容草稿已提交并进入审核。', 20
    union all select 'idea_script', 'Idea/script', 'Skipped', '创作者跳过脚本阶段，直接进入视频制作。', 30
    union all select 'kickoff_production', 'Kickoff production', 'Completed', '合作已启动制作。', 40
    union all select 'deal_confirmed', 'Deal confirmed', 'Completed', '合作条款已确认。', 50
    union all select 'influencer_applied', 'Influencer applied', 'Completed', '达人已申请或被加入 Campaign。', 60
  ) seed
 where not exists (
   select 1
     from biz_campaign_deliverables d
    where d.cooperation_id = c.id
      and d.stage_key = seed.stage_key
 );

-- ============================================================
-- Source: migrations/020_standard_project_import.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_standard_import_column;

delimiter $$

create procedure add_standard_import_column(
  in p_table_name varchar(64),
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = p_table_name
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table ', p_table_name, ' add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

-- audience_size is the common standard-field value for both KOL followers and
-- media monthly visitors. followers remains for compatibility and platform sync.
call add_standard_import_column('biz_resources', 'market', 'market varchar(128) not null default '''' after country');
call add_standard_import_column('biz_resources', 'audience_size', 'audience_size bigint not null default 0 after followers');
call add_standard_import_column('biz_resources', 'audience_size_unit', 'audience_size_unit varchar(32) not null default '''' after audience_size');

-- Owner and vendor belong to a project cooperation, not to the global resource.
call add_standard_import_column('biz_cooperations', 'owner', 'owner varchar(128) not null default '''' after cooperation_type');
call add_standard_import_column('biz_cooperations', 'vendor', 'vendor varchar(128) not null default '''' after owner');

drop procedure if exists add_standard_import_column;

update biz_resources
   set market = if(market = '', country, market),
       audience_size = if(audience_size = 0, followers, audience_size),
       audience_size_unit = if(audience_size_unit = '', if(resource_type = '媒体', 'UMV', 'Followers'), audience_size_unit)
 where market = '' or audience_size = 0 or audience_size_unit = '';

-- ============================================================
-- Source: migrations/021_standard_import_options.sql
-- ============================================================

use kol_admin;

create table if not exists biz_standard_import_options (
  id bigint primary key auto_increment,
  field_key varchar(64) not null,
  value varchar(128) not null,
  status varchar(16) not null default '启用',
  source varchar(32) not null default '系统预置',
  sort_order int not null default 100,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_standard_import_option (field_key, value),
  index idx_biz_standard_import_option_status (field_key, status, sort_order)
);

insert into biz_standard_import_options (field_key, value, source, sort_order)
values
  ('resourceType', 'KOL', '系统预置', 10),
  ('resourceType', '媒体', '系统预置', 20),
  ('resourceType', '艺术家', '系统预置', 30),
  ('category', '科技', '系统预置', 10),
  ('category', '生活方式', '系统预置', 20),
  ('category', '商业', '系统预置', 30),
  ('category', '设计', '系统预置', 40),
  ('category', '游戏', '系统预置', 50),
  ('category', '摄影', '系统预置', 60),
  ('category', '体育', '系统预置', 70),
  ('category', '娱乐', '系统预置', 80),
  ('category', '汽车', '系统预置', 90),
  ('category', '财经', '系统预置', 100),
  ('category', '教育', '系统预置', 110),
  ('category', '大众媒体', '系统预置', 120),
  ('platform', 'Website', '系统预置', 10),
  ('platform', '播客', '系统预置', 20),
  ('platform', '电视', '系统预置', 30),
  ('platform', '报刊', '系统预置', 40),
  ('platform', 'YouTube', '系统预置', 50),
  ('platform', 'TikTok', '系统预置', 60),
  ('platform', 'Instagram', '系统预置', 70),
  ('platform', 'Facebook', '系统预置', 80),
  ('platform', 'X', '系统预置', 90),
  ('platform', 'LinkedIn', '系统预置', 100),
  ('platform', 'Reddit', '系统预置', 110),
  ('cooperationType', '付费合作', '系统预置', 10),
  ('cooperationType', '产品置换', '系统预置', 20),
  ('cooperationType', '联盟合作', '系统预置', 30),
  ('cooperationType', '活动合作', '系统预置', 40),
  ('cooperationType', '采访合作', '系统预置', 50)
on duplicate key update
  status = '启用',
  sort_order = values(sort_order);

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (1006, 1000, 0, '标准字段配置', '/system/standard-import-options', 'SystemStandardImportOptions', 'system/standard-import-options/index', null, 'ri:list-settings-line', '', 1)
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
select 1, id from sys_menus where id = 1006;

-- ============================================================
-- Source: migrations/022_collaborator_tiers.sql
-- ============================================================

use kol_admin;

-- 层级是系统计算字段，选项固定，不允许管理员改写。
insert into biz_standard_import_options (field_key, value, status, source, sort_order)
values
  ('collaboratorTier', '头部', '启用', '系统计算', 10),
  ('collaboratorTier', '腰部', '启用', '系统计算', 20),
  ('collaboratorTier', '尾部', '启用', '系统计算', 30)
on duplicate key update
  status = '启用',
  source = '系统计算',
  sort_order = values(sort_order);

-- Website 媒体的 UMV 数据源使用 Similarweb。
update biz_resources
   set reference_source = 'Similarweb'
 where resource_type = '媒体'
   and trim(reference_source) = '';

-- 同名合作方视为同一主体；达人汇总粉丝数，媒体使用 UMV，
-- 再将合计值和统一层级回写到该主体的每个平台资源记录。
update biz_resources r
join (
  select resource_type,
         case
           when trim(name) = '' then concat('__resource__', id)
           else lower(trim(name))
         end as collaborator_key,
         case
           when resource_type = '媒体' then max(greatest(audience_size, 0))
           else sum(greatest(followers, 0))
         end as total_audience
    from biz_resources
   group by resource_type,
            case
              when trim(name) = '' then concat('__resource__', id)
              else lower(trim(name))
            end
) totals
  on totals.resource_type = r.resource_type
 and totals.collaborator_key = case
       when trim(r.name) = '' then concat('__resource__', r.id)
       else lower(trim(r.name))
     end
set r.audience_size = totals.total_audience,
    r.audience_size_unit = if(r.resource_type = '媒体', 'UMV', 'Followers'),
    r.tier = case
      when totals.total_audience <= 0 then ''
      when totals.total_audience > 1000000 then '头部'
      when totals.total_audience >= 100000 then '腰部'
      else '尾部'
    end;

-- ============================================================
-- Source: migrations/023_remote_content_image_urls.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_remote_content_image_url_column;

delimiter $$

create procedure add_remote_content_image_url_column()
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_resource_platform_posts'
       and column_name = 'cover_remote_url'
  ) then
    alter table biz_resource_platform_posts
      add column cover_remote_url text null after cover_url;
  end if;
end$$

delimiter ;

call add_remote_content_image_url_column();

drop procedure if exists add_remote_content_image_url_column;

-- Preserve remote URLs already stored in the legacy mixed-use cover_url column.
update biz_resource_platform_posts
   set cover_remote_url = cover_url
 where (cover_remote_url is null or trim(cover_remote_url) = '')
   and (cover_url like 'http://%' or cover_url like 'https://%');

-- ============================================================
-- Source: migrations/024_tiktok_post_metrics.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_tiktok_post_metrics_column;

delimiter $$

create procedure add_tiktok_post_metrics_column()
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_resource_platform_posts'
       and column_name = 'save_count'
  ) then
    alter table biz_resource_platform_posts
      add column save_count bigint not null default 0 after share_count;
  end if;
end$$

delimiter ;

call add_tiktok_post_metrics_column();

drop procedure if exists add_tiktok_post_metrics_column;

-- ============================================================
-- Source: migrations/025_project_multi_market.sql
-- ============================================================

use kol_admin;

alter table biz_projects
  modify column target_market text null;

-- ============================================================
-- Source: migrations/026_rename_project_management.sql
-- ============================================================

use kol_admin;

update sys_menus
set title = '项目管理'
where id = 903 or path = '/business/projects';

-- ============================================================
-- Source: migrations/027_x_website_media_metrics.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_media_metric_column;

delimiter $$

create procedure add_media_metric_column(
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_resources'
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table biz_resources add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

call add_media_metric_column('avatar_remote_url', 'avatar_remote_url varchar(1024) not null default '''' after avatar_url');
call add_media_metric_column('umv_month', 'umv_month varchar(7) not null default '''' after audience_size_unit');
call add_media_metric_column('umv_country', 'umv_country varchar(8) not null default '''' after umv_month');
call add_media_metric_column('umv_web_source', 'umv_web_source varchar(16) not null default '''' after umv_country');
call add_media_metric_column('umv_cross_device_deduplicated', 'umv_cross_device_deduplicated tinyint not null default 0 after umv_web_source');
call add_media_metric_column('monthly_visits', 'monthly_visits bigint not null default 0 after umv_cross_device_deduplicated');
call add_media_metric_column('monthly_page_views', 'monthly_page_views bigint not null default 0 after monthly_visits');
call add_media_metric_column('website_bounce_rate', 'website_bounce_rate decimal(8, 4) not null default 0 after monthly_page_views');
call add_media_metric_column('provider_updated_at', 'provider_updated_at datetime null after website_bounce_rate');

drop procedure if exists add_media_metric_column;

update biz_resources
   set audience_size = if(audience_size > 0, audience_size, followers),
       audience_size_unit = 'UMV',
       followers = 0
 where resource_type = '媒体';

insert ignore into biz_platform_sync_settings
  (platform, enabled, sync_profile, sync_posts, post_limit)
values
  ('X', 1, 1, 1, 20),
  ('Website', 1, 1, 0, 1);

-- ============================================================
-- Source: migrations/028_tikhub_linkedin_reddit.sql
-- ============================================================

use kol_admin;

insert ignore into biz_platform_sync_settings
  (platform, enabled, sync_profile, sync_posts, post_limit)
values
  ('Facebook', 0, 1, 1, 20),
  ('LinkedIn', 1, 1, 1, 20),
  ('Reddit', 1, 1, 1, 20);

-- ============================================================
-- Source: migrations/029_editable_project_content.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_project_content_column;

delimiter $$

create procedure add_project_content_column(
  in p_column_name varchar(64),
  in p_column_definition text
)
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_cooperations'
       and column_name = p_column_name
  ) then
    set @sql = concat('alter table biz_cooperations add column ', p_column_definition);
    prepare stmt from @sql;
    execute stmt;
    deallocate prepare stmt;
  end if;
end$$

delimiter ;

call add_project_content_column(
  'content_platform',
  'content_platform varchar(32) not null default '''' after cooperation_type'
);
call add_project_content_column(
  'content_cover_url',
  'content_cover_url text null after final_link'
);
call add_project_content_column(
  'content_cover_remote_url',
  'content_cover_remote_url text null after content_cover_url'
);

drop procedure if exists add_project_content_column;

-- ============================================================
-- Source: migrations/030_project_status_values.sql
-- ============================================================

use kol_admin;

alter table biz_projects
  modify column status varchar(32) not null default '未开始';

update biz_projects
set status = case
  when lower(trim(status)) in ('active') or trim(status) in ('进行中', '执行中') then '进行中'
  when lower(trim(status)) in ('completed') or trim(status) in ('已完成', '已结束', '结束') then '已结束'
  else '未开始'
end;

-- ============================================================
-- Source: migrations/031_standard_project_name_content_type.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_standard_project_template_column;

delimiter $$

create procedure add_standard_project_template_column()
begin
  if not exists (
    select 1
      from information_schema.columns
     where table_schema = database()
       and table_name = 'biz_cooperations'
       and column_name = 'content_type'
  ) then
    alter table biz_cooperations
      add column content_type varchar(64) not null default '' after cooperation_type;
  end if;
end$$

delimiter ;

call add_standard_project_template_column();

drop procedure if exists add_standard_project_template_column;

insert into biz_standard_import_options (field_key, value, status, source, sort_order)
values
  ('contentType', '生活记录类', '启用', '系统预置', 10),
  ('contentType', '娱乐搞笑类', '启用', '系统预置', 20),
  ('contentType', '兴趣圈层类', '启用', '系统预置', 30),
  ('contentType', '消费种草类', '启用', '系统预置', 40),
  ('contentType', '商业/品牌类', '启用', '系统预置', 50),
  ('contentType', '新闻资讯类', '启用', '系统预置', 60),
  ('contentType', '动画/创意类', '启用', '系统预置', 70),
  ('contentType', '短剧类', '启用', '系统预置', 80)
on duplicate key update
  status = '启用',
  source = '系统预置',
  sort_order = values(sort_order);

-- ============================================================
-- Source: migrations/032_sso_users.sql
-- ============================================================

use kol_admin;

drop procedure if exists add_sso_user_columns;

delimiter $$

create procedure add_sso_user_columns()
begin
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'auth_provider'
  ) then
    alter table sys_users add column auth_provider varchar(32) not null default 'local' after remark;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'external_subject'
  ) then
    alter table sys_users add column external_subject varchar(128) null after auth_provider;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'employee_no'
  ) then
    alter table sys_users add column employee_no varchar(64) not null default '' after external_subject;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'department_name'
  ) then
    alter table sys_users add column department_name varchar(128) not null default '' after employee_no;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'last_login_at'
  ) then
    alter table sys_users add column last_login_at datetime null after department_name;
  end if;
  if not exists (
    select 1 from information_schema.statistics
     where table_schema = database() and table_name = 'sys_users' and index_name = 'uk_sys_users_sso_identity'
  ) then
    alter table sys_users add unique key uk_sys_users_sso_identity (auth_provider, external_subject);
  end if;
end$$

delimiter ;

call add_sso_user_columns();

drop procedure if exists add_sso_user_columns;

-- ============================================================
-- Source: migrations/033_sso_default_role.sql
-- ============================================================

use kol_admin;

-- SSO users default to the non-administrator operations role.
insert into sys_roles (name, code, status, remark)
values ('运营', 'operation', 1, '企业 SSO 新用户默认角色')
on duplicate key update status = values(status);

-- The default role can use business features, but cannot access system or monitor menus.
insert ignore into sys_role_menus (role_id, menu_id)
select r.id, m.id
  from sys_roles r
  join sys_menus m on m.id = 900 or m.parent_id = 900
 where r.code = 'operation' and r.status = 1;

-- Repair SSO accounts created before default-role assignment was configurable.
insert ignore into sys_user_roles (user_id, role_id)
select u.id, r.id
  from sys_users u
  join sys_roles r on r.code = 'operation' and r.status = 1
 where u.auth_provider = 'uac'
   and not exists (
     select 1
       from sys_user_roles ur
       join sys_roles assigned_role on assigned_role.id = ur.role_id
      where ur.user_id = u.id and assigned_role.status = 1
   );

-- ============================================================
-- Source: migrations/034_project_name_options.sql
-- ============================================================

use kol_admin;

insert into biz_standard_import_options (field_key, value, status, source, sort_order)
values
  ('projectDivision', '总部_公关', '启用', '系统预置', 10),
  ('projectDivision', '总部_整合营销', '启用', '系统预置', 20),
  ('projectDivision', '总部_创意', '启用', '系统预置', 30),
  ('projectDivision', '总部_达人中台', '启用', '系统预置', 40),
  ('projectDivision', '区域', '启用', '系统预置', 50),
  ('projectProductLine', 'NOTE 60 Series', '启用', '系统预置', 10),
  ('projectProductLine', 'NOTE EDGE', '启用', '系统预置', 20),
  ('projectProductLine', 'GT 50 Pro', '启用', '系统预置', 30),
  ('projectProductLine', 'HOT 70 Series', '启用', '系统预置', 40),
  ('projectProductLine', 'ZClip2 Pro', '启用', '系统预置', 50),
  ('projectProductLine', 'XEH1', '启用', '系统预置', 60),
  ('projectProductLine', 'XPAD 30 Series', '启用', '系统预置', 70),
  ('projectProductLine', 'XPAD Edge', '启用', '系统预置', 80),
  ('projectProductLine', 'XBook B14', '启用', '系统预置', 90),
  ('projectProductLine', 'XBook 14 Neo', '启用', '系统预置', 100),
  ('projectProductLine', 'GTWatch 5 Pro', '启用', '系统预置', 110)
on duplicate key update
  status = '启用',
  sort_order = values(sort_order);
