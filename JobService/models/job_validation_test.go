package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
)

func TestCreateJobRequest_Validate_Success(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	req := CreateJobRequest{
		UserID:      "usr_1234567890",
		Type:        JobTypeSendEmail,
		Payload:     json.RawMessage(`{"recipient": "test@example.com", "template": "welcome"}`),
		ScheduledAt: &futureTime,
	}

	errs := req.Validate()
	if errs.HasErrors() {
		t.Fatalf("expected no validation errors, got: %v", errs)
	}
}

func TestCreateJobRequest_AllowedJobTypes(t *testing.T) {
	allowedTypes := []string{JobTypeSendEmail, JobTypeGenerateReport, JobTypeExtractData}

	for _, jobType := range allowedTypes {
		t.Run(jobType, func(t *testing.T) {
			req := CreateJobRequest{
				UserID:  "usr_123",
				Type:    jobType,
				Payload: json.RawMessage(`{"task": "run"}`),
			}
			errs := req.Validate()
			if errs.HasErrors() {
				t.Fatalf("expected job type %s to be valid, got: %v", jobType, errs)
			}
		})
	}

	invalidTypes := []string{"INVALID_TYPE", "send_email", "process_order", "SEND-EMAIL"}
	for _, jobType := range invalidTypes {
		t.Run("invalid_"+jobType, func(t *testing.T) {
			req := CreateJobRequest{
				UserID:  "usr_123",
				Type:    jobType,
				Payload: json.RawMessage(`{"task": "run"}`),
			}
			errs := req.Validate()
			if !errs.HasErrors() {
				t.Fatalf("expected job type %s to be rejected, but it passed", jobType)
			}
			if msg, ok := errs["type"]; !ok || !strings.Contains(msg, "allowed values: SEND_EMAIL, GENERATE_REPORT, EXTRACT_DATA") {
				t.Errorf("expected allowed values error message for type, got: %s", msg)
			}
		})
	}
}

func TestCreateJobRequest_JobIDNotPresentDuringCreation(t *testing.T) {
	// 1. Verify JSON containing job_id does not populate JobID and passes validation
	jsonInput := `{"job_id": "client-passed-id", "user_id": "usr_123", "type": "GENERATE_REPORT", "payload": {"report_id": 456}}`
	var req CreateJobRequest
	if err := json.Unmarshal([]byte(jsonInput), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if errs := req.Validate(); errs.HasErrors() {
		t.Fatalf("expected validation to pass without job_id, got: %v", errs)
	}

	// 2. Verify ToJob creates a Job with empty JobID so DB hook assigns server-generated UUID
	job := req.ToJob()
	if job.JobID != "" {
		t.Fatalf("expected job.JobID to be empty before DB creation, got: '%s'", job.JobID)
	}

	// 3. Verify Job.Validate passes when JobID is empty (creation state)
	if errs := job.Validate(); errs.HasErrors() {
		t.Fatalf("expected Job.Validate to pass with empty JobID, got: %v", errs)
	}
}

func TestCreateJobRequest_Validate_Success_ArrayPayload(t *testing.T) {
	req := CreateJobRequest{
		UserID:  "550e8400-e29b-41d4-a716-446655440000",
		Type:    JobTypeExtractData,
		Payload: json.RawMessage(`[1, 2, 3, 4]`),
	}

	errs := req.Validate()
	if errs.HasErrors() {
		t.Fatalf("expected no validation errors, got: %v", errs)
	}
}

func TestCreateJobRequest_Validate_MissingFields(t *testing.T) {
	req := CreateJobRequest{}

	errs := req.Validate()
	if !errs.HasErrors() {
		t.Fatal("expected validation errors, got none")
	}

	if _, ok := errs["user_id"]; !ok {
		t.Error("expected error for user_id")
	}
	if _, ok := errs["type"]; !ok {
		t.Error("expected error for type")
	}
	if _, ok := errs["payload"]; !ok {
		t.Error("expected error for payload")
	}
}

func TestCreateJobRequest_Validate_WhitespaceFields(t *testing.T) {
	req := CreateJobRequest{
		UserID:  "   ",
		Type:    " \t ",
		Payload: json.RawMessage(`   `),
	}

	errs := req.Validate()
	if !errs.HasErrors() {
		t.Fatal("expected validation errors, got none")
	}

	if _, ok := errs["user_id"]; !ok {
		t.Error("expected error for user_id with whitespace")
	}
	if _, ok := errs["type"]; !ok {
		t.Error("expected error for type with whitespace")
	}
	if _, ok := errs["payload"]; !ok {
		t.Error("expected error for payload with whitespace")
	}
}

func TestCreateJobRequest_Validate_LengthExceeded(t *testing.T) {
	req := CreateJobRequest{
		UserID:  strings.Repeat("u", 37),
		Type:    JobTypeSendEmail,
		Payload: json.RawMessage(`{"valid": true}`),
	}

	errs := req.Validate()
	if !errs.HasErrors() {
		t.Fatal("expected validation errors, got none")
	}

	if msg, ok := errs["user_id"]; !ok || !strings.Contains(msg, "36 characters") {
		t.Errorf("expected 36 characters length error for user_id, got: %s", msg)
	}
}

func TestCreateJobRequest_Validate_InvalidPayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     json.RawMessage
		expectedErr string
	}{
		{
			name:        "null payload",
			payload:     json.RawMessage(`null`),
			expectedErr: "cannot be null",
		},
		{
			name:        "malformed json",
			payload:     json.RawMessage(`{not_json}`),
			expectedErr: "valid JSON",
		},
		{
			name:        "primitive string json",
			payload:     json.RawMessage(`"just a string"`),
			expectedErr: "JSON object or array",
		},
		{
			name:        "primitive number json",
			payload:     json.RawMessage(`12345`),
			expectedErr: "JSON object or array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateJobRequest{
				UserID:  "usr_123",
				Type:    JobTypeSendEmail,
				Payload: tt.payload,
			}
			errs := req.Validate()
			if !errs.HasErrors() {
				t.Fatalf("expected error for %s, got none", tt.name)
			}
			if msg, ok := errs["payload"]; !ok || !strings.Contains(msg, tt.expectedErr) {
				t.Errorf("expected '%s' in error message, got '%s'", tt.expectedErr, msg)
			}
		})
	}
}

