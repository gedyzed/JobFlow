package repositories

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
	"gorm.io/gorm"
)

type IWorkerRepository interface {
	SaveJobResult(ctx context.Context, jobID string, result string) error
	GetJobResult(ctx context.Context, jobID string) (string, error)
	DeleteJobResult(ctx context.Context, jobID string) error
	UpdateJobStatus(ctx context.Context, jobID string, status string) error
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

func (r *WorkerRepository) SaveJobResult(ctx context.Context, jobID string, result string) error {
	var existing workerModels.JobResult
	err := r.db.WithContext(ctx).Where("job_id = ?", jobID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jobResult := &workerModels.JobResult{
				JobID:      jobID,
				ResultData: []byte(result),
				Status:     models.StatusCompleted,
			}
			if err := r.db.WithContext(ctx).Create(jobResult).Error; err != nil {
				return errors.Wrapf(err, "create job result for job %s", jobID)
			}
			r.logger.Info("Saved new job result", "job_id", jobID)
			return nil
		}
		return errors.Wrapf(err, "query job result for job %s", jobID)
	}

	existing.ResultData = []byte(result)
	existing.Status = models.StatusCompleted
	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return errors.Wrapf(err, "update existing job result for job %s", jobID)
	}
	r.logger.Info("Updated existing job result", "job_id", jobID)
	return nil
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

func (r *WorkerRepository) DeleteJobResult(ctx context.Context, jobID string) error {
	result := r.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&workerModels.JobResult{})
	if result.Error != nil {
		return errors.Wrapf(result.Error, "delete job result for job %s", jobID)
	}
	r.logger.Info("Deleted job result", "job_id", jobID, "rows_affected", result.RowsAffected)
	return nil
}

func (r *WorkerRepository) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	now := time.Now()
	if status == models.StatusCompleted {
		updates["completed_at"] = &now
	} else if status == models.StatusRunning {
		updates["started_at"] = &now
	}

	result := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("job_id = ?", jobID).
		Updates(updates)

	if result.Error != nil {
		return errors.Wrapf(result.Error, "update job status for job %s to %s", jobID, status)
	}
	r.logger.Info("Updated job status", "job_id", jobID, "status", status, "rows_affected", result.RowsAffected)
	return nil
}
