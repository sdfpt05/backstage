package cmd

import (
	"context"
	"fmt"
	"time"

	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/database"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	dryRun      bool
	autoApprove bool
	timeout     int
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Runs database migrations to ensure the database schema
is up-to-date. This is useful for CI/CD pipelines or initial setup.

Examples:
  # Run migrations normally
  device-service migrate
  
  # Run in dry-run mode (shows what would be done)
  device-service migrate --dry-run
  
  # Skip confirmation prompt
  device-service migrate --auto-approve`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		// Skip confirmation in dry-run mode
		if !autoApprove && !dryRun {
			fmt.Print("Are you sure you want to run database migrations? This operation can be destructive. [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				log.Info("Migration cancelled by user")
				return
			}
		}

		runMigration(ctx)
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	// Add migration-specific flags
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show migration plan without executing")
	migrateCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip confirmation prompt")
	migrateCmd.Flags().IntVar(&timeout, "timeout", 60, "Timeout in seconds for migration operations")
}

// runMigration executes the database migrations
func runMigration(ctx context.Context) {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	log.WithField("connection_hash", hashConnectionString(fmt.Sprintf("%s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName))).
		Info("Connecting to database...")

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error_details": err.Error(),
			"db_host":       cfg.Database.Host,
			"db_name":       cfg.Database.DBName,
		}).Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.WithField("error", err.Error()).
				Error("Error closing database connection")
		}
	}()

	if dryRun {
		log.Info("DRY RUN MODE: Would run migrations on these tables:")
		log.Info("- Organization")
		log.Info("- FirmwareRelease")
		log.Info("- Device")
		log.Info("- DeviceMessage")
		log.Info("- APIKey")
		log.Info("No changes were made.")
		return
	}

	// Run database migrations
	log.Info("Running database migrations...")

	if err := database.AutoMigrate(db); err != nil {
		log.WithField("error_details", err.Error()).
			Fatalf("Failed to run database migrations: %v", err)
	}

	duration := time.Since(startTime).Round(time.Millisecond)
	log.WithField("duration_ms", duration.Milliseconds()).
		Info("Database migrations completed successfully")
}

// hashConnectionString returns a masked hash of the connection string for logging
// This prevents exposing passwords in logs while still helping with debugging
func hashConnectionString(connString string) string {
	// This is just a placeholder - in a real system, implement a secure hash
	if len(connString) < 8 {
		return "invalid_connection_string"
	}
	return fmt.Sprintf("...%s", connString[len(connString)-6:])
}
