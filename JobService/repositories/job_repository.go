package repositories

import (
	"encoding/json"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	"gorm.io/gorm"
)

type IJobRepo interface {
	GetJobByID(id string) (*models.Job, error)
	CreateJob(job *models.Job) (*models.Job, error)
	CreateIdempotencyKey(idempotencyKey *models.IdempotencyKey) error
	GetIdempotencyKeyByKey(idempotencyKey string) (*models.IdempotencyKey, error)
	UpdateIdempotencyKeyResponse(idempotencyKey string, response json.RawMessage) error
	ListJobs() ([]*models.Job, error)
	UpdateJob(job *models.Job) error
	DeleteJob(id string) error
	CancelJob(id string) error
	RetryJob(id string) error
}

type JobRepo struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewJobRepo(db *gorm.DB, logger *slog.Logger) IJobRepo {
	return &JobRepo{
		db:     db,
		logger: logger,
	}
}

func (r *JobRepo) GetJobByID(id string) (*models.Job, error) {
	var job models.Job
	if err := r.db.First(&job, "job_id = ?", id).Error; err != nil {
		return nil, errors.Wrapf(err, "get job %s", id)
	}
	return &job, nil
}

func (r *JobRepo) CreateJob(job *models.Job) (*models.Job, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return errors.Wrap(err, "insert job")
		}

		payload, err := json.Marshal(job)
		if err != nil {
			return errors.Wrap(err, "marshal job for outbox")
		}

		outbox := &models.Outbox{
			EventType: "job.created",
			Payload:   payload,
			JobID:     job.JobID,
		}
		if err := tx.Create(outbox).Error; err != nil {
			return errors.Wrap(err, "insert outbox event")
		}

		return nil
	})
	
	if err != nil {
		return nil, errors.Wrap(err, "create job transaction")
	}

	return job, nil
}

func (r *JobRepo) ListJobs() ([]*models.Job, error) {
	jobs := make([]*models.Job, 0)
	if err := r.db.Find(&jobs).Error; err != nil {
		return nil, errors.Wrap(err, "list jobs")
	}
	return jobs, nil
}
func (r *JobRepo) UpdateJob(job *models.Job) error {
	result := r.db.Model(&models.Job{}).
		Where("job_id = ?", job.JobID).
		Updates(job)
	if result.Error != nil {
		return errors.Wrapf(result.Error, "update job %s", job.JobID)
	}
	if result.RowsAffected == 0 {
		return errors.Wrapf(gorm.ErrRecordNotFound, "update job %s", job.JobID)
	}
	return nil
}
func (r *JobRepo) DeleteJob(id string) error {
	result := r.db.Delete(&models.Job{}, "job_id = ?", id)
	if result.Error != nil {
		return errors.Wrapf(result.Error, "delete job %s", id)
	}
	if result.RowsAffected == 0 {
		return errors.Wrapf(gorm.ErrRecordNotFound, "delete job %s", id)
	}
	return nil
}
func (r *JobRepo) CancelJob(id string) error {
	return r.updateStatus(id, models.StatusCancelled)
}
func (r *JobRepo) RetryJob(id string) error {
	return r.updateStatus(id, models.StatusPending)
}

func (r *JobRepo) updateStatus(id string, status string) error {
	result := r.db.Model(&models.Job{}).
		Where("job_id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return errors.Wrapf(result.Error, "update job %s status to %s", id, status)
	}
	if result.RowsAffected == 0 {
		return errors.Wrapf(gorm.ErrRecordNotFound, "update job %s status to %s", id, status)
	}
	return nil
}

func (r *JobRepo) CreateIdempotencyKey(idempotencyKey *models.IdempotencyKey) error {
	if err := r.db.Create(idempotencyKey).Error; err != nil {
		return errors.Wrap(err, "create idempotency key")
	}
	return nil
}

func (r *JobRepo) GetIdempotencyKeyByKey(idempotencyKey string) (*models.IdempotencyKey, error) {
	var key models.IdempotencyKey
	if err := r.db.First(&key, "idempotency_key = ?", idempotencyKey).Error; err != nil {
		return nil, errors.Wrapf(err, "get idempotency key %s", idempotencyKey)
	}
	return &key, nil
}

func (r *JobRepo) UpdateIdempotencyKeyResponse(idempotencyKey string, response json.RawMessage) error {
	result := r.db.Model(&models.IdempotencyKey{}).
		Where("idempotency_key = ?", idempotencyKey).
		Updates(map[string]any{
			"response": response,
			"status":   "completed",
		})
	if result.Error != nil {
		return errors.Wrapf(result.Error, "update idempotency key %s response", idempotencyKey)
	}
	if result.RowsAffected == 0 {
		return errors.Wrapf(gorm.ErrRecordNotFound, "update idempotency key %s response", idempotencyKey)
	}
	return nil
}
	
