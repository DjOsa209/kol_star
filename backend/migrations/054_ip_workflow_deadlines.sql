use kol_admin;

-- Older requests deliberately keep NULL: neither created_at nor updated_at
-- can establish when a draft was submitted or when IP review was completed.
alter table biz_ip_requests
  add column submitted_at datetime null,
  add column ip_reviewed_at datetime null,
  add column marketing_reviewed_at datetime null;

create table if not exists biz_ip_workflow_notifications (
  id bigint primary key auto_increment,
  request_id bigint not null,
  user_id bigint not null,
  stage varchar(32) not null,
  read_at datetime null,
  created_at datetime not null,
  unique key uk_ip_workflow_notice (request_id, user_id, stage),
  index idx_ip_workflow_user (user_id, read_at, id)
);
