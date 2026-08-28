package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Job struct {
    gorm.Model

    JobID       string          `json:"job_id" gorm:"primaryKey"`
    UserID      string          `json:"user_id" gorm:"not null;index"`
    Type        string          `json:"type" gorm:"not null"`
    Status      string          `json:"status" gorm:"not null;index"`
	Payload     json.RawMessage `json:"payload" gorm:"type:jsonb"`
    ResultKey   *string         `json:"result_key,omitempty"`
    Error       *string         `json:"error,omitempty"`
    ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
    StartedAt   *time.Time      `json:"started_at,omitempty"`
    CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

func (job *Job) BeforeCreate(tx *gorm.DB) error {
	if job.JobID == "" {
		jobID, err := uuid.NewV7()
		if err != nil {
			return errors.Wrap(err, "generate job UUIDv7")
		}
		job.JobID = jobID.String()
	}
	return nil
}

type Outbox struct {
    gorm.Model

    OutboxID    string          `json:"outbox_id" gorm:"primaryKey"`
    EventType   string          `json:"event_type" gorm:"not null"`
    Payload     json.RawMessage `json:"payload" gorm:"not null"`
    JobID       string          `json:"job_id" gorm:"not null;index"`
    PublishedAt *time.Time      `json:"published_at,omitempty"`
	Attempts    int				`json:"attempts" gorm:"default:0"`
	LastError   *string			`json:"last_error,omitempty"`
}

func (outbox *Outbox) BeforeCreate(tx *gorm.DB) error {
	if outbox.OutboxID == "" {
		outboxID, err := uuid.NewV7()
		if err != nil {
			return errors.Wrap(err, "generate outbox UUIDv7")
		}
		outbox.OutboxID = outboxID.String()
	}
	return nil
}

type IdempotencyKey struct {
    IdempotencyKey string          `json:"idempotency_key" gorm:"primaryKey"`
    JobID          string          `json:"job_id" gorm:"not null;index"`
    UserID         string          `json:"user_id" gorm:"not null;index"`
    Status         string          `json:"status" gorm:"not null"`
    Response       json.RawMessage `json:"response"`
    ExpiresAt      time.Time       `json:"expires_at" gorm:"not null;index"`
    gorm.Model
}

func (key *IdempotencyKey) BeforeCreate(tx *gorm.DB) error {
	if key.IdempotencyKey == "" {
		idempotencyKey, err := uuid.NewV7()
		if err != nil {
			return errors.Wrap(err, "generate idempotency key UUIDv7")
		}
		key.IdempotencyKey = idempotencyKey.String()
	}
	return nil
}



