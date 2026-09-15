CREATE TABLE Job_Results (
    id     SERIAL NOT NULL,
    job_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    result_id VARCHAR(36),
    result_key VARCHAR(100),
    job_type VARCHAR(100),
    result_data JSONB NULL,
    status VARCHAR(50),
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ NULL,

    CONSTRAINT pk_jobs_result PRIMARY KEY (result_id),

    CONSTRAINT fk_jobs_result_jobs
        FOREIGN KEY (job_id)
        REFERENCES jobs (job_id)
 );

 CREATE INDEX ix_jobs_results_user_id
    ON Job_Results (user_id);

CREATE INDEX ix_job_results_status
    ON Job_Results (status);

CREATE INDEX ix_jobs_result_job_id 
    ON Job_Results (job_id);
