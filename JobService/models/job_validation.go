package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusScheduled = "scheduled"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

var ValidStatuses = map[string]bool{
	StatusPending:   true,
	StatusRunning:   true,
	StatusCompleted: true,
	StatusFailed:    true,
	StatusCancelled: true,
	StatusScheduled: true,
}

// IsValidStatus checks if a given status string is a recognized job status.
func IsValidStatus(status string) bool {
	return ValidStatuses[status]
}

const (
	JobTypeSendEmail      = "SEND_EMAIL"
	JobTypeGenerateReport = "GENERATE_REPORT"
	JobTypeExtractData    = "EXTRACT_DATA"
)

var ValidJobTypes = map[string]bool{
	JobTypeSendEmail:      true,
	JobTypeGenerateReport: true,
	JobTypeExtractData:    true,
}

// IsValidJobType checks if a given job type is one of the supported job types (SEND_EMAIL, GENERATE_REPORT, EXTRACT_DATA).
func IsValidJobType(jobType string) bool {
	return ValidJobTypes[jobType]
}

var (
	// ErrValidationFailed is a sentinel error for validation failures.
	ErrValidationFailed = errors.New("validation failed")
	// ErrJobNotFound is a sentinel error for job record not found.
	ErrJobNotFound = errors.New("job not found")
	// ErrMissingIdempotencyKey is a sentinel error for missing Idempotency-Key header.
	ErrMissingIdempotencyKey = errors.New("missing Idempotency-Key header")
	// ErrInvalidJSON is a sentinel error for malformed request JSON.
	ErrInvalidJSON = errors.New("invalid JSON in request body")
	// ErrInvalidJobID is a sentinel error for invalid or empty job_id parameter.
	ErrInvalidJobID = errors.New("invalid job_id")
)

// ValidationErrors maps field names to their respective validation error messages.
type ValidationErrors map[string]string

// NewValidationErrors creates a new ValidationErrors map.
func NewValidationErrors() ValidationErrors {
	return make(ValidationErrors)
}

// Add adds a field error message to the validation errors.
func (v ValidationErrors) Add(field, message string) {
	v[field] = message
}

// HasErrors returns true if there are any validation errors.
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// Error formats all validation errors into a single human-readable string.
func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("validation failed: ")
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v[k])
	}
	return b.String()
}

// AsError converts ValidationErrors into a cockroachdb error with formatted details for each invalid field.
func (v ValidationErrors) AsError() error {
	if len(v) == 0 {
		return nil
	}
	err := errors.Wrap(ErrValidationFailed, v.Error())
	for field, msg := range v {
		err = errors.WithDetailf(err, "field '%s': %s", field, msg)
	}
	return err
}

