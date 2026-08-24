package controllers

import (
	"log/slog"
	"net/http"
	"github.com/gedyzed/JobFlow/JobService/services"
	"encoding/json"
	"github.com/gedyzed/JobFlow/JobService/models"
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

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	createdJob, err := c.service.CreateJob(&job)
	if err != nil {
		c.logger.Error("Failed to create job:", "error", err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
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
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _,err := c.service.GetJobByID(job.JobID); err != nil {
		c.logger.Error("Failed to get job by ID:", "error", err)
		http.Error(w, "Failed to get job by ID", http.StatusInternalServerError)
		return
	}	

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)

}
func (c *JobServiceController) ListJobs(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := c.service.ListJobs(); err != nil {
		c.logger.Error("Failed to list jobs:", "error", err)
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}	


	w.Header().Set("Content-Type", "application/json")		
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)

}
func (c *JobServiceController) UpdateJob(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.service.UpdateJob(&job); err != nil {
		c.logger.Error("Failed to update job:", "error", err)
		http.Error(w, "Failed to update job", http.StatusInternalServerError)
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
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.service.DeleteJob(job.JobID); err != nil {
		c.logger.Error("Failed to delete job:", "error", err)
		http.Error(w, "Failed to delete job", http.StatusInternalServerError)
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
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.service.CancelJob(job.JobID); err != nil {
		c.logger.Error("Failed to cancel job:", "error", err)
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
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
		c.logger.Error("Failed to decode request body:", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.service.RetryJob(job.JobID); err != nil {
		c.logger.Error("Failed to retry job:", "error", err)
		http.Error(w, "Failed to retry job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}
