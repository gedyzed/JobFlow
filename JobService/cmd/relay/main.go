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

	if cfg.RabbitMQ.Host == "" || cfg.RabbitMQ.Port == "" || cfg.RabbitMQ.User == "" || cfg.RabbitMQ.Password == "" || cfg.RabbitMQ.Name == "" {
		slog.Error("RabbitMQ configuration is incomplete. Please ensure RABBITMQ_HOST, RABBITMQ_PORT, RABBITMQ_USER, RABBITMQ_PASSWORD, and RABBITMQ_QUEUE_NAME are set.")
		os.Exit(1)
	}

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize the database connection with retries
	var db *gorm.DB
	const maxDBRetries = 15
	for attempt := 1; attempt <= maxDBRetries; attempt++ {
		select {
		case <-ctx.Done():
			slog.Info("Shutdown received while waiting for Database")
			return
		default:
		}

		db, err = infra.DBInit(cfg.DB)
		if err == nil {
			break
		}

		slog.Warn("Waiting for Database connection...", "attempt", attempt, "max_attempts", maxDBRetries, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		slog.Error("Failed to initialize database connection after retries", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	// Initialize RabbitMQ service and client with retries
	rMqService := infra.NewRabbitMQService(cfg.RabbitMQ, logger)
	rmqClient := infra.NewRMQClient(rMqService)

	const maxRMQRetries = 15
	var rmqErr error
	for attempt := 1; attempt <= maxRMQRetries; attempt++ {
		select {
		case <-ctx.Done():
			slog.Info("Shutdown received while waiting for RabbitMQ")
			return
		default:
		}

		rmqErr = rmqClient.Connect(ctx)
		if rmqErr == nil {
			break
		}

		slog.Warn("Waiting for RabbitMQ connection...", "attempt", attempt, "max_attempts", maxRMQRetries, "error", rmqErr)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}

	if rmqErr != nil {
		slog.Error("Failed to connect to RabbitMQ after retries", "error", rmqErr, "error_detail", fmt.Sprintf("%+v", rmqErr))
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rmqClient.Close(closeCtx); err != nil {
			slog.Error("Failed to cleanly close RabbitMQ client", "error", err)
		}
	}()

	// Initialize relay repository and service
	relayRepo := repositories.NewRelayRepo(db, rmqClient, logger)
	relayService := services.NewRelayService(relayRepo, logger)

	outboxRelay := infra.NewOutboxRelay(2*time.Second, relayService, logger)

	slog.Info("Starting outbox relay process...")
	outboxRelay.Start(ctx)
	slog.Info("Outbox relay process stopped cleanly")
}
