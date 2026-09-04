package controllers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/services"
	"gorm.io/gorm"
)

type JobServiceController struct {
	service services.IJobService
	logger  *slog.Logger
}

func NewJobServiceController(service services.IJobService, logger *slog.Logger) *JobServiceController {
	return &JobServiceController{
		service: service,
		logger:  logger,
	}
}

// ErrorResponse defines the standardized JSON error structure.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

// CreateJob handles the creation of a new job with request validation and idempotency key check.
func (c *JobServiceController) CreateJob(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		c.writeError(w, models.ErrMissingIdempotencyKey)
		return
	}

	var req models.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.writeError(w, errors.Wrap(models.ErrInvalidJSON, err.Error()))
		return
	}

	if valErrors := req.Validate(); valErrors.HasErrors() {
		c.writeValidationError(w, valErrors)
		return
	}

	job := req.ToJob()
		createdJob, err := c.service.CreateJob(job, idempotencyKey)
	if err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createdJob)
}

// GetJobByID fetches a single job by its unique identifier.
func (c *JobServiceController) GetJobByID(w http.ResponseWriter, r *http.Request) {
	jobID := extractJobID(r)
	if err := models.ValidateJobID(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	job, err := c.service.GetJobByID(jobID)
	if err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// ListJobs fetches all jobs.
func (c *JobServiceController) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := c.service.ListJobs()
	if err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jobs)
}

// UpdateJob updates an existing job after validating the update payload.
func (c *JobServiceController) UpdateJob(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	jobID := extractJobID(r)
	if err := models.ValidateJobID(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	var req models.UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.writeError(w, errors.Wrap(models.ErrInvalidJSON, err.Error()))
		return
	}

	if valErrors := req.Validate(); valErrors.HasErrors() {
		c.writeValidationError(w, valErrors)
		return
	}

	job := &models.Job{
		JobID: jobID,
	}
	if req.Type != nil {
		job.Type = *req.Type
	}
	if req.Status != nil {
		job.Status = *req.Status
	}
	if len(req.Payload) > 0 {
		job.Payload = req.Payload
	}
	if req.ResultKey != nil {
		job.ResultKey = req.ResultKey
	}
	if req.Error != nil {
		job.Error = req.Error
	}
	if req.ScheduledAt != nil {
		job.ScheduledAt = req.ScheduledAt
	}

	if err := c.service.UpdateJob(job); err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Job updated successfully",
		"job_id":  jobID,
	})
}

// DeleteJob deletes a job by its unique identifier.
func (c *JobServiceController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := extractJobID(r)
	if err := models.ValidateJobID(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	if err := c.service.DeleteJob(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Job deleted successfully",
		"job_id":  jobID,
	})
}

// CancelJob marks a job as cancelled.
func (c *JobServiceController) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := extractJobID(r)
	if err := models.ValidateJobID(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	if err := c.service.CancelJob(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Job cancelled successfully",
		"job_id":  jobID,
	})
}

// RetryJob resets a job status to pending for retry.
func (c *JobServiceController) RetryJob(w http.ResponseWriter, r *http.Request) {
	jobID := extractJobID(r)
	if err := models.ValidateJobID(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	if err := c.service.RetryJob(jobID); err != nil {
		c.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Job retried successfully",
		"job_id":  jobID,
	})
}

// writeError handles error formatting, status code mapping, logging with stack traces, and response rendering.
func (c *JobServiceController) writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	status := http.StatusInternalServerError
	userMessage := "Internal server error"

	switch {
	case errors.Is(err, models.ErrMissingIdempotencyKey):
		status = http.StatusBadRequest
		userMessage = "Idempotency-Key header is required"

	case errors.Is(err, models.ErrInvalidJSON):
		status = http.StatusBadRequest
		userMessage = "Invalid JSON in request body"

	case errors.Is(err, models.ErrInvalidJobID):
		status = http.StatusBadRequest
		userMessage = err.Error()

	case errors.Is(err, models.ErrValidationFailed):
		status = http.StatusBadRequest
		userMessage = "Validation failed"

	case errors.Is(err, models.ErrJobNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		status = http.StatusNotFound
		userMessage = "Job not found"

	default:
		errStr := err.Error()
		if strings.Contains(errStr, "not found") {
			status = http.StatusNotFound
			userMessage = "Resource not found"
		} else if strings.Contains(errStr, "required") || strings.Contains(errStr, "invalid") {
			status = http.StatusBadRequest
			userMessage = err.Error()
		}
	}

	if status >= 500 {
		c.logger.Error("Server error",
			"error", err,
			"error_detail", fmt.Sprintf("%+v", err),
		)
	} else {
		c.logger.Warn("Client error",
			"status", status,
			"error", err,
			"error_detail", fmt.Sprintf("%+v", err),
		)
	}

	writeJSON(w, status, ErrorResponse{
		Error: userMessage,
	})
}

// writeValidationError handles validation failure logging with details and responds with 400 Bad Request.
func (c *JobServiceController) writeValidationError(w http.ResponseWriter, valErrors models.ValidationErrors) {
	valErr := valErrors.AsError()
	c.logger.Warn("Validation failed",
		"errors", valErrors,
		"error_detail", fmt.Sprintf("%+v", valErr),
	)

	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "Validation failed",
		Details: valErrors,
	})
}

// Helper methods

func extractJobID(r *http.Request) string {
	if id := r.PathValue("id"); id != "" {
		return id
	}
	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
