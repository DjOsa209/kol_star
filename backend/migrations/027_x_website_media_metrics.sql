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
