use kol_admin;

-- Keep operator-reviewed classification outside the public source snapshot.
-- Guard each column so this migration can safely be rerun.
drop procedure if exists add_creator_discovery_review_column;
delimiter //
create procedure add_creator_discovery_review_column(in column_name_value varchar(64), in definition_value text)
begin
  if not exists (
    select 1 from information_schema.columns
    where table_schema=database() and table_name='biz_creator_discovery_candidates'
      and column_name=column_name_value
  ) then
    set @creator_discovery_alter=concat('alter table biz_creator_discovery_candidates add column ', definition_value);
    prepare creator_discovery_statement from @creator_discovery_alter;
    execute creator_discovery_statement;
    deallocate prepare creator_discovery_statement;
  end if;
end//
delimiter ;
call add_creator_discovery_review_column('industry', 'industry varchar(128) not null default '''' after language');
call add_creator_discovery_review_column('category', 'category varchar(128) not null default '''' after industry');
call add_creator_discovery_review_column('tag_ids', 'tag_ids json null after category');
drop procedure add_creator_discovery_review_column;

-- Only repair confirmed discovery links whose current channel identity still
-- matches. Existing avatars, manual tiers and other business fields are kept.
-- KOL and the current UI value 达人 share the same creator audience group.
update biz_resources r
join (
  select distinct resource_id,channel_id from biz_creator_discovery_candidates
  where status='approved' and resource_id is not null
) approved on approved.resource_id=r.id
join (
  select case when lower(trim(resource_type)) in ('kol','达人') then 'creator' else resource_type end as resource_kind,
         lower(trim(name)) as collaborator_key,
         sum(greatest(followers,0)) as total_audience
  from biz_resources
  where lower(trim(resource_type)) in ('kol','达人') and trim(name)<>''
  group by resource_kind,lower(trim(name))
) totals on totals.resource_kind='creator' and totals.collaborator_key=lower(trim(r.name))
set r.avatar_url=case
      when trim(r.avatar_url)='' and r.avatar_remote_url regexp '^https?://[^[:space:]/]+'
        then r.avatar_remote_url else r.avatar_url end,
    r.tier=case
      when trim(r.tier)<>'' then r.tier
      when totals.total_audience>1000000 then '头部'
      when totals.total_audience>=100000 then '腰部'
      when totals.total_audience>0 then '尾部'
      else '' end
where r.platform='YouTube' and lower(trim(r.resource_type)) in ('kol','达人')
  and approved.channel_id regexp '^UC[A-Za-z0-9_-]{22}$'
  and (r.platform_user_id=approved.channel_id or (
    r.platform_user_id='' and r.platform_url in (
      concat('https://www.youtube.com/channel/',approved.channel_id),
      concat('https://www.youtube.com/channel/',approved.channel_id,'/'),
      concat('https://youtube.com/channel/',approved.channel_id),
      concat('https://youtube.com/channel/',approved.channel_id,'/')
    )
  ))
  and ((trim(r.avatar_url)='' and r.avatar_remote_url regexp '^https?://[^[:space:]/]+')
    or (trim(r.tier)='' and totals.total_audience>0));
