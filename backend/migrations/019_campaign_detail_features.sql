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
