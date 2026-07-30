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
