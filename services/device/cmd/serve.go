package cmd

import (
	"context"
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
	"example.com/backstage/services/device/internal/telemetry"

	"github.com/spf13/cobra"
)

var (
	// Flags for the serve command
	disableMigration bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the device service API server",
	Long: `Start the device service HTTP API server that provides
device management, message processing, and firmware update capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	
	// Add flags specific to serve command
	serveCmd.Flags().BoolVar(&disableMigration, "disable-migration", false, "Disable automatic database migration on startup")
}



// Modify the existing runServer function to pass the repository to the server
func runServer() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Run database migrations if not disabled
	if !disableMigration {
		log.Info("Running database migrations...")
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("Failed to run database migrations: %v", err)
		}
		log.Info("Database migrations completed successfully")
	}

	// Initialize NewRelic
	nrApp, err := telemetry.InitNewRelic(cfg.NewRelic)
	if err != nil {
		log.Warnf("Failed to initialize New Relic: %v", err)
	}

	// Initialize Redis cache
	redisClient, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Warnf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize Azure Service Bus
	sbClient, err := messaging.NewServiceBusClient(cfg.ServiceBus, "device-sender")
	if err != nil {
		log.Fatalf("Failed to connect to Azure Service Bus: %v", err)
	}
	defer sbClient.Close()
	
	// Initialize repository
	repo := repository.NewRepository(db)
	
	// Initialize service
	svc := service.NewService(repo, redisClient, sbClient, log)

	// Initialize and start the server - pass repo to server
	server := api.NewServer(cfg, log, nrApp, svc, repo) // Pass repo here
	go func() {
		log.Infof("Starting server on port %d (mode: %s)", cfg.Server.Port, cfg.Server.Mode)
		if err := server.Start(); err != nil {
			log.Infof("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Infof("Received signal %s, shutting down server...", sig)

	// Shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// First, shut down the service to stop accepting new requests
	log.Info("Shutting down service...")
	if err := svc.Shutdown(); err != nil {
		log.Warnf("Service shutdown error: %v", err)
	}
	
	// Then shut down the HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server successfully shutdown")
}