package repositories

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IRelayRepo interface {
	FetchUnPublishedEvents(ctx context.Context, tx *gorm.DB, limit int) ([]models.Outbox, error)
	PollAndPublish(ctx context.Context, limit int) error
}

type Publisher interface {
	PublishEvent(ctx context.Context, event models.Outbox) error
}

type RelayRepo struct {
	db        *gorm.DB
	Publisher Publisher
	logger    *slog.Logger
}

func NewRelayRepo(db *gorm.DB, publisher Publisher, logger *slog.Logger) IRelayRepo {

	return &RelayRepo{
		db:        db,
		Publisher: publisher,
		logger:    logger,
	}
}
func (r *RelayRepo) FetchUnPublishedEvents(ctx context.Context, tx *gorm.DB, limit int) ([]models.Outbox, error) {

	var events []models.Outbox
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "SKIP LOCKED",
		}).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error

	return events, errors.Wrap(err, "Failed to fetch unpublished events")
}

func (r *RelayRepo) PollAndPublish(ctx context.Context, limit int) error {

	return r.db.Transaction(func(tx *gorm.DB) error {
		rows, err := r.FetchUnPublishedEvents(ctx, tx, limit)
		if err != nil {
			return errors.Wrap(err, "Failed to fetch unpublished events")
		}
		if len(rows) > 0 {
			r.logger.Info("fetched unpublished outbox events", "count", len(rows))
		} else {
			r.logger.Debug("no unpublished outbox events found in db")
		}
		for _, row := range rows {
			if err := r.Publisher.PublishEvent(ctx, row); err != nil {
				r.logger.Error("Failed to publish event", "error", err, "event_id", row.ID)
				continue
			}
			scheduledAt := time.Now()
			if err := tx.WithContext(ctx).
				Model(&models.Job{}).
				Where("job_id = ?", row.JobID).
				Updates(map[string]interface{}{
					"status":       models.StatusScheduled,
					"scheduled_at": scheduledAt,
				}).Error; err != nil {
				r.logger.Error("Failed to mark job as scheduled", "error", err, "job_id", row.JobID, "event_id", row.ID)
				continue
			}
			if err := tx.WithContext(ctx).
				Model(&models.Outbox{}).
				Where("id = ?", row.ID).
				Update("published", true).
				Error; err != nil {
				r.logger.Error("Failed to mark event as published", "error", err, "event_id", row.ID)
			} else {
				r.logger.Info("successfully published and marked outbox event", "event_id", row.ID, "event_type", row.EventType)
			}
		}
		return nil
	})
}
