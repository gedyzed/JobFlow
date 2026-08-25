package controllers

import (
	"errors"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gedyzed/JobFlow/JobService/entities"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/services"
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

func (c *JobServiceController) CreateJob(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		c.writeError(w, &entities.SafeError{
			Code:     "missing_idempotency_key",
			UserMsg:  "Idempotency-Key header is required",	
			Internal: errors.New("missing Idempotency-Key header"),
			Metadata: map[string]any{"operation": "create_job", "method": r.Method},
		})
		return
	}

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "create_job", "method": r.Method},
		})
		return
	}

	createdJob, err := c.service.CreateJob(&job, idempotencyKey)
	if err != nil {
		c.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdJob)

}
func (c *JobServiceController) GetJobByID(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "get_job_by_id", "method": r.Method},
		})
		return
	}

	fetchedJob, err := c.service.GetJobByID(job.JobID)
	if err != nil {
		c.writeError(w, err)
		return
	}	

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fetchedJob)

}
func (c *JobServiceController) ListJobs(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "list_jobs", "method": r.Method},
		})
		return
	}

	jobs, err := c.service.ListJobs()
	if err != nil {
		c.writeError(w, err)
		return
	}	


	w.Header().Set("Content-Type", "application/json")		
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jobs)

}
func (c *JobServiceController) UpdateJob(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "update_job", "method": r.Method},
		})
		return
	}

	if err := c.service.UpdateJob(&job); err != nil {
		c.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)	
}
func (c *JobServiceController) DeleteJob(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "delete_job", "method": r.Method},
		})
		return
	}

	if err := c.service.DeleteJob(job.JobID); err != nil {
		c.writeError(w, err)
		return
	}	

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}
func (c *JobServiceController) CancelJob(w http.ResponseWriter, r *http.Request) {
	
	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "cancel_job", "method": r.Method},
		})
		return
	}

	if err := c.service.CancelJob(job.JobID); err != nil {
		c.writeError(w, err)
		return
	}	

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}
func (c *JobServiceController) RetryJob(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.writeError(w, &entities.SafeError{
			Code:     "invalid_request_body",
			UserMsg:  "Invalid request body",
			Internal: err,
			Metadata: map[string]any{"operation": "retry_job", "method": r.Method},
		})
		return
	}

	if err := c.service.RetryJob(job.JobID); err != nil {
		c.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}

func (c *JobServiceController) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	response := map[string]string{
		"code":    "internal_error",
		"userMsg": "An unexpected error occurred",
	}

	var safeErr *entities.SafeError
	if errors.As(err, &safeErr) {
		switch safeErr.Code {
		case "invalid_request_body":
			status = http.StatusBadRequest
		case "job_not_found":
			status = http.StatusNotFound
		}

		logAttrs := []any{"code", safeErr.Code, "error", safeErr.Internal}
		if safeErr.Metadata != nil {
			logAttrs = append(logAttrs, "metadata", safeErr.Metadata)
		}
		c.logger.Error(safeErr.UserMsg, logAttrs...)
		response = safeErr.Response()
	} else {
		c.logger.Error("Unhandled error", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
