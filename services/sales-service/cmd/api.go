package cmd

import (
	"context"
	"os"
	"os/signal"
	"sales_service/config"
	"sales_service/internal/api"
	"sales_service/internal/cache"
	"sales_service/internal/models"
	"sales_service/internal/search"
	"sales_service/internal/services"
	"sales_service/internal/tracing"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the API server",
	Long:  `Start the HTTP API server to handle incoming sale payloads`,
	RunE:  runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)
}

func runAPI(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		return err
	}

	// Configure logging
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set up signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), 
		os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Initialize database connection
	db, err := initDatabase(cfg)
	if err != nil {
		return err
	}

	// Initialize cache
	redisCache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Redis cache, continuing without caching")
	}

	// Initialize tracer
	tracer, err := tracing.NewTracer(cfg.Tracing)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize tracer, continuing without tracing")
	}

	// Initialize Elasticsearch client
	elasticClient, err := search.NewElasticClient(cfg.Elastic)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Elasticsearch client, continuing without search functionality")
	}

	// Initialize services
	salesService := services.NewSalesService(db, redisCache, elasticClient, tracer)

	// Initialize and start the server
	server := api.NewServer(cfg, salesService, tracer)

	// Start the server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Error().Err(err).Msg("Server error")
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	
	// Shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	log.Info().Msg("Shutting down API server")
	return nil
}

func initDatabase(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the database
	if err := models.SetupModels(db); err != nil {
		return nil, err
	}

	return db, nil
}