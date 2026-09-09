package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	infra "github.com/gedyzed/JobFlow/JobService/infra"
	configs "github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/models"
	"github.com/gedyzed/JobFlow/JobService/repositories"
	"github.com/gedyzed/JobFlow/JobService/services"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found or could not be loaded")
	}

	logDir := "logs"
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr != nil {
		log.Fatalf("failed to create log directory: %v", mkErr) 
	}

	logPath := filepath.Join(logDir, "relay.txt")
	logFile, fileErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if fileErr != nil {
		log.Fatalf("failed to open log file %s: %v", logPath, fileErr)
	}
	defer logFile.Close()

	// Setup logger with configurable level (DEBUG when DEBUG=true or LOG_LEVEL=debug)
	logLevel := slog.LevelInfo
	if strings.EqualFold(os.Getenv("DEBUG"), "true") || strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		logLevel = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := configs.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	if cfg.RabbitMQ.Host == "" || cfg.RabbitMQ.Port == "" || cfg.RabbitMQ.User == "" || cfg.RabbitMQ.Password == "" {
		slog.Error("RabbitMQ configuration is incomplete. Please ensure RABBITMQ_HOST, RABBITMQ_PORT, RABBITMQ_USER, and RABBITMQ_PASSWORD are set.")
		os.Exit(1)
	}

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize the database connection with retries
	var db *gorm.DB
	if err := infra.Retry(ctx, "Database", 5, 5 * time.Second, func() (dbErr error) {
		db, dbErr = infra.DBInit(cfg.DB)
		return dbErr
	}); err != nil {
		slog.Error("Failed to initialize database connection after retries", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	// Initialize RabbitMQ service and client with retries
	rMqService := infra.NewRabbitMQService(cfg.RabbitMQ, logger)
	rmqClient := infra.NewRMQClient(rMqService)

	if err := infra.Retry(ctx, "RabbitMQ", 5, 5 * time.Second, func() error {
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

	// Create dedicated publisher for outbox relay
	queueName := cfg.RabbitMQ.Name
	if queueName == "" {
		queueName = "job_queue"
	}
	publisher, err := rmqClient.NewPublisher(ctx, models.PublisherTarget{
		Queue: queueName,
	})
	if err != nil {
		slog.Error("Failed to create RabbitMQ publisher", "error", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := publisher.Close(closeCtx); err != nil {
			slog.Error("Failed to cleanly close publisher", "error", err)
		}
	}()

	// Initialize relay repository and service
	relayRepo := repositories.NewRelayRepo(db, publisher, logger)
	relayService := services.NewRelayService(relayRepo, logger)

	outboxRelay := infra.NewOutboxRelay(2*time.Second, relayService, logger)

	slog.Info("Starting outbox relay process...")
	outboxRelay.Start(ctx)
	slog.Info("Outbox relay process stopped cleanly")
}
