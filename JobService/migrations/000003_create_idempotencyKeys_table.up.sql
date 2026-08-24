CREATE TABLE idempotency_keys (
    id                  SERIAL NOT NULL,
    idempotency_key     VARCHAR(255) NOT NULL,
    job_id              VARCHAR(36) NOT NULL,
    user_id             VARCHAR(36) NOT NULL,
    status              VARCHAR(50) NOT NULL,
    response            JSONB NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at          TIMESTAMPTZ NULL,

    CONSTRAINT pk_idempotency_keys
        PRIMARY KEY (idempotency_key),

    CONSTRAINT fk_idempotency_keys_jobs
        FOREIGN KEY (job_id)
        REFERENCES jobs (job_id)
);

CREATE INDEX ix_idempotency_keys_job_id
    ON idempotency_keys (job_id);

CREATE INDEX ix_idempotency_keys_user_id
    ON idempotency_keys (user_id);

CREATE INDEX ix_idempotency_keys_expires_at
    ON idempotency_keys (expires_at);