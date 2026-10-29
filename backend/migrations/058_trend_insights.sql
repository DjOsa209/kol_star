use kol_admin;

insert into sys_menus
  (id, parent_id, menu_type, title, path, name, component, `rank`, icon, auths, show_link)
values
  (910, 900, 0, 'menus.trendInsights', '/business/trends', 'BusinessTrends', 'business/trends/index', 6, 'ri:line-chart-line', '', 1)
on duplicate key update
  parent_id=values(parent_id), title=values(title), path=values(path),
  name=values(name), component=values(component), `rank`=values(`rank`),
  icon=values(icon), show_link=values(show_link);

-- Existing resource readers get the new read-only insight menu.
insert ignore into sys_role_menus (role_id, menu_id)
select distinct rm.role_id, 910
from sys_role_menus rm join sys_menus m on m.id=rm.menu_id
where m.path='/business/resources';
