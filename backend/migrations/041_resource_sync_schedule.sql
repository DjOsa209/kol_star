use kol_admin;

create table if not exists biz_resource_sync_schedule (
  id tinyint primary key,
  enabled tinyint not null default 0,
  frequency varchar(16) not null default 'weekly',
  weekday tinyint not null default 1,
  run_time char(5) not null default '02:00',
  timezone varchar(64) not null default 'Asia/Shanghai',
  last_run_key varchar(32) not null default '',
  last_started_at datetime null,
  updated_at datetime not null default current_timestamp on update current_timestamp
);

insert ignore into biz_resource_sync_schedule
  (id, enabled, frequency, weekday, run_time, timezone)
values
  (1, 0, 'weekly', 1, '02:00', 'Asia/Shanghai');