func TestCreateJobRequest_Validate_ScheduledAt(t *testing.T) {
	t.Run("past time", func(t *testing.T) {
		pastTime := time.Now().Add(-10 * time.Minute)
		req := CreateJobRequest{
			UserID:      "usr_123",
			Type:        JobTypeExtractData,
			Payload:     json.RawMessage(`{"a": 1}`),
			ScheduledAt: &pastTime,
		}
		errs := req.Validate()
		if msg, ok := errs["scheduled_at"]; !ok || !strings.Contains(msg, "in the past") {
			t.Errorf("expected past time error, got: %s", msg)
		}
	})

	t.Run("zero time", func(t *testing.T) {
		zeroTime := time.Time{}
		req := CreateJobRequest{
			UserID:      "usr_123",
			Type:        JobTypeExtractData,
			Payload:     json.RawMessage(`{"a": 1}`),
			ScheduledAt: &zeroTime,
		}
		errs := req.Validate()
		if msg, ok := errs["scheduled_at"]; !ok || !strings.Contains(msg, "zero timestamp") {
			t.Errorf("expected zero timestamp error, got: %s", msg)
		}
	})
}

func TestCreateJobRequest_ToJob(t *testing.T) {
	future := time.Now().Add(time.Hour)
	req := CreateJobRequest{
		UserID:      "  usr_123  ",
		Type:        "  SEND_EMAIL  ",
		Payload:     json.RawMessage(`{"to": "+1234567890"}`),
		ScheduledAt: &future,
	}

	job := req.ToJob()
	if job.UserID != "usr_123" {
		t.Errorf("expected trimmed user_id 'usr_123', got '%s'", job.UserID)
	}
	if job.Type != JobTypeSendEmail {
		t.Errorf("expected trimmed type '%s', got '%s'", JobTypeSendEmail, job.Type)
	}
	if job.Status != StatusPending {
		t.Errorf("expected status '%s', got '%s'", StatusPending, job.Status)
	}
	if job.ScheduledAt != &future {
		t.Errorf("expected scheduled_at to match")
	}
}

