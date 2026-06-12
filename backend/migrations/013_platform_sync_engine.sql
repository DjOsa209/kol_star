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
  raw_json json null,
  synced_at datetime not null default current_timestamp,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_resource_platform_post (resource_id, platform, platform_post_id),
  index idx_biz_resource_platform_posts_resource (resource_id),
  index idx_biz_resource_platform_posts_published (published_at),
  index idx_biz_resource_platform_posts_metrics (view_count, like_count)
);
