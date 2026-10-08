use kol_admin;

alter table biz_ip_resources add column profile json null;
alter table biz_ip_requests add column brief json null, add column marketing_profile json null;
alter table biz_ip_request_candidates add column assessment json null;
