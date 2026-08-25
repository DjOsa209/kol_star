use kol_admin;

insert into biz_standard_import_options (field_key, value, status, source, sort_order)
values
  ('projectDivision', '总部_公关', '启用', '系统预置', 10),
  ('projectDivision', '总部_整合营销', '启用', '系统预置', 20),
  ('projectDivision', '总部_创意', '启用', '系统预置', 30),
  ('projectDivision', '总部_达人中台', '启用', '系统预置', 40),
  ('projectDivision', '区域', '启用', '系统预置', 50),
  ('projectProductLine', 'NOTE 60 Series', '启用', '系统预置', 10),
  ('projectProductLine', 'NOTE EDGE', '启用', '系统预置', 20),
  ('projectProductLine', 'GT 50 Pro', '启用', '系统预置', 30),
  ('projectProductLine', 'HOT 70 Series', '启用', '系统预置', 40),
  ('projectProductLine', 'ZClip2 Pro', '启用', '系统预置', 50),
  ('projectProductLine', 'XEH1', '启用', '系统预置', 60),
  ('projectProductLine', 'XPAD 30 Series', '启用', '系统预置', 70),
  ('projectProductLine', 'XPAD Edge', '启用', '系统预置', 80),
  ('projectProductLine', 'XBook B14', '启用', '系统预置', 90),
  ('projectProductLine', 'XBook 14 Neo', '启用', '系统预置', 100),
  ('projectProductLine', 'GTWatch 5 Pro', '启用', '系统预置', 110)
on duplicate key update
  status = '启用',
  sort_order = values(sort_order);
