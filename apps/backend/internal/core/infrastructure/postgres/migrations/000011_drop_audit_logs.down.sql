-- Recreate the audit_logs table as it stood after migrations 0006 + 0009:
-- a range-partitioned parent with a catch-all DEFAULT partition and the
-- group_id lookup index.
CREATE TABLE audit_logs (
    id UUID NOT NULL,
    group_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    action VARCHAR(255) NOT NULL,
    target_id VARCHAR(255),
    details TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE audit_logs_default PARTITION OF audit_logs DEFAULT;

CREATE INDEX idx_audit_logs_group_id ON audit_logs(group_id, created_at DESC);
