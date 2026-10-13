use kol_admin;

-- Keep the recommendation route available for users who can manage IP requests,
-- review candidates, or supplement marketing opinions. This also repairs
-- environments where migration 046 was not applied.
insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (925, 920, 0, 'menus.ipRecommendations', '/business/ip/recommendations',
   'IPRecommendations', 'business/ip/recommendations/index', 2,
   'ri:sparkling-2-line', '', 1)
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
select distinct rm.role_id, m.id
from sys_role_menus rm
join sys_menus m on m.id in (920, 925)
where rm.menu_id in (922, 923, 924);
