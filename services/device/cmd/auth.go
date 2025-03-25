// cmd/apikey.go
package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	apiKeyName     string
	authLevel      int
	expirationDays int
)

// apiKeyCmd represents the apikey command
var apiKeyCmd = &cobra.Command{
	Use:   "apikey",
	Short: "Manage API keys",
	Long:  `Create, list, and delete API keys with different authorization levels.`,
}

// generateCmd represents the generate command
var generateKeyCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new API key",
	Long: `Generate a new API key with a specified authorization level:
  0: NoAuthLevel (Public access)
  1: ViewerAuthLevel (Read-only access)
  2: WriterAuthLevel (Read/write access)
  3: SudoAuthLevel (Administrative access)
  5: RegisteredDeviceAuthLevel (Device authentication)`,
	Run: func(cmd *cobra.Command, args []string) {
		generateAPIKey()
	},
}

// listKeysCmd represents the list command
var listKeysCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys",
	Long:  `List all API keys with their details`,
	Run: func(cmd *cobra.Command, args []string) {
		listAPIKeys()
	},
}

// deleteKeyCmd represents the delete command
var deleteKeyCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an API key",
	Long:  `Delete an API key by its ID`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid ID format: %v", err)
		}
		deleteAPIKey(uint(id))
	},
}

func init() {
	rootCmd.AddCommand(apiKeyCmd)
	apiKeyCmd.AddCommand(generateKeyCmd)
	apiKeyCmd.AddCommand(listKeysCmd)
	apiKeyCmd.AddCommand(deleteKeyCmd)

	// Flags for generate command
	generateKeyCmd.Flags().StringVarP(&apiKeyName, "name", "n", "", "Name for the API key (required)")
	generateKeyCmd.Flags().IntVarP(&authLevel, "level", "l", 1, "Authorization level (0-5)")
	generateKeyCmd.Flags().IntVarP(&expirationDays, "expiration", "e", 365, "Expiration in days (0 for never)")
	generateKeyCmd.MarkFlagRequired("name")
}

// generateSecureKey generates a secure random API key
func generateSecureKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateAPIKey creates a new API key with the specified parameters
func generateAPIKey() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	log.Info("Connecting to database...")
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := repository.NewRepository(db)

	// Validate auth level
	if authLevel != 0 && authLevel != 1 && authLevel != 2 && authLevel != 3 && authLevel != 5 {
		log.Fatalf("Invalid authorization level. Must be 0, 1, 2, 3, or 5.")
	}

	// Generate secure key
	key, err := generateSecureKey(32) // 32 bytes = 256 bits
	if err != nil {
		log.Fatalf("Failed to generate secure key: %v", err)
	}

	// Create API key object
	apiKey := &models.APIKey{
		Key:                key,
		Name:               apiKeyName,
		AuthorizationLevel: models.AuthorizationLevel(authLevel),
	}

	// Set expiration if provided
	if expirationDays > 0 {
		expiry := time.Now().AddDate(0, 0, expirationDays)
		apiKey.ExpiresAt = &expiry
	}

	// Save to database
	if err := repo.CreateAPIKey(context.Background(), apiKey); err != nil {
		log.Fatalf("Failed to save API key: %v", err)
	}

	// Display the new key
	fmt.Println("=================================================================")
	fmt.Println("API Key generated successfully!")
	fmt.Println("=================================================================")
	fmt.Printf("ID: %d\n", apiKey.ID)
	fmt.Printf("Name: %s\n", apiKey.Name)
	fmt.Printf("Authorization Level: %d\n", apiKey.AuthorizationLevel)
	if apiKey.ExpiresAt != nil {
		fmt.Printf("Expires: %s\n", apiKey.ExpiresAt.Format(time.RFC3339))
	} else {
		fmt.Println("Expires: Never")
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("API Key: %s\n", apiKey.Key)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("IMPORTANT: Store this key securely. It won't be displayed again.")
	fmt.Println("=================================================================")
}

// listAPIKeys lists all API keys
func listAPIKeys() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	log.Info("Connecting to database...")
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := repository.NewRepository(db)

	// Get all API keys
	apiKeys, err := repo.ListAPIKeys(context.Background())
	if err != nil {
		log.Fatalf("Failed to list API keys: %v", err)
	}

	// Display keys
	fmt.Println("=================================================================")
	fmt.Printf("Total API Keys: %d\n", len(apiKeys))
	fmt.Println("=================================================================")
	for _, key := range apiKeys {
		fmt.Printf("ID: %d\n", key.ID)
		fmt.Printf("Name: %s\n", key.Name)
		fmt.Printf("Authorization Level: %d\n", key.AuthorizationLevel)
		if key.ExpiresAt != nil {
			fmt.Printf("Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
		} else {
			fmt.Println("Expires: Never")
		}
		if key.LastUsedAt != nil {
			fmt.Printf("Last Used: %s\n", key.LastUsedAt.Format(time.RFC3339))
		} else {
			fmt.Println("Last Used: Never")
		}
		fmt.Println("-----------------------------------------------------------------")
	}
}

// deleteAPIKey deletes an API key by ID
func deleteAPIKey(id uint) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	log.Info("Connecting to database...")
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := repository.NewRepository(db)

	// Delete the API key
	if err := repo.DeleteAPIKey(context.Background(), id); err != nil {
		log.Fatalf("Failed to delete API key: %v", err)
	}

	fmt.Printf("API key with ID %d deleted successfully.\n", id)
}