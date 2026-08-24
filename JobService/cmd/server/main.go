package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

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

	// setup logger
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Load configuration
	configs, err := configs.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration:", "error", err)
		os.Exit(1)
	}
    

	// Initialize the database connection
	db, err := infra.DBInit(configs.DB)
	if err != nil {
		slog.Error("Failed to initialize database connection:", "error", err)
		os.Exit(1)
	}

	// Initialize the job repository
	jobRepo := repositories.NewJobRepo(db, logger)

	// Initialize the job service
	jobService := services.NewJobService(jobRepo, logger)

	// Initialize the job controller
	jobController := controllers.NewJobServiceController(jobService, logger)

	router := routes.NewRouter()
	routes.JobServiceRoutes(router, jobController)

	if configs.APP.Port == "" {
		configs.APP.Port = "8080" // Default port
	}
	slog.Info("Starting server on port http://localhost:" + configs.APP.Port)
	if err := http.ListenAndServe(":"+configs.APP.Port, router); err != nil {
		slog.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}
