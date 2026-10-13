use kol_admin;

alter table biz_recommendation_results
  add column advice text null,
  add column advice_source varchar(16) not null default 'rule',
  add column scenarios json null,
  add column evidence json null,
  add column evidence_status varchar(16) not null default 'not_searched',
  add column advice_resource_hash char(64) not null default '';
