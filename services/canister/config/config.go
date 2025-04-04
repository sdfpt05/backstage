package config

import (
	"fmt"

	"github.com/spf13/viper"
)

var configFile string

type Config struct {
	// Database
	DBDriver  string `mapstructure:"DB_DRIVER"`
	DBSource  string `mapstructure:"DB_SOURCE"`
	
	// HTTP Server
	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`
	
	// Elasticsearch
	ElasticSearchURL      string `mapstructure:"ELASTIC_SEARCH_URL"`
	ElasticSearchUsername string `mapstructure:"ELASTIC_SEARCH_USERNAME"`
	ElasticSearchPassword string `mapstructure:"ELASTIC_SEARCH_PASSWORD"`
	ElasticSearchPrefix   string `mapstructure:"ELASTIC_SEARCH_PREFIX"`
	
	// Azure Service Bus
	AzureQueueConnStr                   string `mapstructure:"AZURE_QUEUE_CONN_STR"`
	AzureMessagesConfigurationQueueName string `mapstructure:"AZURE_MESSAGES_CONF_QUEUE_NAME"`
	AzureMessagesEventsQueueName        string `mapstructure:"AZURE_MESSAGES_EVENTS_QUEUE_NAME"`
	
	// IAM
	IAMServerAddress string `mapstructure:"IAM_SERVER_ADDRESS"`
	
	// Other configuration
	SnapshotFrequency int  `mapstructure:"SNAPSHOT_FREQUENCY"`
	EnableMigrations  bool `mapstructure:"ENABLE_MIGRATIONS"`
}

func SetConfigFile(file string) {
	configFile = file
}

func LoadConfig() (Config, error) {
	var config Config

	viper.SetConfigType("env")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("app")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// Try staging config if default fails
		viper.SetConfigName("staging.app")
		if err := viper.ReadInConfig(); err != nil {
			return config, fmt.Errorf("error loading configuration: %w", err)
		}
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("error unmarshaling configuration: %w", err)
	}

	return config, nil
}

// FormatIndex adds the configured prefix to an index name
func FormatIndex(index string) string {
	cfg, err := LoadConfig()
	if err != nil {
		return index
	}
	return cfg.ElasticSearchPrefix + "-" + index
}