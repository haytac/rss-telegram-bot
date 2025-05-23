package main

import (
	"github.com/haytac/rss-telegram-bot/internal/cli"     // Used to call cli.Execute()
	"github.com/haytac/rss-telegram-bot/internal/logging" // Used for initial basic logger setup with logging.Config
)

// The AppConfig struct definition is NOT needed here.
// Its primary definition is in internal/config/config.go
// and it's instantiated and managed by the cli package.

func main() {
	// Initialize a very basic logger as early as possible.
	// This is for Cobra's own execution path or any errors before RootCmd.PersistentPreRunE
	// fully configures the logger based on the loaded AppCfg.
	// logging.Config is a type from the 'logging' package.
	logging.Setup(logging.Config{Level: "info", Console: true, TimeFormat: "15:04:05"})

	// cli.AppCfg is populated, and full logging setup / DB initialization (like InitEncryptionKey)
	// is handled within cli.RootCmd.PersistentPreRunE.
	cli.Execute()
}