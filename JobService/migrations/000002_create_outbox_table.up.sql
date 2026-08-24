CREATE TABLE outboxes (
    id              SERIAL NOT NULL,
    outbox_id       VARCHAR(36) NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    payload         JSONB NOT NULL,
    job_id          VARCHAR(36) NOT NULL,
    published_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMPTZ NULL,
    Attempts        Int  DEFAULT 0,
	LastError       TEXT NULL,

    CONSTRAINT pk_outboxes PRIMARY KEY (outbox_id),

    CONSTRAINT fk_outboxes_jobs
        FOREIGN KEY (job_id)
        REFERENCES jobs (job_id)
);

CREATE INDEX ix_outboxes_job_id
    ON outboxes (job_id);

CREATE INDEX ix_outboxes_published_at
    ON outboxes (published_at);