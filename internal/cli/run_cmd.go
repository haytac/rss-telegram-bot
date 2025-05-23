package cli

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log" // <--- ADD THIS IMPORT for zerolog global logger

	"github.com/haytac/rss-telegram-bot/internal/app" // For app.NewApplication
	// "github.com/haytac/rss-telegram-bot/internal/config" // Not directly needed if using global cli.AppCfg
	// "github.com/haytac/rss-telegram-bot/internal/database" // Only if calling other database functions directly
	"github.com/spf13/cobra"
)

// NewRunCmd creates the run command.
// It no longer takes appCfg as a parameter.
func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Starts the RSS feed fetching and Telegram notification service",
		Long:  `This command starts the main service that continuously monitors RSS feeds based on the configured schedules and sends updates to Telegram.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use the global cli.AppCfg populated by RootCmd's PersistentPreRunE
			if AppCfg == nil {
				// Use the imported log package
				log.Error().Msg("Configuration (AppCfg) not loaded in 'run' command. PersistentPreRunE might not have run or failed.")
				return fmt.Errorf("critical: AppCfg not loaded")
			}

			// database.InitEncryptionKey() is now handled in root.go's PersistentPreRunE,
			// so it's not called here.

			// Pass the global AppCfg to NewApplication
			application, err := app.NewApplication(AppCfg)
			if err != nil {
				// Use the imported log package
				log.Error().Err(err).Msg("Failed to initialize application")
				return fmt.Errorf("failed to initialize application: %w", err)
			}

			ctx, cancel := context.WithCancel(cmd.Context()) // Use cmd.Context() for signals
			defer cancel()

			// The application.Run method will handle its own logging.
			return application.Run(ctx)
		},
	}
	return cmd
}