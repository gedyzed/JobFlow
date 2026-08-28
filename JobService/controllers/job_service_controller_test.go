package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
)

type mockJobService struct {
	createJobFn func(job *models.Job, idempotencyKey string) (*models.Job, error)
	getJobByIDFn func(id string) (*models.Job, error)
	listJobsFn   func() ([]*models.Job, error)
	updateJobFn  func(job *models.Job) error
	deleteJobFn  func(id string) error
	cancelJobFn  func(id string) error
	retryJobFn   func(id string) error
}

func (m *mockJobService) CreateJob(job *models.Job, idempotencyKey string) (*models.Job, error) {
	if m.createJobFn != nil {
		return m.createJobFn(job, idempotencyKey)
	}
	return job, nil
}

func (m *mockJobService) GetJobByID(id string) (*models.Job, error) {
	if m.getJobByIDFn != nil {
		return m.getJobByIDFn(id)
	}
	return &models.Job{JobID: id}, nil
}

func (m *mockJobService) ListJobs() ([]*models.Job, error) {
	if m.listJobsFn != nil {
		return m.listJobsFn()
	}
	return []*models.Job{}, nil
}

func (m *mockJobService) UpdateJob(job *models.Job) error {
	if m.updateJobFn != nil {
		return m.updateJobFn(job)
	}
	return nil
}

func (m *mockJobService) DeleteJob(id string) error {
	if m.deleteJobFn != nil {
		return m.deleteJobFn(id)
	}
	return nil
}

func (m *mockJobService) CancelJob(id string) error {
	if m.cancelJobFn != nil {
		return m.cancelJobFn(id)
	}
	return nil
}

func (m *mockJobService) RetryJob(id string) error {
	if m.retryJobFn != nil {
		return m.retryJobFn(id)
	}
	return nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestCreateJob_MissingIdempotencyKey(t *testing.T) {
	ctrl := NewJobServiceController(&mockJobService{}, newTestLogger())

	body := `{"user_id": "usr_1", "type": "SEND_EMAIL", "payload": {"to": "a@b.com"}}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	ctrl.CreateJob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Error, "Idempotency-Key") {
		t.Errorf("expected error message to mention Idempotency-Key, got: '%s'", resp.Error)
	}
}

func TestCreateJob_InvalidJSON(t *testing.T) {
	ctrl := NewJobServiceController(&mockJobService{}, newTestLogger())

	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString("{invalid_json}"))
	req.Header.Set("Idempotency-Key", "key-123")
	rec := httptest.NewRecorder()

	ctrl.CreateJob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Error, "Invalid JSON") {
		t.Errorf("expected error to contain 'Invalid JSON', got: '%s'", resp.Error)
	}
}

func TestCreateJob_ValidationFailure(t *testing.T) {
	ctrl := NewJobServiceController(&mockJobService{}, newTestLogger())

	body := `{"user_id": "", "type": "INVALID_TYPE", "payload": null}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", "key-123")
	rec := httptest.NewRecorder()

	ctrl.CreateJob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got: %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != "Validation failed" {
		t.Errorf("expected error 'Validation failed', got: '%s'", resp.Error)
	}
	if len(resp.Details) == 0 {
		t.Error("expected validation error details")
	}
}

func TestCreateJob_Success(t *testing.T) {
	mockSvc := &mockJobService{
		createJobFn: func(job *models.Job, idempotencyKey string) (*models.Job, error) {
			job.JobID = "job-001"
			return job, nil
		},
	}
	ctrl := NewJobServiceController(mockSvc, newTestLogger())

	body := `{"user_id": "usr_123", "type": "SEND_EMAIL", "payload": {"to": "user@example.com"}}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", "key-123")
	rec := httptest.NewRecorder()

	ctrl.CreateJob(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got: %d", rec.Code)
	}

	var created models.Job
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created job: %v", err)
	}
	if created.JobID != "job-001" {
		t.Errorf("expected job-001, got: %s", created.JobID)
	}
}

func TestGetJobByID_NotFound(t *testing.T) {
	mockSvc := &mockJobService{
		getJobByIDFn: func(id string) (*models.Job, error) {
			return nil, errors.Wrapf(models.ErrJobNotFound, "job %s", id)
		},
	}
	ctrl := NewJobServiceController(mockSvc, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/jobs/nonexistent-id", nil)
	rec := httptest.NewRecorder()

	ctrl.GetJobByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got: %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != "Job not found" {
		t.Errorf("expected 'Job not found', got: '%s'", resp.Error)
	}
}

func TestGetJobByID_InvalidID(t *testing.T) {
	ctrl := NewJobServiceController(&mockJobService{}, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/jobs/", nil)
	rec := httptest.NewRecorder()

	ctrl.GetJobByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got: %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Error, "job_id is required") {
		t.Errorf("expected error to contain 'job_id is required', got: '%s'", resp.Error)
	}
}
