package repositories

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"gorm.io/gorm"
)

type IWorkerRepository interface {
	SaveJobResult(ctx context.Context, jobID string, result workerModels.JobResult) error
	GetJobResult(ctx context.Context, jobID string) (string, error)
	GetJobResultRecord(ctx context.Context, jobID string) (*workerModels.JobResult, error)
	SaveJobResultStatus(ctx context.Context, jobID string, status string, result workerModels.JobResult) error
	DeleteJobResult(ctx context.Context, jobID string) error
}

type WorkerRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewWorkerRepository(db *gorm.DB, logger *slog.Logger) IWorkerRepository {
	return &WorkerRepository{
		db:     db,
		logger: logger,
	}
}

func (r *WorkerRepository) SaveJobResult(ctx context.Context, jobID string, result workerModels.JobResult) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing workerModels.JobResult
		err := tx.Where("job_id = ?", jobID).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				jobResult := &workerModels.JobResult{
					JobID:      jobID,
					UserID:     result.UserID,
					JobType:    result.JobType,
					ResultKey:  result.ResultKey,
					ResultData: result.ResultData,
					Status:     result.Status,
					Error:      result.Error,
				}
				if err := tx.Create(jobResult).Error; err != nil {
					return errors.Wrapf(err, "create job result for job %s", jobID)
				}
			} else {
				return errors.Wrapf(err, "query job result for job %s", jobID)
			}
		} else {
			existing.ResultData = result.ResultData
			existing.Status = result.Status
			existing.Error = result.Error
			if err := tx.Save(&existing).Error; err != nil {
				return errors.Wrapf(err, "update existing job result for job %s", jobID)
			}
		}

		// Create outbox event for worker result
		payload, err := json.Marshal(result)
		if err != nil {
			return errors.Wrap(err, "marshal job result for outbox")
		}
		outbox := &models.Outbox{
			EventType: "job.output.result",
			Payload:   payload,
			JobID:     jobID,
			Service:   "worker",
		}
		if err := tx.Create(outbox).Error; err != nil {
			return errors.Wrap(err, "create outbox event for worker result")
		}

		r.logger.Info("Saved job result with outbox event", "job_id", jobID, "status", result.Status)
		return nil
	})
}

func (r *WorkerRepository) GetJobResult(ctx context.Context, jobID string) (string, error) {
	var jobResult workerModels.JobResult
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).First(&jobResult).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.Wrapf(err, "job result not found for job %s", jobID)
		}
		return "", errors.Wrapf(err, "get job result for job %s", jobID)
	}
	return string(jobResult.ResultData), nil
}

func (r *WorkerRepository) GetJobResultRecord(ctx context.Context, jobID string) (*workerModels.JobResult, error) {
	var jobResult workerModels.JobResult
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).First(&jobResult).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "get job result record for job %s", jobID)
	}
	return &jobResult, nil
}

func (r *WorkerRepository) SaveJobResultStatus(ctx context.Context, jobID string, status string, result workerModels.JobResult) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing workerModels.JobResult
		err := tx.Where("job_id = ?", jobID).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				jobResult := &workerModels.JobResult{
					JobID:      jobID,
					UserID:     result.UserID,
					JobType:    result.JobType,
					ResultKey:  result.ResultKey,
					ResultData: result.ResultData,
					Status:     status,
					Error:      result.Error,
				}
				if err := tx.Create(jobResult).Error; err != nil {
					return errors.Wrapf(err, "create job result status for job %s", jobID)
				}
			} else {
				return errors.Wrapf(err, "query job result for job %s", jobID)
			}
		} else {
			existing.Status = status
			if result.Error != "" {
				existing.Error = result.Error
			}
			if err := tx.Save(&existing).Error; err != nil {
				return errors.Wrapf(err, "update job result status for job %s", jobID)
			}
		}

		// Create outbox event for running status update so JobService can notify the user
		outboxPayload, err := json.Marshal(workerModels.JobResult{
			JobID:   jobID,
			UserID:  result.UserID,
			JobType: result.JobType,
			Status:  status,
		})
		if err != nil {
			return errors.Wrap(err, "marshal job result status for outbox")
		}
		outbox := &models.Outbox{
			EventType: "job.output.running",
			Payload:   outboxPayload,
			JobID:     jobID,
			Service:   "worker",
		}
		if err := tx.Create(outbox).Error; err != nil {
			return errors.Wrap(err, "create outbox event for job status")
		}

		r.logger.Info("Saved job result status with outbox event", "job_id", jobID, "status", status)
		return nil
	})
}

func (r *WorkerRepository) DeleteJobResult(ctx context.Context, jobID string) error {
	result := r.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&workerModels.JobResult{})
	if result.Error != nil {
		return errors.Wrapf(result.Error, "delete job result for job %s", jobID)
	}
	r.logger.Info("Deleted job result", "job_id", jobID, "rows_affected", result.RowsAffected)
	return nil
}
