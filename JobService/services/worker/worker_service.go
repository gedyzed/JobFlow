package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/repositories"
)

type IWorkerService interface {
	StartWorker(context.Context) error
	StopWorker(context.Context) error
	ConsumeJobs(ctx context.Context) error
}

type ISendEmail interface {
	SendEmail(ctx context.Context, payload interface{}) error
}

type WorkerService struct {
	repo        repositories.IWorkerRepository
	logger      *slog.Logger
	rmqClient   models.IRMQClient
	emailSender ISendEmail
	s3Client    models.IObjectStorage
}

func NewWorkerService(repo repositories.IWorkerRepository, logger *slog.Logger, rmqClient models.IRMQClient, emailSender ISendEmail, s3Client models.IObjectStorage) IWorkerService {
	return &WorkerService{
		repo:        repo,
		logger:      logger,
		rmqClient:   rmqClient,
		emailSender: emailSender,
		s3Client:    s3Client,
	}
}

func (w *WorkerService) StartWorker(ctx context.Context) error {
	w.logger.Info("Worker started")
	return w.ConsumeJobs(ctx)
}

func (w *WorkerService) StopWorker(ctx context.Context) error {
	w.logger.Info("Worker stopped")
	return nil
}

func (w *WorkerService) ConsumeJobs(ctx context.Context) error {
	w.logger.Info("Fetching published jobs")
	consumer, err := w.rmqClient.NewConsumer(ctx, "job_queue")
	if err != nil {
		w.logger.Error("Failed to create consumer", "error", err)
		return err
	}
	defer consumer.Close(ctx)

	w.logger.Info("Consumer listening on job_queue")
	return consumer.Consume(ctx, func(ctx context.Context, body []byte) error {
		w.logger.Info("Received job event", "payload_size", len(body))

		var job models.Job
		if err := json.Unmarshal(body, &job); err != nil {
			w.logger.Error("Failed to unmarshal job event", "error", err)
			return errors.Wrap(err, "unmarshal job event")
		}

		w.logger.Info("Processing job", "job_id", job.JobID, "type", job.Type)

		// Update job status to running
		if err := w.repo.UpdateJobStatus(ctx, job.JobID, models.StatusRunning); err != nil {
			w.logger.Warn("Failed to update job status to running", "job_id", job.JobID, "error", err)
		}

		switch job.Type {
		case "SEND_EMAIL":
			if err := w.emailSender.SendEmail(ctx, job.Payload); err != nil {
				w.logger.Error("Failed to send email", "job_id", job.JobID, "error", err)
				_ = w.repo.UpdateJobStatus(ctx, job.JobID, models.StatusFailed)
				_ = w.repo.SaveJobResult(ctx, job.JobID, fmt.Sprintf(`{"error": %q}`, err.Error()))
				return nil
			}

			w.logger.Info("Email sent successfully", "job_id", job.JobID)
			if err := w.repo.UpdateJobStatus(ctx, job.JobID, models.StatusCompleted); err != nil {
				w.logger.Warn("Failed to update job status to completed", "job_id", job.JobID, "error", err)
			}
			if err := w.repo.SaveJobResult(ctx, job.JobID, `{"status": "email_sent"}`); err != nil {
				w.logger.Warn("Failed to save job result", "job_id", job.JobID, "error", err)
			}
		default:
			w.logger.Warn("Unknown job type", "type", job.Type, "job_id", job.JobID)
		}

		return nil
	})
}
