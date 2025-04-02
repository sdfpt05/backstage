package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/backstage/services/device/api"
	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/cache"
	"example.com/backstage/services/device/internal/database"
	"example.com/backstage/services/device/internal/messaging"
	"example.com/backstage/services/device/internal/repository"
	"example.com/backstage/services/device/internal/service"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Serve command flags
	disableNewRelic   bool
	disableMetrics    bool
	serverPort        int
	gracefulTimeout   int
	enableProfiling   bool
	enableHealthCheck bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	Long: `Starts the device service API server that handles device registration,
firmware updates, and telemetry processing.

The server respects the configuration in config.yaml or specified via the --config flag.
It will gracefully shut down on receiving SIGINT or SIGTERM signals.`,
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Serve-specific flags
	serveCmd.Flags().BoolVar(&disableNewRelic, "disable-newrelic", false, "Disable New Relic monitoring")
	serveCmd.Flags().BoolVar(&disableMetrics, "disable-metrics", false, "Disable metrics collection")
	serveCmd.Flags().IntVar(&serverPort, "port", 0, "Server port (overrides config file)")
	serveCmd.Flags().IntVar(&gracefulTimeout, "graceful-timeout", 30, "Graceful shutdown timeout in seconds")
	serveCmd.Flags().BoolVar(&enableProfiling, "enable-profiling", false, "Enable pprof profiling endpoints")
	serveCmd.Flags().BoolVar(&enableHealthCheck, "enable-health-check", true, "Enable health check endpoints")
}

// startServer initializes and starts the API server
func startServer() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with command line flags if provided
	if serverPort > 0 {
		cfg.Server.Port = serverPort
	}

	log.WithFields(logrus.Fields{
		"port":             cfg.Server.Port,
		"metrics_enabled":  !disableMetrics,
		"newrelic_enabled": cfg.NewRelic.Enabled && !disableNewRelic,
	}).Info("Initializing service components...")

	// Initialize database with retry logic
	var db database.DB
	maxRetries := 5
	retryInterval := time.Second

	for i := 0; i < maxRetries; i++ {
		log.WithField("attempt", i+1).Info("Connecting to database...")
		db, err = database.Connect(cfg.Database)
		if err == nil {
			break
		}

		log.WithFields(logrus.Fields{
			"error":         err.Error(),
			"retry_attempt": i + 1,
			"max_retries":   maxRetries,
		}).Error("Failed to connect to database, retrying...")

		if i < maxRetries-1 {
			time.Sleep(retryInterval)
			// Exponential backoff
			retryInterval *= 2
		}
	}

	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
	}

	log.Info("Successfully connected to database")
	defer func() {
		log.Info("Closing database connection...")
		if err := db.Close(); err != nil {
			log.WithField("error", err.Error()).Error("Error closing database connection")
		}
	}()

	// Initialize Redis cache client
	log.Info("Connecting to Redis...")
	redisClient, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		log.Info("Closing Redis connection...")
		if err := redisClient.Close(); err != nil {
			log.WithField("error", err.Error()).Error("Error closing Redis connection")
		}
	}()

	// Initialize messaging client
	log.Info("Connecting to message broker...")
	msgClient, err := messaging.NewServiceBusClient(cfg.ServiceBus, "device-service")
	if err != nil {
		log.Fatalf("Failed to connect to message broker: %v", err)
	}
	defer func() {
		log.Info("Closing messaging connection...")
		if err := msgClient.Close(); err != nil {
			log.WithField("error", err.Error()).Error("Error closing messaging connection")
		}
	}()

	// Initialize New Relic if enabled
	var nrApp *newrelic.Application
	if cfg.NewRelic.Enabled && !disableNewRelic {
		log.Info("Initializing New Relic monitoring...")
		nrApp, err = newrelic.NewApplication(
			newrelic.ConfigAppName(cfg.NewRelic.AppName),
			newrelic.ConfigLicense(cfg.NewRelic.LicenseKey),
			newrelic.ConfigDistributedTracerEnabled(true),
		)
		if err != nil {
			log.Warnf("Failed to initialize New Relic: %v", err)
		} else {
			log.Info("New Relic monitoring initialized successfully")
		}
	}

	// Create repositories
	log.Info("Initializing repositories...")
	repo := repository.NewRepository(db)

	// Create service with configuration
	log.Info("Initializing service layer...")
	svc, err := service.NewService(service.ServiceConfig{
		Repository:      repo,
		Cache:           redisClient,
		MessagingClient: msgClient,
		Logger:          log,
		StoragePath:     cfg.Firmware.StoragePath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	// Create and initialize the server
	log.Info("Initializing API server...")
	server := api.NewServer(cfg, log, nrApp, svc, repo)

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.WithFields(logrus.Fields{
			"port": cfg.Server.Port,
		}).Info("Starting server...")

		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-stop
	log.Infof("Received signal %s, shutting down gracefully...", sig.String())

	// Create a timeout context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(gracefulTimeout)*time.Second)
	defer cancel()

	// Shutdown service components
	log.Info("Shutting down service components...")
	if err := svc.Shutdown(); err != nil {
		log.Warnf("Service shutdown error: %v", err)
	}

	// Shutdown HTTP server
	log.Info("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Warnf("Server shutdown error: %v", err)
	}

	log.Info("Server shutdown complete")
}
