CREATE TABLE jobs (
    id              SERIAL NOT NULL,
    job_id          VARCHAR(36) NOT NULL,

    user_id         VARCHAR(36) NOT NULL,
    type            VARCHAR(100) NOT NULL,
    status          VARCHAR(50) NOT NULL,

    payload         JSONB NULL,

    result_key      VARCHAR(500) NULL,
    error           TEXT NULL,

    scheduled_at    TIMESTAMPTZ NULL,
    started_at      TIMESTAMPTZ NULL,
    completed_at    TIMESTAMPTZ NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMPTZ NULL,

    CONSTRAINT pk_jobs PRIMARY KEY (job_id)
);

CREATE INDEX ix_jobs_user_id
    ON jobs (user_id);

CREATE INDEX ix_jobs_status
    ON jobs (status);

CREATE INDEX ix_jobs_scheduled_at
    ON jobs (scheduled_at);