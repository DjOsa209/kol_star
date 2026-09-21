use kol_admin;

create table if not exists biz_resource_metric_snapshots (
  id bigint primary key auto_increment,
  resource_id bigint not null,
  snapshot_week date not null,
  audience_size bigint not null default 0,
  average_views bigint not null default 0,
  average_interactions bigint not null default 0,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_resource_metric_snapshot_week (resource_id, snapshot_week),
  index idx_resource_metric_snapshot_lookup (resource_id, snapshot_week)
);
