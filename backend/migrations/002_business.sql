use kol_admin;

create table if not exists biz_resources (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  resource_type varchar(32) not null default 'KOL',
  media_outlet varchar(128) not null default '',
  tier varchar(32) not null default '',
  country varchar(64) not null default '',
  region varchar(64) not null default '',
  city varchar(64) not null default '',
  language varchar(64) not null default '',
  platform varchar(64) not null default '',
  industry varchar(128) not null default '',
  category varchar(128) not null default '',
  title varchar(128) not null default '',
  contact varchar(255) not null default '',
  owner varchar(64) not null default '',
  region_team varchar(64) not null default '',
  reference_source varchar(255) not null default '',
  shipping_address text null,
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
  website varchar(512) not null default '',
  import_source_sheet varchar(128) not null default '',
  platform_user_id varchar(128) not null default '',
  platform_handle varchar(128) not null default '',
  total_views bigint not null default 0,
  video_count bigint not null default 0,
  last_sync_status varchar(32) not null default '',
  last_sync_error text null,
  last_sync_at datetime null,
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
  target_market varchar(128) not null default '',
  language varchar(64) not null default '',
  platform varchar(64) not null default '',
  campaign_type varchar(64) not null default '',
  budget decimal(14, 2) not null default 0,
  currency varchar(16) not null default 'USD',
  status varchar(32) not null default '需求创建',
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
  views bigint not null default 0,
  clicks bigint not null default 0,
  conversions bigint not null default 0,
  engagement_count bigint not null default 0,
  comments_count bigint not null default 0,
  roi decimal(10, 4) not null default 0,
  team_rating int not null default 0,
  release_date date null,
  deliverable_links text null,
  import_batch_id varchar(64) not null default '',
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
