package config

import (
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Environment       string `mapstructure:"ENVIRONMENT"`
	DB                DatabaseConfig
	Redis             RedisConfig
	Azure             AzureConfig
	Elastic           ElasticConfig
	Tracing           TracingConfig
	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	DSN         string `mapstructure:"DB_SOURCE"`
	ReadOnlyDSN string `mapstructure:"DB_SOURCE_READ_ONLY"`
	Name        string `mapstructure:"DB_NAME"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"REDIS_HOST"`
	Port     int    `mapstructure:"REDIS_PORT"`
	Password string `mapstructure:"REDIS_PASSWORD"`
	DB       int    `mapstructure:"REDIS_DB"`
	Enabled  bool   `mapstructure:"REDIS_ENABLED"`
}

// AzureConfig holds Azure Service Bus configuration
type AzureConfig struct {
	QueueConnStr string `mapstructure:"AZURE_QUEUE_CONN_STR"`
	QueueName    string `mapstructure:"AZURE_QUEUE_NAME"`
}

// ElasticConfig holds Elasticsearch configuration
type ElasticConfig struct {
	URL      string `mapstructure:"ELASTIC_SEARCH_URL"`
	Username string `mapstructure:"ELASTIC_SEARCH_USERNAME"`
	Password string `mapstructure:"ELASTIC_SEARCH_PASSWORD"`
	Prefix   string `mapstructure:"ELASTIC_SEARCH_PREFIX"`
	Index    string `mapstructure:"ELASTIC_SALES_ELASTIC_INDEX"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	LicenseKey       string `mapstructure:"NEW_RELIC_LICENSE_KEY"`
	AppName          string `mapstructure:"NEW_RELIC_APP_NAME"`
	LogLevel         string `mapstructure:"NEW_RELIC_LOG_LEVEL"`
	LogEnabled       bool   `mapstructure:"NEW_RELIC_LOG_ENABLED"`
	DistribTracing   bool   `mapstructure:"NEW_RELIC_DISTRIBUTED_TRACING_ENABLED"`
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	// Map direct fields
	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}

	// Database configuration
	config.DB = DatabaseConfig{
		DSN:         viper.GetString("DB_SOURCE"),
		ReadOnlyDSN: viper.GetString("DB_SOURCE_READ_ONLY"),
		Name:        viper.GetString("DB_NAME"),
	}

	// Redis configuration with sensible defaults
	config.Redis = RedisConfig{
		Host:     viper.GetString("REDIS_HOST"),
		Port:     viper.GetInt("REDIS_PORT"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
		Enabled:  viper.GetBool("REDIS_ENABLED"),
	}

	if config.Redis.Host == "" {
		config.Redis.Host = "localhost"
	}
	if config.Redis.Port == 0 {
		config.Redis.Port = 6379
	}

	// Azure configuration
	config.Azure = AzureConfig{
		QueueConnStr: viper.GetString("AZURE_QUEUE_CONN_STR"),
		QueueName:    viper.GetString("AZURE_QUEUE_NAME"),
	}

	// Elasticsearch configuration
	config.Elastic = ElasticConfig{
		URL:      viper.GetString("ELASTIC_SEARCH_URL"),
		Username: viper.GetString("ELASTIC_SEARCH_USERNAME"),
		Password: viper.GetString("ELASTIC_SEARCH_PASSWORD"),
		Prefix:   viper.GetString("ELASTIC_SEARCH_PREFIX"),
		Index:    viper.GetString("ELASTIC_SALES_ELASTIC_INDEX"),
	}

	// New Relic configuration
	config.Tracing = TracingConfig{
		LicenseKey:     viper.GetString("NEW_RELIC_LICENSE_KEY"),
		AppName:        viper.GetString("NEW_RELIC_APP_NAME"),
		LogLevel:       viper.GetString("NEW_RELIC_LOG_LEVEL"),
		LogEnabled:     viper.GetBool("NEW_RELIC_LOG_ENABLED"),
		DistribTracing: viper.GetBool("NEW_RELIC_DISTRIBUTED_TRACING_ENABLED"),
	}

	return
}

// FormatIndex formats an Elasticsearch index name with the configured prefix
func FormatIndex(cfg ElasticConfig, index string) string {
	return cfg.Prefix + "-" + index
}