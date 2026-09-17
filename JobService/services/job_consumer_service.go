package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"github.com/gedyzed/JobFlow/JobService/repositories"
)

// IJobConsumerService consumes worker result events from RabbitMQ and updates job records.
type IJobConsumerService interface {
	HandleWorkerResult(ctx context.Context, body []byte) error
}

// JobConsumerService processes worker output messages and updates job status in the database.
type JobConsumerService struct {
	repo   repositories.IJobRepo
	logger *slog.Logger
}

func NewJobConsumerService(repo repositories.IJobRepo, logger *slog.Logger) IJobConsumerService {
	return &JobConsumerService{
		repo:   repo,
		logger: logger,
	}
}

// HandleWorkerResult processes a worker result message, updating the corresponding job record.
func (s *JobConsumerService) HandleWorkerResult(ctx context.Context, body []byte) error {
	var result workerModels.JobResult
	if err := json.Unmarshal(body, &result); err != nil {
		s.logger.Error("failed to unmarshal worker result", "error", err)
		return errors.Wrap(err, "unmarshal worker result")
	}

	s.logger.Info("processing worker result", "job_id", result.JobID, "status", result.Status)

	existingJob, err := s.repo.GetJobByID(result.JobID)
	if err != nil {
		s.logger.Error("failed to find job for worker result", "job_id", result.JobID, "error", err)
		return errors.Wrapf(err, "find job %s for worker result", result.JobID)
	}

	// State machine protection:
	// If the job already reached a terminal state (completed or failed), ignore late-arriving running events
	if result.Status == models.StatusRunning {
		if existingJob.Status == models.StatusCompleted || existingJob.Status == models.StatusFailed {
			s.logger.Info("ignoring running status update for already finished job", "job_id", result.JobID, "current_status", existingJob.Status)
			return nil
		}
	}

	now := time.Now()
	job := &models.Job{
		JobID:  result.JobID,
		Status: result.Status,
	}

	if result.Status == models.StatusRunning {
		job.StartedAt = &now
	} else if result.Status == models.StatusCompleted || result.Status == models.StatusFailed {
		job.CompletedAt = &now
	}

	if result.Error != "" {
		job.Error = &result.Error
	}

	if result.ResultKey != "" {
		job.ResultKey = &result.ResultKey
	}

	if err := s.repo.UpdateJob(job); err != nil {
		s.logger.Error("failed to update job from worker result", "job_id", result.JobID, "error", err)
		return errors.Wrapf(err, "update job %s from worker result", result.JobID)
	}

	s.logger.Info("job updated from worker result", "job_id", result.JobID, "status", result.Status)
	return nil
}
