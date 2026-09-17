package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/models"
	workerModels "github.com/gedyzed/JobFlow/JobService/models/worker"
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

		if job.JobID == "" || job.Type == "" {
			w.logger.Warn("Ignoring invalid message: missing job_id or type", "job_id", job.JobID, "type", job.Type)
			return nil
		}

		w.logger.Info("Processing job", "job_id", job.JobID, "type", job.Type)

		// Deduplication check: check if already processed in job_results table
		existing, err := w.repo.GetJobResultRecord(ctx, job.JobID)
		if err != nil {
			w.logger.Error("Failed to query job result for deduplication", "job_id", job.JobID, "error", err)
			return err
		}
		if existing != nil && existing.Status == models.StatusCompleted {
			w.logger.Info("Job already completed, skipping duplicate execution", "job_id", job.JobID)
			return nil
		}

		// Record running status in worker's job_results table (does not touch Job table)
		if err := w.repo.SaveJobResultStatus(ctx, job.JobID, models.StatusRunning, workerModels.JobResult{
			JobID:   job.JobID,
			UserID:  job.UserID,
			JobType: job.Type,
			Status:  models.StatusRunning,
		}); err != nil {
			w.logger.Warn("Failed to record running status in job_results", "job_id", job.JobID, "error", err)
		}

		switch job.Type {
		case "SEND_EMAIL":
			if err := w.emailSender.SendEmail(ctx, job.Payload); err != nil {
				w.logger.Error("Failed to send email", "job_id", job.JobID, "error", err)
				// Save failed result + outbox event atomically
				if saveErr := w.repo.SaveJobResult(ctx, job.JobID, workerModels.JobResult{
					JobID:      job.JobID,
					UserID:     job.UserID,
					JobType:    job.Type,
					ResultData: json.RawMessage(fmt.Sprintf(`{"error": %q}`, err.Error())),
					Status:     models.StatusFailed,
					Error:      err.Error(),
				}); saveErr != nil {
					w.logger.Error("Failed to save failed job result", "job_id", job.JobID, "error", saveErr)
				}
				return nil
			}

			w.logger.Info("Email sent successfully", "job_id", job.JobID)
			// Save successful result + outbox event atomically
			if err := w.repo.SaveJobResult(ctx, job.JobID, workerModels.JobResult{
				JobID:      job.JobID,
				UserID:     job.UserID,
				JobType:    job.Type,
				ResultData: json.RawMessage(`{"status": "email_sent"}`),
				Status:     models.StatusCompleted,
			}); err != nil {
				w.logger.Warn("Failed to save job result", "job_id", job.JobID, "error", err)
			}
		default:
			w.logger.Warn("Unknown job type", "type", job.Type, "job_id", job.JobID)
		}

		return nil
	})
}
