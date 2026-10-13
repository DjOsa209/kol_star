-- Convert the former 1..N recommendation rank to the T0/T1/T2 priority tiers.
-- Apply once, before deploying the corresponding API version.
use kol_admin;

update biz_ip_request_candidates
set priority_order = case
  when priority_order <= 1 then 0
  when priority_order = 2 then 1
  else 2
end;