// CreateJobRequest represents the payload for creating a new job.
// Note: job_id is intentionally omitted as it is generated server-side upon job creation (UUIDv7).
type CreateJobRequest struct {
	UserID      string          `json:"user_id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
}

// Validate validates the CreateJobRequest data.
func (r *CreateJobRequest) Validate() ValidationErrors {
	errs := NewValidationErrors()

	validateUserID(r.UserID, errs, true)
	validateJobType(r.Type, errs, true)
	validatePayload(r.Payload, errs, true)
	validateScheduledAt(r.ScheduledAt, errs)

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateWithError validates CreateJobRequest and returns a cockroachdb/errors error if validation fails.
func (r *CreateJobRequest) ValidateWithError() error {
	errs := r.Validate()
	if errs.HasErrors() {
		return errs.AsError()
	}
	return nil
}

// ToJob converts CreateJobRequest into a Job model instance with default pending status.
func (r *CreateJobRequest) ToJob() *Job {
	return &Job{
		UserID:      strings.TrimSpace(r.UserID),
		Type:        strings.TrimSpace(r.Type),
		Status:      StatusPending,
		Payload:     r.Payload,
		ScheduledAt: r.ScheduledAt,
	}
}

// UpdateJobRequest represents the payload for updating an existing job.
type UpdateJobRequest struct {
	Type        *string         `json:"type,omitempty"`
	Status      *string         `json:"status,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	ResultKey   *string         `json:"result_key,omitempty"`
	Error       *string         `json:"error,omitempty"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
}

// Validate validates the UpdateJobRequest data.
func (r *UpdateJobRequest) Validate() ValidationErrors {
	errs := NewValidationErrors()
	hasFields := false

	if r.Type != nil {
		hasFields = true
		validateJobType(*r.Type, errs, true)
	}

	if r.Status != nil {
		hasFields = true
		validateStatus(*r.Status, errs, true)
	}

	if len(r.Payload) > 0 {
		hasFields = true
		validatePayload(r.Payload, errs, false)
	}

	if r.ResultKey != nil {
		hasFields = true
		validateResultKey(r.ResultKey, errs)
	}

	if r.Error != nil {
		hasFields = true
	}

	if r.ScheduledAt != nil {
		hasFields = true
		if r.ScheduledAt.IsZero() {
			errs.Add("scheduled_at", "scheduled_at cannot be a zero timestamp")
		}
	}

	if !hasFields {
		errs.Add("body", "at least one field must be provided for update")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateWithError validates UpdateJobRequest and returns a cockroachdb/errors error if validation fails.
func (r *UpdateJobRequest) ValidateWithError() error {
	errs := r.Validate()
	if errs.HasErrors() {
		return errs.AsError()
	}
	return nil
}

// Validate validates the fields on the Job model directly.
// Note: JobID is not required during creation because it is automatically generated by the server (UUIDv7).
// If JobID is set (e.g. for existing records or updates), it is checked for validity.
func (job *Job) Validate() ValidationErrors {
	errs := NewValidationErrors()

	if job.JobID != "" {
		if err := ValidateJobID(job.JobID); err != nil {
			errs.Add("job_id", err.Error())
		}
	}

	validateUserID(job.UserID, errs, true)
	validateJobType(job.Type, errs, true)
	if job.Status != "" {
		validateStatus(job.Status, errs, true)
	}
	validatePayload(job.Payload, errs, true)
	validateScheduledAt(job.ScheduledAt, errs)
	validateResultKey(job.ResultKey, errs)

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateWithError validates Job model fields and returns a cockroachdb/errors error if validation fails.
func (job *Job) ValidateWithError() error {
	errs := job.Validate()
	if errs.HasErrors() {
		return errs.AsError()
	}
	return nil
}

// ValidateForUpdate validates that the job instance has a required JobID and valid fields for an update operation.
func (job *Job) ValidateForUpdate() ValidationErrors {
	errs := NewValidationErrors()

	if err := ValidateJobID(job.JobID); err != nil {
		errs.Add("job_id", err.Error())
	}

	if job.Status != "" {
		validateStatus(job.Status, errs, true)
	}
	if len(job.Payload) > 0 {
		validatePayload(job.Payload, errs, false)
	}
	validateScheduledAt(job.ScheduledAt, errs)
	validateResultKey(job.ResultKey, errs)

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateForUpdateWithError validates Job for update and returns a cockroachdb/errors error if validation fails.
func (job *Job) ValidateForUpdateWithError() error {
	errs := job.ValidateForUpdate()
	if errs.HasErrors() {
		return errs.AsError()
	}
	return nil
}

// ValidateJobID validates that a job ID string is non-empty and conforms to length constraints.
func ValidateJobID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return errors.Wrap(ErrInvalidJobID, "job_id is required")
	}
	if len(trimmed) > 36 {
		return errors.Wrapf(ErrInvalidJobID, "job_id must not exceed 36 characters (got %d)", len(trimmed))
	}
	return nil
}

// Helper validation functions

func validateUserID(userID string, errs ValidationErrors, required bool) {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		if required {
			errs.Add("user_id", "user_id is required")
		}
		return
	}
	if len(trimmed) > 36 {
		errs.Add("user_id", "user_id cannot exceed 36 characters")
	}
}

func validateJobType(jobType string, errs ValidationErrors, required bool) {
	trimmed := strings.TrimSpace(jobType)
	if trimmed == "" {
		if required {
			errs.Add("type", "type is required")
		}
		return
	}
	if !IsValidJobType(trimmed) {
		errs.Add("type", fmt.Sprintf("invalid type '%s', allowed values: %s, %s, %s",
			trimmed, JobTypeSendEmail, JobTypeGenerateReport, JobTypeExtractData))
	}
}

func validatePayload(payload json.RawMessage, errs ValidationErrors, required bool) {
	if len(payload) == 0 {
		if required {
			errs.Add("payload", "payload is required")
		}
		return
	}

	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		if required {
			errs.Add("payload", "payload is required")
		}
		return
	}

	if string(trimmed) == "null" {
		errs.Add("payload", "payload cannot be null")
		return
	}

	if !json.Valid(trimmed) {
		errs.Add("payload", "payload must be valid JSON")
		return
	}

	// Ensure payload is a JSON object or array (structured data)
	if trimmed[0] != '{' && trimmed[0] != '[' {
		errs.Add("payload", "payload must be a JSON object or array")
		return
	}
}

func validateScheduledAt(scheduledAt *time.Time, errs ValidationErrors) {
	if scheduledAt == nil {
		return
	}
	if scheduledAt.IsZero() {
		errs.Add("scheduled_at", "scheduled_at cannot be a zero timestamp")
		return
	}
	// Allow 1-minute buffer for clock skew
	if scheduledAt.Before(time.Now().Add(-1 * time.Minute)) {
		errs.Add("scheduled_at", "scheduled_at cannot be in the past")
	}
}


func validateStatus(status string, errs ValidationErrors, required bool) {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		if required {
			errs.Add("status", "status is required")
		}
		return
	}
	if !IsValidStatus(trimmed) {
		errs.Add("status", fmt.Sprintf("invalid status '%s', allowed values: %s, %s, %s, %s, %s",
			trimmed, StatusPending, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled))
	}
}

func validateResultKey(resultKey *string, errs ValidationErrors) {
	if resultKey == nil {
		return
	}
	if len(*resultKey) > 500 {
		errs.Add("result_key", "result_key cannot exceed 500 characters")
	}
}
