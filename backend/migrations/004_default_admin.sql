use kol_admin;

insert into sys_departments (id, parent_id, name, principal, phone, email, sort, status, remark)
values (103, 100, '研发部门', '小铭', '15888886789', 'rd@example.com', 1, 1, '研发团队')
on duplicate key update
  name = values(name),
  status = 1;

insert into sys_roles (id, name, code, status, remark)
values (1, '超级管理员', 'admin', 1, '超级管理员拥有最高权限')
on duplicate key update
  name = values(name),
  code = values(code),
  status = 1,
  remark = values(remark);

insert into sys_users
(id, dept_id, avatar, username, nickname, password_hash, phone, email, sex, status, remark)
values
(1, 103, 'https://avatars.githubusercontent.com/u/44761321', 'admin', '小铭', '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', '15888886789', 'admin@example.com', 0, 1, '管理员')
on duplicate key update
  dept_id = values(dept_id),
  avatar = values(avatar),
  username = values(username),
  nickname = values(nickname),
  password_hash = values(password_hash),
  phone = values(phone),
  email = values(email),
  sex = values(sex),
  status = 1,
  remark = values(remark);

insert ignore into sys_user_roles (user_id, role_id) values (1, 1);

insert ignore into sys_role_menus (role_id, menu_id)
select 1, id from sys_menus;
