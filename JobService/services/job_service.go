package services

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/repositories"
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
	repo   repositories.IJobRepo
	logger *slog.Logger
}

func NewJobService(repo repositories.IJobRepo, logger *slog.Logger) IJobService {
	return &JobService{
		repo:   repo,
		logger: logger,
	}
}

func (s *JobService) CreateJob(job *models.Job, idempotencyKey string) (*models.Job, error) {
	if idempotencyKey == "" {
		return nil, errors.Wrap(models.ErrMissingIdempotencyKey, "service create job")
	}

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
			return nil, errors.Wrapf(getErr, "lookup idempotency key %s", idempotencyKey)
		}

		var previousJob models.Job
		if unmarshalErr := json.Unmarshal(existingKey.Response, &previousJob); unmarshalErr != nil {
			return nil, errors.Wrapf(unmarshalErr, "decode previous idempotent response for key %s", idempotencyKey)
		}

		return &previousJob, nil
	}

	if job.Status == "" {
		job.Status = models.StatusPending
	}

	createdJob, err := s.repo.CreateJob(job)
	if err != nil {
		return nil, errors.Wrap(err, "service create job")
	}

	responsePayload, err := json.Marshal(createdJob)
	if err != nil {
		return nil, errors.Wrap(err, "marshal job for idempotency response")
	}

	if err := s.repo.UpdateIdempotencyKeyResponse(idempotencyKey, responsePayload); err != nil {
		return nil, errors.Wrapf(err, "update idempotency key response %s", idempotencyKey)
	}

	return createdJob, nil
}

func (s *JobService) GetJobByID(id string) (*models.Job, error) {
	if err := models.ValidateJobID(id); err != nil {
		return nil, errors.Wrap(err, "service get job")
	}

	job, err := s.repo.GetJobByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Wrapf(models.ErrJobNotFound, "job %s", id)
		}
		return nil, errors.Wrapf(err, "service get job %s", id)
	}
	return job, nil
}

func (s *JobService) ListJobs() ([]*models.Job, error) {
	jobs, err := s.repo.ListJobs()
	if err != nil {
		return nil, errors.Wrap(err, "service list jobs")
	}
	return jobs, nil
}

func (s *JobService) UpdateJob(job *models.Job) error {
	if err := models.ValidateJobID(job.JobID); err != nil {
		return errors.Wrap(err, "service update job")
	}

	err := s.repo.UpdateJob(job)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrapf(models.ErrJobNotFound, "job %s", job.JobID)
		}
		return errors.Wrapf(err, "service update job %s", job.JobID)
	}
	return nil
}

func (s *JobService) DeleteJob(id string) error {
	if err := models.ValidateJobID(id); err != nil {
		return errors.Wrap(err, "service delete job")
	}

	err := s.repo.DeleteJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrapf(models.ErrJobNotFound, "job %s", id)
		}
		return errors.Wrapf(err, "service delete job %s", id)
	}
	return nil
}

func (s *JobService) CancelJob(id string) error {
	if err := models.ValidateJobID(id); err != nil {
		return errors.Wrap(err, "service cancel job")
	}

	err := s.repo.CancelJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrapf(models.ErrJobNotFound, "job %s", id)
		}
		return errors.Wrapf(err, "service cancel job %s", id)
	}
	return nil
}

func (s *JobService) RetryJob(id string) error {
	if err := models.ValidateJobID(id); err != nil {
		return errors.Wrap(err, "service retry job")
	}

	err := s.repo.RetryJob(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrapf(models.ErrJobNotFound, "job %s", id)
		}
		return errors.Wrapf(err, "service retry job %s", id)
	}
	return nil
}
