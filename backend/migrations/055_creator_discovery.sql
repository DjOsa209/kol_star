use kol_admin;

create table if not exists biz_creator_discovery_jobs (
  id bigint primary key auto_increment,
  user_id bigint not null,
  market varchar(100) not null,
  query_text text not null,
  queries json not null,
  max_candidates int not null default 12,
  status varchar(24) not null default 'queued',
  message text not null,
  recommendation json null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  index idx_creator_discovery_user (user_id, id),
  index idx_creator_discovery_status (status)
);

create table if not exists biz_creator_discovery_candidates (
  id bigint primary key auto_increment,
  job_id bigint not null,
  channel_id varchar(128) not null,
  snapshot json not null,
  status varchar(24) not null default 'pending',
  resource_id bigint null,
  country varchar(100) not null default '',
  language varchar(100) not null default '',
  note text null,
  evidence_url varchar(1024) not null default '',
  reviewed_by bigint null,
  reviewed_at datetime null,
  created_at datetime not null default current_timestamp,
  updated_at datetime not null default current_timestamp on update current_timestamp,
  unique key uk_creator_discovery_candidate (job_id, channel_id),
  index idx_creator_discovery_review (job_id, status)
);

-- Serialize concurrent confirmations of the same stable platform identity.
create table if not exists biz_creator_discovery_identities (
  platform varchar(32) not null,
  channel_id varchar(128) not null,
  resource_id bigint null,
  primary key (platform, channel_id)
);
