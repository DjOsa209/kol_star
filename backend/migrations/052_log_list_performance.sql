use kol_admin;

-- The operation log list filters audit_version and orders by newest id.
-- The existing time-based index does not support that order.
create index idx_sys_operation_logs_feed
  on sys_operation_logs (audit_version, id);
