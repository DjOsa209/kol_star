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
