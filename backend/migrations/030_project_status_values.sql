use kol_admin;

alter table biz_projects
  modify column status varchar(32) not null default '未开始';

update biz_projects
set status = case
  when lower(trim(status)) in ('active') or trim(status) in ('进行中', '执行中') then '进行中'
  when lower(trim(status)) in ('completed') or trim(status) in ('已完成', '已结束', '结束') then '已结束'
  else '未开始'
end;
