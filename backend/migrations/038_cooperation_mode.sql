use kol_admin;

drop procedure if exists add_cooperation_mode_columns;

delimiter $$

create procedure add_cooperation_mode_columns()
begin
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'biz_cooperations' and column_name = 'cooperation_mode'
  ) then
    alter table biz_cooperations
      add column cooperation_mode varchar(16) not null default 'single' after cooperation_type;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'biz_cooperations' and column_name = 'package_id'
  ) then
    alter table biz_cooperations
      add column package_id varchar(64) not null default '' after cooperation_mode;
  end if;
end$$

delimiter ;

call add_cooperation_mode_columns();

drop procedure if exists add_cooperation_mode_columns;
