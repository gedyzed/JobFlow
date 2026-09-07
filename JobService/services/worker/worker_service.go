package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/gedyzed/JobFlow/JobService/models"
)

type IWorkerService interface {
	StartWorker(context.Context) error
	StopWorker(context.Context) error
	FetchPublshedJobs(ctx context.Context) error
}

type ISendEmail interface {
	SendEmail(ctx context.Context, payload interface{}) error
}

type WorkerService struct {
	logger      *slog.Logger
	rmqClient   models.IRMQClient
	emailSender ISendEmail
	s3Client models.IObjectStorage
}

func NewWorkerService(logger *slog.Logger, rmqClient models.IRMQClient, emailSender ISendEmail, s3Client models.IObjectStorage) IWorkerService {
	return &WorkerService{
		logger:      logger,
		rmqClient:   rmqClient,
		emailSender: emailSender,
		s3Client: s3Client,
	}
}

func (w *WorkerService) StartWorker(ctx context.Context) error {
	w.logger.Info("Worker started")

	return w.FetchPublshedJobs(ctx)
}

func (w *WorkerService) StopWorker(ctx context.Context) error {
	w.logger.Info("Worker stopped")
	return nil
}

func (w *WorkerService) FetchPublshedJobs(ctx context.Context) error {
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
		// Process job event / send email if needed
		time.Sleep(100 * time.Millisecond)
		return nil
	})
}
