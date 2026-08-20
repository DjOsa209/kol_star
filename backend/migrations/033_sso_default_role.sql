use kol_admin;

-- SSO users default to the non-administrator operations role.
insert into sys_roles (name, code, status, remark)
values ('运营', 'operation', 1, '企业 SSO 新用户默认角色')
on duplicate key update status = values(status);

-- The default role can use business features, but cannot access system or monitor menus.
insert ignore into sys_role_menus (role_id, menu_id)
select r.id, m.id
  from sys_roles r
  join sys_menus m on m.id = 900 or m.parent_id = 900
 where r.code = 'operation' and r.status = 1;

-- Repair SSO accounts created before default-role assignment was configurable.
insert ignore into sys_user_roles (user_id, role_id)
select u.id, r.id
  from sys_users u
  join sys_roles r on r.code = 'operation' and r.status = 1
 where u.auth_provider = 'uac'
   and not exists (
     select 1
       from sys_user_roles ur
       join sys_roles assigned_role on assigned_role.id = ur.role_id
      where ur.user_id = u.id and assigned_role.status = 1
   );
