use kol_admin;

create table if not exists biz_recommendation_subscriptions (
  id bigint primary key auto_increment,
  user_id bigint not null,
  domain varchar(16) not null,
  name varchar(128) not null default '',
  query_text text not null,
  filters json not null,
  active tinyint not null default 1,
  request_id bigint null,
  last_run_at datetime null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_rec_subscription_user (user_id, domain, active),
  index idx_rec_subscription_active (active, id)
);

create table if not exists biz_recommendation_runs (
  id bigint primary key auto_increment,
  subscription_id bigint not null,
  run_key varchar(80) not null,
  source varchar(16) not null default 'rule',
  created_at datetime not null default current_timestamp,
  unique key uk_rec_run (subscription_id, run_key),
  index idx_rec_run_created (created_at)
);

create table if not exists biz_recommendation_results (
  id bigint primary key auto_increment,
  subscription_id bigint not null,
  run_id bigint not null,
  resource_id bigint not null,
  resource_name varchar(255) not null,
  score int not null default 0,
  reason varchar(500) not null default '',
  risks varchar(500) not null default '',
  source varchar(16) not null default 'rule',
  feedback varchar(24) not null default '',
  is_new tinyint not null default 0,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_rec_result (subscription_id, resource_id),
  index idx_rec_result_subscription (subscription_id, created_at),
  index idx_rec_result_run (run_id)
);

create table if not exists biz_recommendation_notifications (
  id bigint primary key auto_increment,
  user_id bigint not null,
  domain varchar(16) not null,
  result_id bigint not null,
  title varchar(128) not null,
  description varchar(500) not null,
  target_path varchar(512) not null,
  read_at datetime null,
  created_at datetime not null default current_timestamp,
  unique key uk_rec_notice_result (result_id),
  index idx_rec_notice_user (user_id, read_at, created_at)
);
