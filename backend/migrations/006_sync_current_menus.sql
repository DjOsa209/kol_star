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

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(900, 0, 0, '资源运营', '/business', '', '', 2, 'ri:global-line', '', 1),
(901, 900, 0, '全球资源库', '/business/resources', 'BusinessResources', 'business/resources/index', null, 'ri:contacts-book-3-line', '', 1),
(902, 900, 0, '标签体系', '/business/tags', 'BusinessTags', 'business/tags/index', null, 'ri:price-tag-3-line', '', 1),
(903, 900, 0, '项目合作', '/business/projects', 'BusinessProjects', 'business/projects/index', null, 'ri:briefcase-4-line', '', 1),
(904, 900, 0, 'Brief模板库', '/business/briefs', 'BusinessBriefs', 'business/briefs/index', null, 'ri:file-list-3-line', '', 1),
(905, 900, 0, '数据看板', '/business/dashboard', 'BusinessDashboard', 'business/dashboard/index', null, 'ri:bar-chart-box-line', '', 1),
(1000, 0, 0, 'menus.pureSysManagement', '/system', '', '', 14, 'ri:settings-3-line', '', 1),
(1001, 1000, 0, 'menus.pureUser', '/system/user/index', 'SystemUser', '', null, 'ri:admin-line', '', 1),
(1002, 1000, 0, 'menus.pureRole', '/system/role/index', 'SystemRole', '', null, 'ri:admin-fill', '', 1),
(1003, 1000, 0, 'menus.pureSystemMenu', '/system/menu/index', 'SystemMenu', '', null, 'ep:menu', '', 1),
(1004, 1000, 0, 'menus.pureDept', '/system/dept/index', 'SystemDept', '', null, 'ri:git-branch-line', '', 1),
(1100, 0, 0, 'menus.pureSysMonitor', '/monitor', '', '', 15, 'ep:monitor', '', 1),
(1101, 1100, 0, 'menus.pureOnlineUser', '/monitor/online-user', 'OnlineUser', 'monitor/online/index', null, 'ri:user-voice-line', '', 1),
(1102, 1100, 0, 'menus.pureLoginLog', '/monitor/login-logs', 'LoginLog', 'monitor/logs/login/index', null, 'ri:window-line', '', 1),
(1103, 1100, 0, 'menus.pureOperationLog', '/monitor/operation-logs', 'OperationLog', 'monitor/logs/operation/index', null, 'ri:history-fill', '', 1),
(1104, 1100, 0, 'menus.pureSystemLog', '/monitor/system-logs', 'SystemLog', 'monitor/logs/system/index', null, 'ri:file-search-line', '', 1),
(100101, 1001, 3, '新增用户', '', '', '', 1, '', 'system:user:add', 0),
(100102, 1001, 3, '修改用户', '', '', '', 2, '', 'system:user:edit', 0),
(100103, 1001, 3, '删除用户', '', '', '', 3, '', 'system:user:delete', 0),
(100104, 1001, 3, '上传头像', '', '', '', 4, '', 'system:user:upload', 0),
(100105, 1001, 3, '重置密码', '', '', '', 5, '', 'system:user:reset-password', 0),
(100106, 1001, 3, '分配角色', '', '', '', 6, '', 'system:user:assign-role', 0),
(100201, 1002, 3, '新增角色', '', '', '', 1, '', 'system:role:add', 0),
(100202, 1002, 3, '修改角色', '', '', '', 2, '', 'system:role:edit', 0),
(100203, 1002, 3, '删除角色', '', '', '', 3, '', 'system:role:delete', 0),
(100204, 1002, 3, '分配菜单权限', '', '', '', 4, '', 'system:role:menu', 0),
(100301, 1003, 3, '新增菜单', '', '', '', 1, '', 'system:menu:add', 0),
(100302, 1003, 3, '修改菜单', '', '', '', 2, '', 'system:menu:edit', 0),
(100303, 1003, 3, '删除菜单', '', '', '', 3, '', 'system:menu:delete', 0),
(100401, 1004, 3, '新增部门', '', '', '', 1, '', 'system:dept:add', 0),
(100402, 1004, 3, '修改部门', '', '', '', 2, '', 'system:dept:edit', 0),
(100403, 1004, 3, '删除部门', '', '', '', 3, '', 'system:dept:delete', 0)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  `rank` = values(`rank`),
  icon = values(icon),
  auths = values(auths),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus;
