package config

import (
	"os"
	"strconv"
)

// Config holds the service configuration
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	ServiceBus    ServiceBusConfig
	NewRelic      NewRelicConfig
	Elasticsearch ElasticsearchConfig
}

// ServerConfig holds the HTTP server configuration
type ServerConfig struct {
	Port int
	Mode string // debug, release, test
}

// DatabaseConfig holds the database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds the Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// ServiceBusConfig holds the Azure Service Bus configuration
type ServiceBusConfig struct {
	ConnectionString string
	QueueName        string
}

// NewRelicConfig holds the New Relic configuration
type NewRelicConfig struct {
	AppName    string
	LicenseKey string
	Enabled    bool
}

// ElasticsearchConfig holds the Elasticsearch configuration
type ElasticsearchConfig struct {
	URLs     []string
	Username string
	Password string
	Index    string
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	// Server
	port, _ := strconv.Atoi(getEnv("PORT", "8095"))
	mode := getEnv("GIN_MODE", "debug")
	
	// Database
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	
	// Redis
	redisPort, _ := strconv.Atoi(getEnv("REDIS_PORT", "6379"))
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	
	// New Relic
	nrEnabled, _ := strconv.ParseBool(getEnv("NEW_RELIC_ENABLED", "true"))
	
	// Elasticsearch
	esURLs := []string{getEnv("ES_URL", "http://localhost:9200")}
	
	return &Config{
		Server: ServerConfig{
			Port: port,
			Mode: mode,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "truck_db"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     redisPort,
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		ServiceBus: ServiceBusConfig{
			ConnectionString: getEnv("SERVICEBUS_CONNECTION_STRING", ""),
			QueueName:        getEnv("SERVICEBUS_QUEUE_NAME", "truck-queue"),
		},
		NewRelic: NewRelicConfig{
			AppName:    getEnv("NEW_RELIC_APP_NAME", "Truck Aggregator"),
			LicenseKey: getEnv("NEW_RELIC_LICENSE_KEY", ""),
			Enabled:    nrEnabled,
		},
		Elasticsearch: ElasticsearchConfig{
			URLs:     esURLs,
			Username: getEnv("ES_USERNAME", ""),
			Password: getEnv("ES_PASSWORD", ""),
			Index:    getEnv("ES_INDEX", "truck-index"),
		},
	}, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
