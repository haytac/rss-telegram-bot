package cli

import (
	"fmt"
	// "strconv" // <--- REMOVE THIS LINE if not used

	"github.com/haytac/rss-telegram-bot/internal/database"
	// "github.com/haytac/rss-telegram-bot/internal/config" // Not needed if using global AppCfg
	"github.com/spf13/cobra"
)

// NewFeedCmd creates the 'feed' command and its subcommands.
// No longer takes appCfg.
func NewFeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "feed",
		Short:   "Manage RSS feeds",
		Aliases: []string{"feeds"},
	}

	// Subcommand constructors no longer take appCfg.
	cmd.AddCommand(newFeedAddCmd())
	cmd.AddCommand(newFeedListCmd())
	// Add update, remove commands

	return cmd
}

// newFeedAddCmd no longer takes appCfg.
func newFeedAddCmd() *cobra.Command {
	var (
		// url string // This will come from args[0]
		userTitle           string
		freqSeconds         int
		botTokenID          int64
		chatID              string
		proxyID             int64
		formatProfileID     int64
		enabled             bool
	)

	addCmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a new RSS feed",
		Args:  cobra.ExactArgs(1), // Ensures <url> is provided
		RunE: func(cmd *cobra.Command, args []string) error {
			urlFromArg := args[0] // Get URL from arguments

			// Use the global cli.AppCfg
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded for feed add")
			}

			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			feedStore := database.NewFeedStore(db)

			// Use the global AppCfg for DefaultFetchFreq if freqSeconds flag is not set
			// Cobra handles default values for flags, so freqSeconds will have either the user's value or its default.
			// The default value for the freqSeconds flag should ideally use AppCfg.DefaultFetchFreq
			// if AppCfg is available at flag definition time.
			// However, since AppCfg is populated in PersistentPreRunE, flag defaults must be static
			// or explicitly checked against AppCfg here if not set by user.

			// For now, the flag definition in feed_cmd.go sets a static default.
			// If the --freq flag was *not* provided by the user, freqSeconds will be its default.
			// If you want the default to be from AppCfg dynamically:
			// currentFreq := freqSeconds
			// if !cmd.Flags().Changed("freq") { // If user didn't provide --freq
			//    currentFreq = AppCfg.DefaultFetchFreq // Use config default
			// }


			feed := &database.Feed{
				URL:              urlFromArg,
				FrequencySeconds: freqSeconds, // Will be the flag's value or its static default
				TelegramChatID:   chatID,
				IsEnabled:        enabled,
			}
			if cmd.Flags().Changed("title") {
				feed.UserTitle = &userTitle
			}
			if cmd.Flags().Changed("bot-token-id") {
				feed.TelegramBotID = &botTokenID
			}
			if cmd.Flags().Changed("proxy-id") {
				feed.ProxyID = &proxyID
			}
			if cmd.Flags().Changed("format-profile-id") {
				feed.FormattingProfileID = &formatProfileID
			}

			id, err := feedStore.CreateFeed(cmd.Context(), feed)
			if err != nil {
				return fmt.Errorf("failed to add feed: %w", err)
			}
			fmt.Printf("Feed added successfully with ID: %d\n", id)
			return nil
		},
	}

	// Flag definitions for addCmd
	addCmd.Flags().StringVarP(&userTitle, "title", "t", "", "Custom title for the feed")
	// The default for freqSeconds can be a static value here.
	// If AppCfg was guaranteed to be loaded before flag parsing, you could use AppCfg.DefaultFetchFreq.
	// Since it's not, a static default is safer for the flag itself.
	// The RunE logic can then override if the flag wasn't explicitly set by the user.
	addCmd.Flags().IntVarP(&freqSeconds, "freq", "f", 300, "Fetch frequency in seconds (default: 300 if AppCfg not loaded, otherwise uses AppCfg.DefaultFetchFreq if not specified)")
	addCmd.Flags().Int64Var(&botTokenID, "bot-token-id", 0, "ID of the Telegram Bot configuration to use")
	addCmd.Flags().StringVar(&chatID, "chat-id", "", "Telegram Chat ID (numeric) or @channelusername (required)")
	_ = addCmd.MarkFlagRequired("chat-id") // Error can be ignored for MarkFlagRequired in init
	addCmd.Flags().Int64Var(&proxyID, "proxy-id", 0, "ID of the Proxy configuration to use")
	addCmd.Flags().Int64Var(&formatProfileID, "format-profile-id", 0, "ID of the Formatting Profile to use")
	addCmd.Flags().BoolVar(&enabled, "enabled", true, "Enable the feed immediately")

	return addCmd
}

// newFeedListCmd no longer takes appCfg
func newFeedListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured RSS feeds",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use the global cli.AppCfg
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded for feed list")
			}
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("failed to list feeds: %w", err)
			}
			defer db.Close()
			feedStore := database.NewFeedStore(db)

			feeds, err := feedStore.GetEnabledFeeds(cmd.Context()) // Or a ListAllFeeds method
			if err != nil {
				return fmt.Errorf("failed to list feeds: %w", err)
			}

			if len(feeds) == 0 {
				fmt.Println("No feeds configured.")
				return nil
			}
			fmt.Println("Configured Feeds:")
			for _, f := range feeds {
				title := f.URL
				if f.UserTitle != nil && *f.UserTitle != "" {
					title = *f.UserTitle
				}
				status := "Disabled"
				if f.IsEnabled {
					status = "Enabled"
				}
				fmt.Printf("ID: %d, Title: %s, URL: %s, Freq: %ds, ChatID: %s, Status: %s\n",
					f.ID, title, f.URL, f.FrequencySeconds, f.TelegramChatID, status)
			}
			return nil
		},
	}
	return listCmd
}