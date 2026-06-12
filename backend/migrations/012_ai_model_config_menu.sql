use kol_admin;

insert into sys_menus (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link) values
(908, 900, 0, 'AI模型配置', '/business/ai-model', 'BusinessAIModel', 'business/ai-model/index', null, 'ri:sparkling-2-line', '', 1)
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
select 1, id from sys_menus where id = 908;
