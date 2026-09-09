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
	configs, err := configs.LoadConfig()
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
		db, dbErr = infra.DBInit(configs.DB)
		return dbErr
	}); err != nil {
		slog.Error("Failed to initialize database connection after retries", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}

	// Initialize the job repository
	jobRepo := repositories.NewJobRepo(db, logger)

	// Initialize the jobservice
	jobService := services.NewJobService(jobRepo, logger)

	// Initialize the controllers
	jobController := controllers.NewJobServiceController(jobService, logger)

	router := routes.NewRouter()
	routes.JobServiceRoutes(router, jobController)

	if configs.APP.Port == "" {
		configs.APP.Port = "8080" // Default port
	}
	slog.Info("Starting server on port http://localhost:" + configs.APP.Port)
	if err := http.ListenAndServe(":"+configs.APP.Port, router); err != nil {
		slog.Error("HTTP server stopped", "error", err, "error_detail", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
}
