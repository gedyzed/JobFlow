package services

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/gedyzed/JobFlow/JobService/entities"
	"github.com/gedyzed/JobFlow/JobService/repositories"
	"github.com/gedyzed/JobFlow/JobService/models"
	"gorm.io/gorm"
)

type IJobService interface {
	CreateJob(job *models.Job, idempotencyKey string) (*models.Job, error)
	GetJobByID(id string) (*models.Job, error)
	ListJobs() ([]*models.Job, error)
	UpdateJob(job *models.Job) error
	DeleteJob(id string) error
	CancelJob(id string) error
	RetryJob(id string) error
}

type JobService struct {
	repo repositories.IJobRepo
	logger *slog.Logger
}

func NewJobService(repo repositories.IJobRepo, logger *slog.Logger) IJobService {
	return &JobService{
		repo:   repo,
		logger: logger,
	}
}

func (s *JobService) CreateJob(job *models.Job, idempotencyKey string) (*models.Job, error) {
	idempotencyRecord := &models.IdempotencyKey{
		IdempotencyKey: idempotencyKey,
		JobID:          job.JobID,
		UserID:         job.UserID,
		Status:         "pending",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}
	err := s.repo.CreateIdempotencyKey(idempotencyRecord)
	if err != nil {
		existingKey, getErr := s.repo.GetIdempotencyKeyByKey(idempotencyKey)
		if getErr != nil {
			return nil, &entities.SafeError{
				Code:     "idempotency_lookup_failed",
				UserMsg:  "Failed to resolve previous idempotent request",
				Internal: getErr,
				Metadata: map[string]any{"operation": "create_job", "idempotency_key": idempotencyKey},
			}
		}

		var previousJob models.Job
		if unmarshalErr := json.Unmarshal(existingKey.Response, &previousJob); unmarshalErr != nil {
			return nil, &entities.SafeError{
				Code:     "idempotency_response_decode_failed",
				UserMsg:  "Failed to decode previous idempotent response",
				Internal: unmarshalErr,
				Metadata: map[string]any{"operation": "create_job", "idempotency_key": idempotencyKey},
			}
		}

		return &previousJob, nil
	} 

	createdJob, err := s.repo.CreateJob(job)
	if err != nil {
		return nil, &entities.SafeError{
			Code:     "job_create_failed",
			UserMsg:  "Failed to create job",
			Internal: err,
			Metadata: map[string]any{"operation": "create_job", "job_id": job.JobID, "user_id": job.UserID},
		}
	}

	responsePayload, err := json.Marshal(createdJob)
	if err != nil {
		return nil, &entities.SafeError{
			Code:     "idempotency_response_encode_failed",
			UserMsg:  "Failed to store idempotent response",
			Internal: err,
			Metadata: map[string]any{"operation": "create_job", "idempotency_key": idempotencyKey},
		}
	}

	if err := s.repo.UpdateIdempotencyKeyResponse(idempotencyKey, responsePayload); err != nil {
		return nil, &entities.SafeError{
			Code:     "idempotency_response_update_failed",
			UserMsg:  "Failed to store idempotent response",
			Internal: err,
			Metadata: map[string]any{"operation": "create_job", "idempotency_key": idempotencyKey},
		}
	}

	return createdJob, nil
}

func (s *JobService) GetJobByID(id string) (*models.Job, error) {
	job, err := s.repo.GetJobByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &entities.SafeError{
				Code:     "job_not_found",
				UserMsg:  "Job not found",
				Internal: err,
				Metadata: map[string]any{"operation": "get_job_by_id", "job_id": id},
			}
		}

		return nil, &entities.SafeError{
			Code:     "job_fetch_failed",
			UserMsg:  "Failed to get job",
			Internal: err,
			Metadata: map[string]any{"operation": "get_job_by_id", "job_id": id},
		}
	}
	return job, nil
}

func (s *JobService) ListJobs() ([]*models.Job, error) {
	jobs, err := s.repo.ListJobs()
	if err != nil {
		return nil, &entities.SafeError{
			Code:     "job_list_failed",
			UserMsg:  "Failed to list jobs",
			Internal: err,
			Metadata: map[string]any{"operation": "list_jobs"},
		}
	}
	return jobs, nil
}

func (s *JobService) UpdateJob(job *models.Job) error {
	err := s.repo.UpdateJob(job)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entities.SafeError{
				Code:     "job_not_found",
				UserMsg:  "Job not found",
				Internal: err,
				Metadata: map[string]any{"operation": "update_job", "job_id": job.JobID},
			}
		}

		return &entities.SafeError{
			Code:     "job_update_failed",
			UserMsg:  "Failed to update job",
			Internal: err,
			Metadata: map[string]any{"operation": "update_job", "job_id": job.JobID},
		}
	}
	return nil
}

func (s *JobService) DeleteJob(id string) error {		
	err := s.repo.DeleteJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entities.SafeError{
				Code:     "job_not_found",
				UserMsg:  "Job not found",
				Internal: err,
				Metadata: map[string]any{"operation": "delete_job", "job_id": id},
			}
		}

		return &entities.SafeError{
			Code:     "job_delete_failed",
			UserMsg:  "Failed to delete job",
			Internal: err,
			Metadata: map[string]any{"operation": "delete_job", "job_id": id},
		}
	}
	return nil
}

func (s *JobService) CancelJob(id string) error {
	err := s.repo.CancelJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entities.SafeError{
				Code:     "job_not_found",
				UserMsg:  "Job not found",
				Internal: err,
				Metadata: map[string]any{"operation": "cancel_job", "job_id": id},
			}
		}

		return &entities.SafeError{
			Code:     "job_cancel_failed",
			UserMsg:  "Failed to cancel job",
			Internal: err,
			Metadata: map[string]any{"operation": "cancel_job", "job_id": id},
		}
	}
	return nil
}

func (s *JobService) RetryJob(id string) error {
	err := s.repo.RetryJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entities.SafeError{
				Code:     "job_not_found",
				UserMsg:  "Job not found",
				Internal: err,
				Metadata: map[string]any{"operation": "retry_job", "job_id": id},
			}
		}

		return &entities.SafeError{
			Code:     "job_retry_failed",
			UserMsg:  "Failed to retry job",
			Internal: err,
			Metadata: map[string]any{"operation": "retry_job", "job_id": id},
		}
	}
	return nil
}

