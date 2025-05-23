package logging

import (
	"io"
	"os"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logging configuration.
type Config struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	Console    bool   `mapstructure:"console"`
	TimeFormat string `mapstructure:"time_format"`
}

// Setup initializes the global logger.
func Setup(cfg Config) {
	var writers []io.Writer

	if cfg.Console {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: cfg.TimeFormat})
	}

	if cfg.File != "" {
		file, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Error().Err(err).Msg("Failed to open log file")
		} else {
			writers = append(writers, file) // TODO: Add file rotation if needed
		}
	}

	if len(writers) == 0 {
		// Default to console if no writers configured
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: cfg.TimeFormat})
	}

	multi := zerolog.MultiLevelWriter(writers...)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		log.Warn().Str("configured_level", cfg.Level).Msg("Invalid log level, defaulting to info")
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(level)
	}

	log.Info().Str("level", zerolog.GlobalLevel().String()).Msg("Logger initialized")
}

// ContextualLogger creates a logger with context fields.
func ContextualLogger(ctx map[string]interface{}) zerolog.Logger {
	return log.With().Fields(ctx).Logger()
}