func TestUpdateJobRequest_Validate(t *testing.T) {
	t.Run("empty update", func(t *testing.T) {
		req := UpdateJobRequest{}
		errs := req.Validate()
		if msg, ok := errs["body"]; !ok || !strings.Contains(msg, "at least one field") {
			t.Errorf("expected empty body error, got: %s", msg)
		}
	})

	t.Run("valid type update", func(t *testing.T) {
		jobType := JobTypeGenerateReport
		req := UpdateJobRequest{
			Type: &jobType,
		}
		errs := req.Validate()
		if errs.HasErrors() {
			t.Fatalf("expected no error for valid type, got: %v", errs)
		}
	})

	t.Run("invalid type update", func(t *testing.T) {
		invalidType := "INVALID"
		req := UpdateJobRequest{
			Type: &invalidType,
		}
		errs := req.Validate()
		if msg, ok := errs["type"]; !ok || !strings.Contains(msg, "invalid type") {
			t.Errorf("expected invalid type error, got: %s", msg)
		}
	})

	t.Run("valid status update", func(t *testing.T) {
		status := StatusRunning
		req := UpdateJobRequest{
			Status: &status,
		}
		errs := req.Validate()
		if errs.HasErrors() {
			t.Fatalf("expected no error for valid status, got: %v", errs)
		}
	})

	t.Run("invalid status update", func(t *testing.T) {
		invalidStatus := "in_progress"
		req := UpdateJobRequest{
			Status: &invalidStatus,
		}
		errs := req.Validate()
		if msg, ok := errs["status"]; !ok || !strings.Contains(msg, "invalid status") {
			t.Errorf("expected invalid status error, got: %s", msg)
		}
	})

	t.Run("result key too long", func(t *testing.T) {
		longKey := strings.Repeat("k", 501)
		req := UpdateJobRequest{
			ResultKey: &longKey,
		}
		errs := req.Validate()
		if msg, ok := errs["result_key"]; !ok || !strings.Contains(msg, "500 characters") {
			t.Errorf("expected result_key length error, got: %s", msg)
		}
	})
}

func TestValidateJobID(t *testing.T) {
	if err := ValidateJobID("550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Errorf("expected valid UUID to pass, got: %v", err)
	}

	if err := ValidateJobID("  "); err == nil {
		t.Error("expected empty job ID to fail")
	}

	if err := ValidateJobID(strings.Repeat("x", 37)); err == nil {
		t.Error("expected long job ID to fail")
	}
}

func TestJob_Validate(t *testing.T) {
	job := Job{
		UserID:  "usr_123",
		Type:    JobTypeExtractData,
		Status:  StatusPending,
		Payload: json.RawMessage(`{"file": "video.mp4"}`),
	}

	errs := job.Validate()
	if errs.HasErrors() {
		t.Fatalf("expected valid job to pass, got: %v", errs)
	}

	invalidJob := Job{
		Type:   "UNKNOWN",
		Status: "invalid_status",
	}
	errs = invalidJob.Validate()
	if !errs.HasErrors() {
		t.Fatal("expected errors on invalid job, got none")
	}
	if _, ok := errs["type"]; !ok {
		t.Error("expected type validation error for invalid job type")
	}
}

func TestJob_ValidateForUpdate(t *testing.T) {
	// Job without JobID must fail update validation
	jobNoID := Job{
		Status: StatusRunning,
	}
	errs := jobNoID.ValidateForUpdate()
	if !errs.HasErrors() {
		t.Fatal("expected ValidateForUpdate to fail when JobID is empty")
	}
	if _, ok := errs["job_id"]; !ok {
		t.Error("expected error on job_id for empty ID during update")
	}

	// Job with valid JobID must pass update validation
	jobWithID := Job{
		JobID:  "550e8400-e29b-41d4-a716-446655440000",
		Status: StatusCompleted,
	}
	errs = jobWithID.ValidateForUpdate()
	if errs.HasErrors() {
		t.Fatalf("expected valid job with ID to pass update validation, got: %v", errs)
	}

	if err := jobWithID.ValidateForUpdateWithError(); err != nil {
		t.Fatalf("expected nil error for valid update, got: %v", err)
	}
}

func TestValidationErrors_CockroachErrorsIntegration(t *testing.T) {
	req := CreateJobRequest{}
	err := req.ValidateWithError()
	if err == nil {
		t.Fatal("expected ValidateWithError to return an error, got nil")
	}

	// Verify errors.Is matches ErrValidationFailed
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("expected error to match ErrValidationFailed, got: %v", err)
	}

	// Verify formatted error message and details
	errStr := err.Error()
	if !strings.Contains(errStr, "user_id is required") {
		t.Errorf("expected error to contain user_id requirement, got: %s", errStr)
	}

	// Valid request should return nil error
	future := time.Now().Add(time.Hour)
	validReq := CreateJobRequest{
		UserID:      "usr_123",
		Type:        JobTypeSendEmail,
		Payload:     json.RawMessage(`{"ok": true}`),
		ScheduledAt: &future,
	}
	if err := validReq.ValidateWithError(); err != nil {
		t.Fatalf("expected nil error on valid request, got: %v", err)
	}
}
