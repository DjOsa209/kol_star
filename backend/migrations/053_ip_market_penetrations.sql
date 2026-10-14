use kol_admin;

-- Percentages are optional for IPs created before this feature. The JSON
-- object is keyed by a market from biz_ip_resources.markets and stores 0–100.
alter table biz_ip_resources
  add column market_penetrations json null after markets;
