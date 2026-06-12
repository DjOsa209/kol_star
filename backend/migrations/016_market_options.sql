use kol_admin;

create table if not exists biz_market_options (
  id bigint primary key auto_increment,
  name varchar(128) not null,
  region_group varchar(64) not null default '',
  status varchar(16) not null default '启用',
  source varchar(32) not null default '系统预置',
  sort_order int not null default 100,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_biz_market_options_name (name),
  index idx_biz_market_options_status_sort (status, sort_order)
);

insert into biz_market_options
  (name, region_group, status, source, sort_order)
values
  ('美国', '欧美', '启用', '系统预置', 10),
  ('英国', '欧美', '启用', '系统预置', 20),
  ('欧洲', '欧美', '启用', '系统预置', 30),
  ('德国', '欧美', '启用', '系统预置', 40),
  ('日本', '亚太', '启用', '系统预置', 50),
  ('中东北非', 'MENA', '启用', '系统预置', 60),
  ('东非', '非洲', '启用', '系统预置', 70),
  ('西非', '非洲', '启用', '系统预置', 80),
  ('东南亚', '亚太', '启用', '系统预置', 90),
  ('拉美', '拉美', '启用', '系统预置', 100)
on duplicate key update
  region_group = values(region_group),
  sort_order = values(sort_order);
