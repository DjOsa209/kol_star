use kol_admin;

drop procedure if exists add_sso_user_columns;

delimiter $$

create procedure add_sso_user_columns()
begin
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'auth_provider'
  ) then
    alter table sys_users add column auth_provider varchar(32) not null default 'local' after remark;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'external_subject'
  ) then
    alter table sys_users add column external_subject varchar(128) null after auth_provider;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'employee_no'
  ) then
    alter table sys_users add column employee_no varchar(64) not null default '' after external_subject;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'department_name'
  ) then
    alter table sys_users add column department_name varchar(128) not null default '' after employee_no;
  end if;
  if not exists (
    select 1 from information_schema.columns
     where table_schema = database() and table_name = 'sys_users' and column_name = 'last_login_at'
  ) then
    alter table sys_users add column last_login_at datetime null after department_name;
  end if;
  if not exists (
    select 1 from information_schema.statistics
     where table_schema = database() and table_name = 'sys_users' and index_name = 'uk_sys_users_sso_identity'
  ) then
    alter table sys_users add unique key uk_sys_users_sso_identity (auth_provider, external_subject);
  end if;
end$$

delimiter ;

call add_sso_user_columns();

drop procedure if exists add_sso_user_columns;
