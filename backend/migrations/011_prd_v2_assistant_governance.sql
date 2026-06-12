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
