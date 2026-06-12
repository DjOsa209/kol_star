use kol_admin;

insert ignore into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
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
(100403, 1004, 3, '删除部门', '', '', '', 3, '', 'system:dept:delete', 0);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id in (
  100101, 100102, 100103, 100104, 100105, 100106,
  100201, 100202, 100203, 100204,
  100301, 100302, 100303,
  100401, 100402, 100403
);
