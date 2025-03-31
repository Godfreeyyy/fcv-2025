-- job-scheduler/migrations/001_initial_schema.sql

-- Create jobs table
CREATE TABLE IF NOT EXISTS jobs (
                                    id UUID PRIMARY KEY,
                                    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    payload JSONB NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
                               worker_id VARCHAR(50),
    retry_count INTEGER NOT NULL DEFAULT 0
    );

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);
CREATE INDEX IF NOT EXISTS idx_jobs_worker_id ON jobs (worker_id);
CREATE INDEX IF NOT EXISTS idx_jobs_priority_created_at ON jobs (priority DESC, created_at ASC);

-- Create views
CREATE OR REPLACE VIEW pending_jobs AS
SELECT * FROM jobs WHERE status = 'PENDING'
ORDER BY priority DESC, created_at ASC;

CREATE OR REPLACE VIEW scheduled_jobs AS
SELECT * FROM jobs WHERE status = 'SCHEDULED'
ORDER BY priority DESC, scheduled_at ASC;

CREATE OR REPLACE VIEW running_jobs AS
SELECT * FROM jobs WHERE status = 'RUNNING'
ORDER BY started_at ASC;

-- Create job statistics function
CREATE OR REPLACE FUNCTION get_job_statistics()
RETURNS TABLE (
    status VARCHAR(20),
    count BIGINT,
    avg_wait_time INTERVAL,
    avg_execution_time INTERVAL
) AS $$
BEGIN
RETURN QUERY
SELECT
    j.status,
    COUNT(*) as count,
        AVG(j.scheduled_at - j.created_at) as avg_wait_time,
        AVG(j.completed_at - j.started_at) as avg_execution_time
FROM jobs j
GROUP BY j.status;
END;
$$ LANGUAGE plpgsql;