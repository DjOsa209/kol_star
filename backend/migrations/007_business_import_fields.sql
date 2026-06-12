use kol_admin;

alter table biz_cooperations
  add column views bigint not null default 0 after impressions,
  add column engagement_count bigint not null default 0 after conversions,
  add column comments_count bigint not null default 0 after engagement_count,
  add column release_date date null after team_rating,
  add column deliverable_links text null after release_date,
  add column import_batch_id varchar(64) not null default '' after deliverable_links;
