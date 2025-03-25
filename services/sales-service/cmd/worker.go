package cmd

import (
	"context"
	"os"
	"os/signal"
	"sales_service/config"
	"sales_service/internal/cache"
	"sales_service/internal/messaging"
	"sales_service/internal/models"
	"sales_service/internal/search"
	"sales_service/internal/services"
	"sales_service/internal/tracing"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the background worker",
	Long:  `Start the background worker to process messages from Azure Service Bus and reconcile sales`,
	RunE:  runWorker,
}

func init() {
	rootCmd.AddCommand(workerCmd)
}

func runWorker(cmd *cobra.Command, args []string) error {
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

	// Create an error group to manage goroutines
	g, ctx := errgroup.WithContext(ctx)

	// Initialize database connection
	db, err := initDatabaseForWorker(cfg)
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

	// Initialize Azure Service Bus client
	azureBus, err := messaging.NewAzureServiceBus(cfg.Azure, tracer)
	if err != nil {
		return err
	}

	// Start the service bus processor
	g.Go(func() error {
		log.Info().Str("queue", cfg.Azure.QueueName).Msg("Starting Azure Service Bus processor")
		return azureBus.ProcessMessages(ctx, salesService.ProcessDispenseMessage)
	})

	// Start the sales reconciliation cron job
	g.Go(func() error {
		log.Info().Msg("Starting sales reconciliation cron job")
		
		// Create a scheduler
		scheduler, err := gocron.NewScheduler()
		if err != nil {
			return err
		}
		
		// Add the reconciliation job to run every minute
		_, err = scheduler.NewJob(
			gocron.DurationJob(1*time.Minute),
			gocron.NewTask(func() {
				if err := salesService.ReconcileSales(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to reconcile sales")
				}
			}),
		)
		if err != nil {
			return err
		}
		
		// Start the scheduler
		scheduler.Start()
		
		// Wait for context cancellation
		<-ctx.Done()
		
		// Shutdown the scheduler
		return scheduler.Shutdown()
	})

	// Wait for any goroutine to exit
	if err := g.Wait(); err != nil {
		log.Error().Err(err).Msg("Worker error")
		return err
	}

	log.Info().Msg("Worker shutting down gracefully")
	return nil
}

func initDatabaseForWorker(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the database
	if err := models.SetupModels(db); err != nil {
		return nil, err
	}

	// Get the underlying SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool parameters for long-running processes
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}