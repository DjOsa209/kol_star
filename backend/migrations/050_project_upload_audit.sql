use kol_admin;

-- 0 identifies projects created before uploader auditing was introduced.
alter table biz_projects
  add column created_by bigint not null default 0 after owner,
  add index idx_biz_projects_created_by (created_by, created_at);
