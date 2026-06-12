use kol_admin;

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (909, 900, 0, '作品数据', '/business/resource-posts', 'BusinessResourcePosts', 'business/resource-posts/index', null, 'ri:video-line', '', 1)
on duplicate key update
  parent_id = values(parent_id),
  menu_type = values(menu_type),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  icon = values(icon),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus where id = 909;
