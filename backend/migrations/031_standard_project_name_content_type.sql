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
