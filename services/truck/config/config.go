package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the service
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	MessageBus MessageBusConfig
	Redis      RedisConfig
	Logging    LoggingConfig
}

// ServerConfig holds http server configuration
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	CorsWhiteList   []string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
	Debug    bool
	MaxConn  int
	MaxIdle  int
	MaxLife  time.Duration
}

// MessageBusConfig holds message bus configuration
type MessageBusConfig struct {
	ConnectionString string
	Prefix           string
	Queues           []string
	ERPQueue         string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	Enabled  bool
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level string
	JSON  bool
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Read from config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	
	// Override with environment variables
	v.SetEnvPrefix("OPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8000)
	v.SetDefault("server.readTimeout", "1m")
	v.SetDefault("server.writeTimeout", "1m")
	v.SetDefault("server.shutdownTimeout", "10s")
	v.SetDefault("server.corsWhiteList", []string{"*"})

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "root")
	v.SetDefault("database.password", "password")
	v.SetDefault("database.name", "operations_db")
	v.SetDefault("database.sslMode", "disable")
	v.SetDefault("database.debug", false)
	v.SetDefault("database.maxConn", 100)
	v.SetDefault("database.maxIdle", 10)
	v.SetDefault("database.maxLife", "5m")

	// MessageBus defaults
	v.SetDefault("messageBus.prefix", "dev")
	v.SetDefault("messageBus.erpQueue", "erp-messages-operations")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.enabled", false)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.json", false)
}