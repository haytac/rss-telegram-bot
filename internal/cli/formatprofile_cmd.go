package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/haytac/rss-telegram-bot/internal/database" // Module path
	"github.com/spf13/cobra"
)

func NewFormatProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "formatprofile",
		Short:   "Manage Formatting Profiles",
		Aliases: []string{"fp", "format"},
	}
	cmd.AddCommand(newFormatProfileAddCmd())
	cmd.AddCommand(newFormatProfileListCmd())
	return cmd
}

func newFormatProfileAddCmd() *cobra.Command {
	var configFile string
	var ( // Direct flags for common config options
		titleTemplate         string
		messageTemplate       string
		hashtags              []string
		includeAuthor         bool
		omitGenericTitleRegex string
	)

	addCmd := &cobra.Command{
		Use:   "add <profile_name>",
		Short: "Add a new formatting profile",
		Long: `Add a new formatting profile. Configuration can be provided via a JSON file (--config-file)
or individual flags. Flags override file settings if both are provided for the same option.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			if AppCfg == nil { return fmt.Errorf("configuration not loaded") }

			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil { return fmt.Errorf("db connect: %w", err) }
			defer db.Close()
			profileStore := database.NewFormattingProfileStore(db)

			profile := &database.FormattingProfile{Name: profileName}
			// Default empty config
			profile.ParsedConfig = database.FormattingProfileConfig{}

			if configFile != "" {
				data, errFile := os.ReadFile(configFile)
				if errFile != nil {
					return fmt.Errorf("failed to read config file %s: %w", configFile, errFile)
				}
				if errJson := json.Unmarshal(data, &profile.ParsedConfig); errJson != nil {
					return fmt.Errorf("failed to parse JSON from config file %s: %w", configFile, errJson)
				}
				fmt.Printf("Loaded base configuration from %s\n", configFile)
			}

			// Override with flags if they were set
			if cmd.Flags().Changed("title-template") { profile.ParsedConfig.TitleTemplate = titleTemplate }
			if cmd.Flags().Changed("message-template") { profile.ParsedConfig.MessageTemplate = messageTemplate }
			if cmd.Flags().Changed("hashtags") { profile.ParsedConfig.Hashtags = hashtags }
			if cmd.Flags().Changed("include-author") { profile.ParsedConfig.IncludeAuthor = includeAuthor }
			if cmd.Flags().Changed("omit-generic-title-regex") { profile.ParsedConfig.OmitGenericTitleRegex = omitGenericTitleRegex }
			// Add other flags for UseTelegraphThresholdChars, etc.

			if errMarshal := profile.MarshalConfig(); errMarshal != nil { // To update ConfigJSON
				return fmt.Errorf("failed to marshal profile config to JSON: %w", errMarshal)
			}

			id, err := profileStore.CreateProfile(cmd.Context(), profile)
			if err != nil { return fmt.Errorf("failed to add formatting profile: %w", err) }
			fmt.Printf("Formatting Profile '%s' added with ID: %d\n", profileName, id)
			return nil
		},
	}
	addCmd.Flags().StringVarP(&configFile, "config-file", "c", "", "Path to a JSON file with formatting config")
	addCmd.Flags().StringVar(&titleTemplate, "title-template", "", "Go template for item title")
	addCmd.Flags().StringVar(&messageTemplate, "message-template", "", "Go template for item message body")
	addCmd.Flags().StringSliceVar(&hashtags, "hashtags", []string{}, "Comma-separated list of hashtags (e.g., tag1,tag2)")
	addCmd.Flags().BoolVar(&includeAuthor, "include-author", false, "Include author name in messages")
	addCmd.Flags().StringVar(&omitGenericTitleRegex, "omit-generic-title-regex", "", "Regex to detect and omit generic RSS item titles")
	// Add more flags as needed

	return addCmd
}

func newFormatProfileListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured formatting profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if AppCfg == nil { return fmt.Errorf("configuration not loaded") }
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil { return fmt.Errorf("db connect: %w", err) }
			defer db.Close()
			profileStore := database.NewFormattingProfileStore(db)

			profiles, err := profileStore.ListProfiles(cmd.Context())
			if err != nil { return fmt.Errorf("failed to list profiles: %w", err) }

			if len(profiles) == 0 {
				fmt.Println("No formatting profiles configured.")
				return nil
			}
			fmt.Println("Configured Formatting Profiles:")
			for _, p := range profiles {
				// Optionally, print a summary of the config
				configSummary, _ := json.MarshalIndent(p.ParsedConfig, "", "  ")
				fmt.Printf("ID: %d, Name: %s\nConfig:\n%s\n---\n", p.ID, p.Name, string(configSummary))
			}
			return nil
		},
	}
	return listCmd
}