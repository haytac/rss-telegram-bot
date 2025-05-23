package cli

import (
	"fmt"
	"os"

	"github.com/haytac/rss-telegram-bot/internal/config"
	"github.com/haytac/rss-telegram-bot/internal/database" // For InitEncryptionKey (if called here)
	"github.com/haytac/rss-telegram-bot/internal/logging"  // <--- ADD THIS IMPORT
	"github.com/rs/zerolog/log"                            // <--- ADD THIS IMPORT for global logger
	"github.com/spf13/cobra"
	// "github.com/spf13/viper" // Not directly used in this snippet, but likely needed by config.LoadConfig
)

var (
	cfgFile string
	dryRun  bool
	AppCfg  *config.AppConfig // This global AppCfg is populated in PersistentPreRunE
)

var RootCmd = &cobra.Command{
	Use:   "rss-telegram-bot",
	Short: "A bot to fetch RSS feeds and send updates to Telegram.",
	Long:  `rss-telegram-bot is a configurable application that monitors RSS feeds...`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		loadedCfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		AppCfg = loadedCfg // Global AppCfg is set HERE

		logging.Setup(AppCfg.Log) // Now logging.Setup is defined
		AppCfg.DryRun = dryRun

		if AppCfg.EncryptionKey == "" {
			log.Warn().Msg("Configuration 'encryption_key' (or RSS_BOT_ENCRYPTION_KEY env var) is not set. Token storage will be INSECURE (DEMO MODE).") // Now log is defined
		}
		if errKey := database.InitEncryptionKey(AppCfg.EncryptionKey); errKey != nil { // database should be imported
			log.Warn().Err(errKey).Msg("Encryption key initialization issue. Tokens may not be handled securely.")
		}
		if AppCfg.DatabasePath == "" {
			return fmt.Errorf("database_path is not configured")
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// Error is usually printed by Cobra itself.
		// log.Error().Err(err).Msg("CLI execution failed") // If logger is available
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml, $HOME/.rss-telegram-bot/config.yaml)")
	RootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes or sending messages")
	
	// Subcommands will use the global AppCfg populated by PersistentPreRunE
	RootCmd.AddCommand(NewRunCmd())
	RootCmd.AddCommand(NewFeedCmd()) // These constructors won't take AppCfg
	RootCmd.AddCommand(NewProxyCmd())
	RootCmd.AddCommand(NewDbCmd())
	RootCmd.AddCommand(NewBotCmd())
	RootCmd.AddCommand(NewFormatProfileCmd())
	// RootCmd.AddCommand(NewOPMLCmd())
	// RootCmd.AddCommand(NewConfigCmd()) // For managing formatting profiles, telegram bots
}