package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the service configuration
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	ServiceBus ServiceBusConfig
	NewRelic   NewRelicConfig
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

// InitConfig initializes the configuration using Viper
func InitConfig(cfgFile string) error {
	// Set defaults for configuration
	setDefaults()
	
	// Use config file from the flag if provided
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in common directories with name "config"
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/device-service")
		viper.SetConfigName("config")
	}
	
	// Set environment variable prefix for config overrides
	viper.SetEnvPrefix("DEVICE")
	
	// Enable automatic environment variable binding
	// For example, DEVICE_SERVER_PORT will override server.port
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	
	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, using defaults and environment variables
			fmt.Println("No config file found, using defaults and environment variables")
		} else {
			// Config file was found but another error occurred
			return fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
	
	return nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8091)
	viper.SetDefault("server.mode", "debug")
	
	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "device")
	viper.SetDefault("database.password", "device")
	viper.SetDefault("database.dbname", "device_service_db")
	viper.SetDefault("database.sslmode", "disable")
	
	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	
	// Service Bus defaults - no default connection string for security
	viper.SetDefault("servicebus.queuename", "device-events")
	
	// New Relic defaults
	viper.SetDefault("newrelic.appname", "Device Service Local")
	viper.SetDefault("newrelic.enabled", false)
}

// Load loads the configuration
func Load() (*Config, error) {
	// Server
	serverConfig := ServerConfig{
		Port: viper.GetInt("server.port"),
		Mode: viper.GetString("server.mode"),
	}
	
	// Database
	dbConfig := DatabaseConfig{
		Host:     viper.GetString("database.host"),
		Port:     viper.GetInt("database.port"),
		User:     viper.GetString("database.user"),
		Password: viper.GetString("database.password"),
		DBName:   viper.GetString("database.dbname"),
		SSLMode:  viper.GetString("database.sslmode"),
	}
	
	// Redis
	redisConfig := RedisConfig{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetInt("redis.port"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	}
	
	// Service Bus
	serviceBusConfig := ServiceBusConfig{
		ConnectionString: viper.GetString("servicebus.connectionstring"),
		QueueName:        viper.GetString("servicebus.queuename"),
	}
	
	// New Relic
	newRelicConfig := NewRelicConfig{
		AppName:    viper.GetString("newrelic.appname"),
		LicenseKey: viper.GetString("newrelic.licensekey"),
		Enabled:    viper.GetBool("newrelic.enabled"),
	}
	
	return &Config{
		Server:     serverConfig,
		Database:   dbConfig,
		Redis:      redisConfig,
		ServiceBus: serviceBusConfig,
		NewRelic:   newRelicConfig,
	}, nil
}
