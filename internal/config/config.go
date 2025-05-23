package config

import (
	"strings" // <--- ENSURE THIS IS PRESENT
	"time"

	"github.com/haytac/rss-telegram-bot/internal/logging" // Use your actual module path
	"github.com/spf13/viper"
)

// AppConfig holds the application configuration.
type AppConfig struct {
	DatabasePath                string         `mapstructure:"database_path"`
	Log                         logging.Config `mapstructure:"log"`
	MetricsPort                 string         `mapstructure:"metrics_port"`
	DefaultFetchFreq            int            `mapstructure:"default_fetch_frequency_seconds"` // in seconds
	EncryptionKey               string         `mapstructure:"encryption_key"`
	DryRun                      bool           // Not from config file, set by flag
}

// LoadConfig loads configuration from file and environment variables.
func LoadConfig(configPath string) (*AppConfig, error) {
	var cfg AppConfig

	// Set defaults
	viper.SetDefault("database_path", "./rss_bot.db")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.console", true)
	viper.SetDefault("log.time_format", time.RFC3339)
	viper.SetDefault("metrics_port", ":9090")
	viper.SetDefault("default_fetch_frequency_seconds", 300)
	viper.SetDefault("encryption_key", "")


	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, err
			}
		}
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.rss-telegram-bot") // Adjusted path
		viper.AddConfigPath("/etc/rss-telegram-bot/")  // Adjusted path
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, err
			}
		}
	}

	// Environment variables
	viper.SetEnvPrefix("RSS_BOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // <--- ENSURE THIS LINE IS PRESENT AND UNCOMMENTED
	viper.AutomaticEnv()

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}