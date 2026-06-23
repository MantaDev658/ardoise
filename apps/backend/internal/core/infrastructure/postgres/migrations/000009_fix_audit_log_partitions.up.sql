-- The audit_logs table is range-partitioned by created_at, but only a single
-- partition (May 2026) was ever created. Once the clock passed 2026-06-01,
-- inserts had no matching partition and failed with
-- "no partition of relation \"audit_logs\" found for row", rolling back every
-- group expense / settle-up that writes an audit entry in the same transaction.

-- Catch-all partition so audit inserts can never fail on date again, regardless
-- of when the next explicit monthly partition is created.
CREATE TABLE IF NOT EXISTS audit_logs_default PARTITION OF audit_logs DEFAULT;

-- Explicit partition for the current month (was missing -> production failures).
CREATE TABLE IF NOT EXISTS audit_logs_y2026m06 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
