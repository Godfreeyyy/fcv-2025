-- Database schema for job scheduler and execution service
-- migrations/001_initial_schema.sql

BEGIN;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types for job status and job type
CREATE TYPE job_status AS ENUM (
    'PENDING',    -- Job is waiting to be scheduled
    'SCHEDULED',  -- Job is scheduled but not yet running
    'RUNNING',    -- Job is currently running on a worker
    'COMPLETED',  -- Job completed successfully
    'FAILED',     -- Job execution failed
    'CANCELLED'   -- Job was cancelled before completion
);

-- Create jobs table
CREATE TABLE jobs (
                      id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                      type            VARCHAR(50) NOT NULL,               -- Type of job (e.g., 'java', 'python', 'cpp')
                      status          job_status NOT NULL,                -- Current status of the job
                      payload         JSONB NOT NULL,                     -- Job execution payload (code, inputs, etc.)
                      priority        INTEGER NOT NULL DEFAULT 0,         -- Job priority (higher values = higher priority)
                      created_at      TIMESTAMP WITH TIME ZONE NOT NULL,  -- When the job was created
                      scheduled_at    TIMESTAMP WITH TIME ZONE,           -- When the job was scheduled
                      started_at      TIMESTAMP WITH TIME ZONE,           -- When the job started execution
                      completed_at    TIMESTAMP WITH TIME ZONE,           -- When the job completed (success or failure)
                      worker_id       VARCHAR(50),                        -- ID of worker executing the job
                      retry_count     INTEGER NOT NULL DEFAULT 0,         -- Number of retry attempts
                      retry_limit     INTEGER NOT NULL DEFAULT 3,         -- Maximum retry attempts
                      timeout_seconds INTEGER NOT NULL DEFAULT 60,        -- Maximum execution time in seconds

    -- Add constraints
                      CONSTRAINT jobs_priority_check CHECK (priority >= 0),
                      CONSTRAINT jobs_retry_count_check CHECK (retry_count >= 0),
                      CONSTRAINT jobs_retry_limit_check CHECK (retry_limit >= 0),
                      CONSTRAINT jobs_timeout_seconds_check CHECK (timeout_seconds > 0)
);

-- Create job_results table to store execution results
CREATE TABLE job_results (
                             job_id          UUID PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
                             output          TEXT NOT NULL,                      -- Output of the job execution
                             error           TEXT,                               -- Error message, if any
                             completed_at    TIMESTAMP WITH TIME ZONE NOT NULL,  -- When the result was recorded
                             success         BOOLEAN NOT NULL,                   -- Whether the job succeeded
                             execution_time_ms INTEGER NOT NULL DEFAULT 0,       -- Execution time in milliseconds
                             memory_usage_bytes BIGINT NOT NULL DEFAULT 0        -- Memory usage in bytes
);

-- Create workers table
CREATE TABLE workers (
                         id              VARCHAR(50) PRIMARY KEY,            -- Worker identifier
                         type            VARCHAR(50) NOT NULL,               -- Type of worker (e.g., 'java', 'python', 'cpp')
                         capacity        INTEGER NOT NULL,                   -- Maximum concurrent jobs
                         current_load    INTEGER NOT NULL DEFAULT 0,         -- Current number of jobs being processed
                         last_heartbeat  TIMESTAMP WITH TIME ZONE NOT NULL,  -- Last heartbeat time
                         created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
                         ip_address      VARCHAR(45),                        -- IP address of the worker
                         version         VARCHAR(50),                        -- Version information

    -- Add constraints
                         CONSTRAINT workers_capacity_check CHECK (capacity > 0),
                         CONSTRAINT workers_current_load_check CHECK (current_load >= 0)
);

-- Create job_logs table for detailed job execution logs
CREATE TABLE job_logs (
                          id              BIGSERIAL PRIMARY KEY,
                          job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
                          log_time        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
                          log_level       VARCHAR(10) NOT NULL,               -- INFO, WARN, ERROR, etc.
                          message         TEXT NOT NULL,
                          metadata        JSONB
);

