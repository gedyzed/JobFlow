package repositories

import (
	"encoding/json"
	"log/slog"

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
	db 		*gorm.DB
	logger  *slog.Logger
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
		return nil, err
	}
	return &job, nil
}

func (r *JobRepo) CreateJob(job *models.Job) (*models.Job, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return err
		}

		payload, err := json.Marshal(job)
		if err != nil {
			return err
		}

		outbox := &models.Outbox{
			EventType: "job.created",
			Payload:   payload,
			JobID:     job.JobID,
		}
		if err := tx.Create(outbox).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return job, nil
} 

func (r *JobRepo) ListJobs() ([]*models.Job, error) {
	jobs := make([]*models.Job, 0)
	if err := r.db.Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}	
func (r *JobRepo) UpdateJob(job *models.Job) error {
	return r.db.Save(job).Error
}
func (r *JobRepo) DeleteJob(id string) error {
	result := r.db.Delete(&models.Job{}, "job_id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *JobRepo) CancelJob(id string) error {
	return r.updateStatus(id, "cancelled")
}
func (r *JobRepo) RetryJob(id string) error {
	return r.updateStatus(id, "pending")
}

func (r *JobRepo) updateStatus(id string, status string) error {
	result := r.db.Model(&models.Job{}).
		Where("job_id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *JobRepo) CreateIdempotencyKey(idempotencyKey *models.IdempotencyKey) error {
	return r.db.Create(idempotencyKey).Error
}

func (r *JobRepo) GetIdempotencyKeyByKey(idempotencyKey string) (*models.IdempotencyKey, error) {
	var key models.IdempotencyKey
	if err := r.db.First(&key, "idempotency_key = ?", idempotencyKey).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *JobRepo) UpdateIdempotencyKeyResponse(idempotencyKey string, response json.RawMessage) error {
	return r.db.Model(&models.IdempotencyKey{}).
		Where("idempotency_key = ?", idempotencyKey).
		Updates(map[string]any{
			"response": response,
			"status":   "completed",
		}).Error
}
	