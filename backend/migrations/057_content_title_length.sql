use kol_admin;

-- Preserve longer fetched content titles in both cooperation and report data.
-- The full report unique key is 2824 bytes with utf8mb4 (below InnoDB's 3072-byte limit).
-- Widening these columns preserves existing values and can safely be rerun.
alter table biz_cooperations
  modify column creative_name varchar(512) not null default '';
alter table biz_campaign_report_segments
  modify column creative_name varchar(512) not null default '';
