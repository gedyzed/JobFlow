package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gorm.io/gorm"

	"github.com/gedyzed/JobFlow/JobService/controllers"
	infra "github.com/gedyzed/JobFlow/JobService/infra"
	configs "github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/repositories"
	"github.com/gedyzed/JobFlow/JobService/routes"
	"github.com/gedyzed/JobFlow/JobService/services"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load() // Load environment variables from .env file if it exists
	if err != nil {
		log.Println("Error loading .env file")
	}

	logDir := "logs"
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr != nil {
		log.Fatalf("failed to create log directory: %v", mkErr)
	}

	logPath := filepath.Join(logDir, "jobservice.txt")
	logFile, fileErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if fileErr != nil {
		log.Fatalf("failed to open log file %s: %v", logPath, fileErr)
	}
	defer logFile.Close()

	// setup logger
	handler := slog.NewJSONHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := configs.LoadJobServiceConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize the database connection with retry
	var db *gorm.DB
	if err := infra.Retry(ctx, "Database", 5, 5 * time.Second, func() (dbErr error) {
		db, dbErr = infra.DBInit(cfg.DB)
		return dbErr
	}); err != nil {
		slog.Error("Failed to initialize database connection after retries", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	// Initialize RabbitMQ for consuming worker results
	rMqService := infra.NewRabbitMQService(cfg.RabbitMQ, logger)
	rmqClient := infra.NewRMQClient(rMqService)

	if err := infra.Retry(ctx, "RabbitMQ", 5, 5*time.Second, func() error {
		return rmqClient.Connect(ctx)
	}); err != nil {
		slog.Error("Failed to connect to RabbitMQ after retries", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rmqClient.Close(closeCtx); err != nil {
			slog.Error("Failed to cleanly close RabbitMQ client", "error", err)
		}
	}()

	// Setup topic exchange and output queue for consuming worker results
	exchangeName := cfg.RabbitMQ.Exchange
	if exchangeName == "" {
		exchangeName = "job_exchange"
	}
	if err := rmqClient.DeclareExchange(ctx, exchangeName, "topic"); err != nil {
		slog.Error("Failed to declare topic exchange", "exchange", exchangeName, "error", err)
		os.Exit(1)
	}

	outputQueueName := "job_output_queue"
	if err := rmqClient.DeclareQueue(ctx, outputQueueName); err != nil {
		slog.Error("Failed to declare output queue", "queue", outputQueueName, "error", err)
		os.Exit(1)
	}
	if err := rmqClient.BindQueue(ctx, outputQueueName, exchangeName, "job.output.#"); err != nil {
		slog.Error("Failed to bind output queue to exchange", "queue", outputQueueName, "exchange", exchangeName, "error", err)
		os.Exit(1)
	}

	rmqConsumer, err := rmqClient.NewConsumer(ctx, outputQueueName)
	if err != nil {
		slog.Error("Failed to create consumer for worker results", "queue", outputQueueName, "error", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rmqConsumer.Close(closeCtx); err != nil {
			slog.Error("Failed to cleanly close consumer", "error", err)
		}
	}()

	// Initialize the job repository
	jobRepo := repositories.NewJobRepo(db, logger)

	// Initialize the jobservice
	jobService := services.NewJobService(jobRepo, logger)

	// Initialize the job consumer service for processing worker results
	consumerService := services.NewJobConsumerService(jobRepo, logger)

	// Start consuming worker results in background
	go func() {
		slog.Info("Starting worker result consumer", "queue", outputQueueName, "exchange", exchangeName)
		if err := rmqConsumer.Consume(ctx, consumerService.HandleWorkerResult); err != nil {
			slog.Error("Worker result consumer stopped with error", "error", err)
		}
	}()

	// Initialize the controllers
	jobController := controllers.NewJobServiceController(jobService, logger)

	router := routes.NewRouter()
	routes.JobServiceRoutes(router, jobController)

	if cfg.APP.Port == "" {
		cfg.APP.Port = "8080" // Default port
	}
	slog.Info("Starting server on port http://localhost:" + cfg.APP.Port)
	if err := http.ListenAndServe(":"+cfg.APP.Port, router); err != nil {
		slog.Error("HTTP server stopped", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
}

