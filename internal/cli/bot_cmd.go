package cli

import (
	"fmt"

	"github.com/haytac/rss-telegram-bot/internal/database" // Module path
	"github.com/spf13/cobra"
	"github.com/rs/zerolog/log"
)

func NewBotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bot",
		Short:   "Manage Telegram Bot configurations",
		Aliases: []string{"bots"},
	}
	cmd.AddCommand(newBotAddCmd())
	cmd.AddCommand(newBotListCmd())
	// Add update, remove commands
	return cmd
}

func newBotAddCmd() *cobra.Command {
	var description string
	addCmd := &cobra.Command{
		Use:   "add <raw_bot_token>",
		Short: "Add a new Telegram Bot (token will be 'encrypted')",
		Long:  "Adds a new Telegram Bot. The provided token will be 'encrypted' for storage (DEMO encryption only).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawToken := args[0]
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded")
			}
            // Ensure encryption key is initialized (it should be by NewApplication or similar)
            // If not, database.InitEncryptionKey would have logged warnings.
            // For CLI commands not running the full app, need to ensure this path.
            // Re-calling InitEncryptionKey here if AppCfg is available might be an option,
            // or ensure main.go handles it before any command.
            // For simplicity, assume it's handled or botStore methods log if key is missing.
            if AppCfg.EncryptionKey == "" {
                log.Warn().Msg("CLI: Encryption key not configured. Token will be stored INSECURELY if demo encryption falls back.")
            }
            // It's better if database.InitEncryptionKey is called once centrally.
            // We will rely on the one in app.NewApplication for `run` cmd, and for CLI,
            // it's a bit more complex if they don't run NewApplication.
            // Let's ensure main.go calls it.

			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("db connect: %w", err)
			}
			defer db.Close()
			botStore := database.NewTelegramBotStore(db)

			var descPtr *string
			if cmd.Flags().Changed("description") {
				descPtr = &description
			}

			id, err := botStore.CreateBot(cmd.Context(), rawToken, descPtr)
			if err != nil {
				return fmt.Errorf("failed to add bot: %w", err)
			}
			fmt.Printf("Telegram Bot added with ID: %d. Token hash stored.\n", id)
			fmt.Println("WARNING: The token 'encryption' is for DEMO PURPOSES ONLY and NOT secure for production.")
			return nil
		},
	}
	addCmd.Flags().StringVarP(&description, "description", "d", "", "Optional description for the bot")
	return addCmd
}

func newBotListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured Telegram Bots (metadata only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if AppCfg == nil { return fmt.Errorf("configuration not loaded") }
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil { return fmt.Errorf("db connect: %w", err) }
			defer db.Close()
			botStore := database.NewTelegramBotStore(db)

			bots, err := botStore.ListBots(cmd.Context())
			if err != nil { return fmt.Errorf("failed to list bots: %w", err) }

			if len(bots) == 0 {
				fmt.Println("No Telegram Bots configured.")
				return nil
			}
			fmt.Println("Configured Telegram Bots:")
			for _, b := range bots {
				desc := ""
				if b.Description != nil {
					desc = *b.Description
				}
				// Do NOT print b.EncryptedToken or b.TokenHash unless for debugging very carefully
				fmt.Printf("ID: %d, Description: '%s', Token Hash: ...%s (last 8), Created: %s\n",
					b.ID, desc, b.TokenHash[len(b.TokenHash)-8:], b.CreatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	return listCmd
}