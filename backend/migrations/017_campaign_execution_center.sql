use kol_admin;

update sys_menus
set
  title = '营销项目执行中心',
  icon = 'ri:flow-chart'
where id = 903 or path = '/business/projects';
