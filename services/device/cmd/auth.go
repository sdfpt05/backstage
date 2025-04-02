package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/database"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"

	"github.com/spf13/cobra"
)

var (
	// Auth command flags
	keyName       string
	authLevel     int
	expiresDays   int
	listKeys      bool
	revokeKeyID   uint
	revokeKeyName string
)

// authCmd represents the auth commands for API key management
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage API keys and authentication",
	Long: `Manage API keys for authenticating with the device service API.
This command allows you to create, list, and revoke API keys.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, show help
		cmd.Help()
	},
}

// createKeyCmd represents the create-key command
var createKeyCmd = &cobra.Command{
	Use:   "create-key",
	Short: "Create a new API key",
	Long: `Create a new API key for authenticating with the device service API.
	
Authorization levels:
  1: Viewer - Can view but not modify resources
  2: Writer - Can create and modify resources
  3: Sudo - Full administrative access
  4: RegisteredDevice - Used by devices for authentication`,
	Run: func(cmd *cobra.Command, args []string) {
		createAPIKey()
	},
}

// listKeysCmd represents the list-keys command
var listKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List all API keys",
	Long:  `List all API keys currently registered in the system.`,
	Run: func(cmd *cobra.Command, args []string) {
		listAPIKeys()
	},
}

// revokeKeyCmd represents the revoke-key command
var revokeKeyCmd = &cobra.Command{
	Use:   "revoke-key",
	Short: "Revoke an API key",
	Long:  `Revoke an API key to prevent it from being used for authentication.`,
	Run: func(cmd *cobra.Command, args []string) {
		revokeAPIKey()
	},
}

func init() {
	rootCmd.AddCommand(authCmd)

	// Add subcommands
	authCmd.AddCommand(createKeyCmd)
	authCmd.AddCommand(listKeysCmd)
	authCmd.AddCommand(revokeKeyCmd)

	// Create key flags
	createKeyCmd.Flags().StringVar(&keyName, "name", "", "Name for the API key (required)")
	createKeyCmd.Flags().IntVar(&authLevel, "level", 1, "Authorization level: 1=Viewer, 2=Writer, 3=Sudo, 4=RegisteredDevice")
	createKeyCmd.Flags().IntVar(&expiresDays, "expires", 365, "Number of days until the key expires (0 for never)")
	createKeyCmd.MarkFlagRequired("name")

	// Revoke key flags
	revokeKeyCmd.Flags().UintVar(&revokeKeyID, "id", 0, "ID of the API key to revoke")
	revokeKeyCmd.Flags().StringVar(&revokeKeyName, "name", "", "Name of the API key to revoke")
}

// createAPIKey creates a new API key
func createAPIKey() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create repository
	repo := repository.NewRepository(db)

	// Validate auth level
	if authLevel < 1 || authLevel > 4 {
		log.Fatalf("Invalid authorization level. Must be 1-4")
	}

	// Convert to AuthorizationLevel enum
	var level models.AuthorizationLevel
	switch authLevel {
	case 1:
		level = models.ViewerAuthLevel
	case 2:
		level = models.WriterAuthLevel
	case 3:
		level = models.SudoAuthLevel
	case 4:
		level = models.RegisteredDeviceAuthLevel
	}

	// Generate a random API key (in production, use a more secure method)
	key := generateRandomKey()

	// Set expiration time if specified
	var expiresAt *time.Time
	if expiresDays > 0 {
		expires := time.Now().AddDate(0, 0, expiresDays)
		expiresAt = &expires
	}

	// Create the API key record
	apiKey := &models.APIKey{
		Key:                key,
		Name:               keyName,
		AuthorizationLevel: level,
		ExpiresAt:          expiresAt,
	}

	// Save to database
	if err := repo.CreateAPIKey(context.Background(), apiKey); err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	// Display the created key
	fmt.Printf("API key created successfully!\n")
	fmt.Printf("Name: %s\n", apiKey.Name)
	fmt.Printf("Key: %s\n", apiKey.Key)
	fmt.Printf("Authorization Level: %s (Level %d)\n", getAuthLevelName(level), authLevel)
	if expiresAt != nil {
		fmt.Printf("Expires: %s\n", expiresAt.Format("2006-01-02"))
	} else {
		fmt.Printf("Expires: Never\n")
	}
	fmt.Printf("\nIMPORTANT: Copy this key now. It will not be displayed again.\n")
}

// listAPIKeys lists all API keys
func listAPIKeys() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create repository
	repo := repository.NewRepository(db)

	// Get all API keys
	keys, err := repo.ListAPIKeys(context.Background())
	if err != nil {
		log.Fatalf("Failed to list API keys: %v", err)
	}

	// Display the keys
	fmt.Printf("API Keys:\n")
	fmt.Printf("%-5s | %-30s | %-15s | %-20s | %-20s\n", "ID", "Name", "Auth Level", "Expires", "Last Used")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	for _, key := range keys {
		// Format expiration and last used
		expires := "Never"
		if key.ExpiresAt != nil {
			expires = key.ExpiresAt.Format("2006-01-02")
		}

		lastUsed := "Never"
		if key.LastUsedAt != nil {
			lastUsed = key.LastUsedAt.Format("2006-01-02 15:04:05")
		}

		// Check if expired
		expired := ""
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			expired = " (EXPIRED)"
		}

		fmt.Printf("%-5d | %-30s | %-15s | %-20s | %-20s%s\n",
			key.ID,
			key.Name,
			getAuthLevelName(key.AuthorizationLevel),
			expires,
			lastUsed,
			expired,
		)
	}
}

// revokeAPIKey revokes an API key
func revokeAPIKey() {
	if revokeKeyID == 0 && revokeKeyName == "" {
		log.Fatalf("Either --id or --name must be specified")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create repository
	repo := repository.NewRepository(db)

	// If name is provided, find the key by name
	if revokeKeyName != "" {
		// List all keys and find by name
		keys, err := repo.ListAPIKeys(context.Background())
		if err != nil {
			log.Fatalf("Failed to list API keys: %v", err)
		}

		found := false
		for _, key := range keys {
			if key.Name == revokeKeyName {
				revokeKeyID = key.ID
				found = true
				break
			}
		}

		if !found {
			log.Fatalf("API key with name '%s' not found", revokeKeyName)
		}
	}

	// Revoke the key
	if err := repo.DeleteAPIKey(context.Background(), revokeKeyID); err != nil {
		log.Fatalf("Failed to revoke API key: %v", err)
	}

	fmt.Printf("API key revoked successfully (ID: %d)\n", revokeKeyID)
}

// Helper functions

// generateRandomKey generates a random API key
func generateRandomKey() string {
	// In a real implementation, use a cryptographically secure method
	// This is just a placeholder implementation
	now := time.Now().UnixNano()
	return fmt.Sprintf("ak_%d_%s", now, strconv.FormatInt(now, 36))
}

// getAuthLevelName returns a human-readable name for an authorization level
func getAuthLevelName(level models.AuthorizationLevel) string {
	switch level {
	case models.ViewerAuthLevel:
		return "Viewer"
	case models.WriterAuthLevel:
		return "Writer"
	case models.SudoAuthLevel:
		return "Sudo"
	case models.RegisteredDeviceAuthLevel:
		return "Device"
	default:
		return "Unknown"
	}
}
