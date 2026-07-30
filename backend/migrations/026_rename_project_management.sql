use kol_admin;

update sys_menus
set title = '项目管理'
where id = 903 or path = '/business/projects';
