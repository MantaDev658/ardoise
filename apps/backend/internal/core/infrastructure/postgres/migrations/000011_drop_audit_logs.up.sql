-- The audit log feature has been removed. Drop the partitioned table; CASCADE
-- removes all child partitions (audit_logs_default, audit_logs_yYYYYmMM) and the
-- group_id index along with it.
DROP TABLE IF EXISTS audit_logs CASCADE;
