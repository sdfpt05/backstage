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
	Firmware   FirmwareConfig    // New: Firmware configuration
	OTA        OTAConfig         // New: OTA configuration
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
	
	// Firmware defaults
	viper.SetDefault("firmware.storage_path", "./firmware")
	viper.SetDefault("firmware.keys_path", "./keys")
	viper.SetDefault("firmware.signing_algorithm", "secp256r1")
	viper.SetDefault("firmware.public_key_file", "ecdsa-public.pem")
	viper.SetDefault("firmware.private_key_file", "ecdsa-private.pem")
	viper.SetDefault("firmware.verify_signatures", true)
	
	// OTA defaults
	viper.SetDefault("ota.chunk_size", 8192)
	viper.SetDefault("ota.max_concurrent_updates", 100)
	viper.SetDefault("ota.download_timeout", 3600)  // 1 hour
	viper.SetDefault("ota.max_retries", 3)
	viper.SetDefault("ota.session_lifetime", 86400) // 24 hours
	viper.SetDefault("ota.delta_updates", false)
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
	
	// Firmware configuration
	firmwareConfig := FirmwareConfig{
		StoragePath:       viper.GetString("firmware.storage_path"),
		KeysPath:          viper.GetString("firmware.keys_path"),
		SigningAlgorithm:  viper.GetString("firmware.signing_algorithm"),
		PublicKeyFile:     viper.GetString("firmware.public_key_file"),
		PrivateKeyFile:    viper.GetString("firmware.private_key_file"),
		VerifySignatures:  viper.GetBool("firmware.verify_signatures"),
		RequireSignatures: viper.GetBool("firmware.require_signatures"),
	}
	
	// OTA configuration
	otaConfig := OTAConfig{
		ChunkSize:           viper.GetInt("ota.chunk_size"),
		MaxConcurrentUpdates: viper.GetInt("ota.max_concurrent_updates"),
		DownloadTimeout:     viper.GetInt("ota.download_timeout"),
		MaxRetries:          viper.GetInt("ota.max_retries"),
		SessionLifetime:     viper.GetInt("ota.session_lifetime"),
		DeltaUpdates:        viper.GetBool("ota.delta_updates"),
		DefaultUpdateType:   viper.GetString("ota.default_update_type"),
	}
	
	return &Config{
		Server:     serverConfig,
		Database:   dbConfig,
		Redis:      redisConfig,
		ServiceBus: serviceBusConfig,
		NewRelic:   newRelicConfig,
		Firmware:   firmwareConfig,
		OTA:        otaConfig,
	}, nil
}