-- Create job_type_config table for configuring job types
CREATE TABLE job_type_config (
                                 type            VARCHAR(50) PRIMARY KEY,            -- Job type name
                                 description     TEXT,                               -- Description of the job type
                                 timeout_seconds INTEGER NOT NULL DEFAULT 60,        -- Default timeout for this job type
                                 retry_limit     INTEGER NOT NULL DEFAULT 3,         -- Default retry limit for this job type
                                 memory_limit_mb INTEGER NOT NULL DEFAULT 256,       -- Memory limit in MB
                                 cpu_limit       REAL NOT NULL DEFAULT 1.0,          -- CPU limit (1.0 = 1 core)
                                 active          BOOLEAN NOT NULL DEFAULT true,      -- Whether this job type is active
                                 created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
                                 updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- Create indexes for performance
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_worker_id ON jobs(worker_id);
CREATE INDEX idx_jobs_type_status ON jobs(type, status);
CREATE INDEX idx_jobs_priority_created_at ON jobs(priority DESC, created_at ASC);
CREATE INDEX idx_jobs_priority_scheduled_at ON jobs(priority DESC, scheduled_at ASC);
CREATE INDEX idx_workers_type ON workers(type);
CREATE INDEX idx_workers_last_heartbeat ON workers(last_heartbeat);
CREATE INDEX idx_job_logs_job_id ON job_logs(job_id);
CREATE INDEX idx_job_logs_log_time ON job_logs(log_time);

-- Create views for common queries
CREATE OR REPLACE VIEW pending_jobs AS
SELECT * FROM jobs WHERE status = 'PENDING'
ORDER BY priority DESC, created_at ASC;

CREATE OR REPLACE VIEW scheduled_jobs AS
SELECT * FROM jobs WHERE status = 'SCHEDULED'
ORDER BY priority DESC, scheduled_at ASC;

CREATE OR REPLACE VIEW running_jobs AS
SELECT * FROM jobs WHERE status = 'RUNNING'
ORDER BY started_at ASC;

CREATE OR REPLACE VIEW failed_jobs AS
SELECT * FROM jobs WHERE status = 'FAILED'
ORDER BY completed_at DESC;

CREATE OR REPLACE VIEW worker_status AS
SELECT
    w.id,
    w.type,
    w.capacity,
    w.current_load,
    w.last_heartbeat,
    (EXTRACT(EPOCH FROM (now() - w.last_heartbeat)) < 120) AS is_active,
    COUNT(j.id) AS assigned_jobs,
    w.capacity - w.current_load AS available_capacity
FROM
    workers w
        LEFT JOIN
    jobs j ON w.id = j.worker_id AND j.status = 'RUNNING'
GROUP BY
    w.id;

-- Create function to check for inactive workers
CREATE OR REPLACE FUNCTION cleanup_inactive_workers()
RETURNS INTEGER AS $$
DECLARE
inactive_threshold TIMESTAMP WITH TIME ZONE := now() - INTERVAL '5 minutes';
    deleted_count INTEGER := 0;
BEGIN
    -- Delete inactive workers
WITH deleted AS (
DELETE FROM workers
WHERE last_heartbeat < inactive_threshold
    RETURNING id
    )
SELECT COUNT(*) INTO deleted_count FROM deleted;

RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create function to retry failed jobs
CREATE OR REPLACE FUNCTION retry_failed_jobs(max_retries INTEGER DEFAULT 3)
RETURNS INTEGER AS $$
DECLARE
retried_count INTEGER := 0;
BEGIN
    -- Mark jobs for retry
WITH retried AS (
UPDATE jobs
SET
    status = 'PENDING',
    retry_count = retry_count + 1,
    scheduled_at = NULL,
    started_at = NULL,
    completed_at = NULL,
    worker_id = NULL
WHERE
    status = 'FAILED'
  AND retry_count < LEAST(retry_limit, max_retries)
    RETURNING id
    )
SELECT COUNT(*) INTO retried_count FROM retried;

RETURN retried_count;
END;
$$ LANGUAGE plpgsql;

-- Create function to assign jobs to workers
CREATE OR REPLACE FUNCTION assign_job_to_worker(
    p_job_id UUID,
    p_worker_id VARCHAR(50)
) RETURNS BOOLEAN AS $$
DECLARE
v_worker_type VARCHAR(50);
    v_job_type VARCHAR(50);
    v_capacity INTEGER;
    v_current_load INTEGER;
BEGIN
    -- Get worker information
SELECT type, capacity, current_load
INTO v_worker_type, v_capacity, v_current_load
FROM workers
WHERE id = p_worker_id
    FOR UPDATE;

-- Check if worker exists and has capacity
IF NOT FOUND OR v_current_load >= v_capacity THEN
        RETURN FALSE;
END IF;

    -- Get job type
SELECT type
INTO v_job_type
FROM jobs
WHERE id = p_job_id AND status = 'SCHEDULED'
    FOR UPDATE;

-- Check if job exists and is of correct type
IF NOT FOUND OR v_job_type <> v_worker_type THEN
        RETURN FALSE;
END IF;

    -- Update job
UPDATE jobs
SET
    status = 'RUNNING',
    worker_id = p_worker_id,
    started_at = now()
WHERE id = p_job_id;

-- Update worker load
UPDATE workers
SET current_load = current_load + 1
WHERE id = p_worker_id;

RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Add initial job types
INSERT INTO job_type_config (type, description, timeout_seconds, memory_limit_mb)
VALUES
    ('java', 'Java code execution', 30, 512),
    ('python', 'Python code execution', 30, 256),
    ('cpp', 'C++ code execution', 30, 256),
    ('csharp', 'C# code execution', 30, 512)
    ON CONFLICT (type) DO NOTHING;

COMMIT;

ALTER TABLE workers
    ADD COLUMN OS VARCHAR DEFAULT 'Linux';

DROP VIEW worker_status;

ALTER TABLE workers
    DROP COLUMN id;

ALTER TABLE workers
    ADD COLUMN id UUID PRIMARY KEY DEFAULT uuid_generate_v4();