use kol_admin;

update sys_menus
set title = 'menus.creatorRecommendations'
where id = 906 and path = '/business/assistant';

update sys_menus set `rank` = 3 where id = 922 and parent_id = 920;
update sys_menus set `rank` = 4 where id = 923 and parent_id = 920;
update sys_menus set `rank` = 5 where id = 924 and parent_id = 920;

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (925, 920, 0, 'menus.ipRecommendations', '/business/ip/recommendations', 'IPRecommendations', 'business/ip/recommendations/index', 2, 'ri:sparkling-2-line', '', 1)
on duplicate key update
  parent_id = values(parent_id),
  title = values(title),
  path = values(path),
  name = values(name),
  component = values(component),
  `rank` = values(`rank`),
  icon = values(icon),
  show_link = values(show_link);

insert ignore into sys_role_menus (role_id, menu_id)
select r.id, 925
from sys_roles r
where r.code in ('admin', 'operation') and r.status = 1;
