package cli

import (
	"fmt"
	"strings" // strings is used by strings.ToLower

	"github.com/haytac/rss-telegram-bot/internal/database" // Used by all RunE functions
	"github.com/haytac/rss-telegram-bot/internal/proxy"    // Used by newProxyValidateCmd
	// "github.com/haytac/rss-telegram-bot/pkg/interfaces" // Not directly used in this file's functions
	"github.com/spf13/cobra"
)

// NewProxyCmd creates the 'proxy' command and its subcommands.
// It no longer takes appCfg as a parameter.
func NewProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "proxy",
		Short:   "Manage proxy configurations",
		Aliases: []string{"proxies"},
	}

	// Subcommand constructors also no longer take appCfg.
	cmd.AddCommand(newProxyAddCmd())
	cmd.AddCommand(newProxyListCmd())
	cmd.AddCommand(newProxyValidateCmd())
	// Add update, remove commands

	return cmd
}

// newProxyAddCmd no longer takes appCfg.
func newProxyAddCmd() *cobra.Command {
	var (
		name               string
		pType              string
		address            string // host:port
		username           string
		password           string
		defaultForRSS      bool
		defaultForTelegram bool
	)

	addCmd := &cobra.Command{
		Use:   "add <name> <type> <address>",
		Short: "Add a new proxy (e.g., proxy add myproxy http 1.2.3.4:8080)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			pType = strings.ToLower(args[1]) // Uses "strings" package
			address = args[2]

			// Use the global cli.AppCfg
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded for proxy add")
			}
			// Connect to DB using path from global AppCfg
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			proxyStore := database.NewProxyStore(db)

			if pType != "http" && pType != "https" && pType != "socks5" {
				return fmt.Errorf("invalid proxy type: %s. Must be http, https, or socks5", pType)
			}

			p := &database.Proxy{
				Name:                 name,
				Type:                 pType,
				Address:              address,
				IsDefaultForRSS:      defaultForRSS,
				IsDefaultForTelegram: defaultForTelegram,
			}
			if cmd.Flags().Changed("username") {
				p.Username = &username
			}
			if cmd.Flags().Changed("password") {
				p.Password = &password
			}

			id, err := proxyStore.CreateProxy(cmd.Context(), p)
			if err != nil {
				return fmt.Errorf("failed to add proxy: %w", err)
			}
			fmt.Printf("Proxy '%s' added successfully with ID: %d\n", name, id)
			return nil
		},
	}

	addCmd.Flags().StringVarP(&username, "username", "u", "", "Proxy username")
	addCmd.Flags().StringVarP(&password, "password", "p", "", "Proxy password")
	addCmd.Flags().BoolVar(&defaultForRSS, "default-rss", false, "Set as default proxy for RSS feeds")
	addCmd.Flags().BoolVar(&defaultForTelegram, "default-telegram", false, "Set as default proxy for Telegram communication")

	return addCmd
}

// newProxyListCmd no longer takes appCfg.
func newProxyListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured proxies",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use the global cli.AppCfg
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded for proxy list")
			}
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			proxyStore := database.NewProxyStore(db)

			proxies, err := proxyStore.ListProxies(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list proxies: %w", err)
			}
			if len(proxies) == 0 {
				fmt.Println("No proxies configured.")
				return nil
			}
			fmt.Println("Configured Proxies:")
			for _, p := range proxies {
				auth := "no"
				if p.Username != nil && *p.Username != "" {
					auth = "yes"
				}
				rssDef := ""
				if p.IsDefaultForRSS {
					rssDef = "[Default RSS]"
				}
				tgDef := ""
				if p.IsDefaultForTelegram {
					tgDef = "[Default TG]"
				}

				fmt.Printf("ID: %d, Name: %s, Type: %s, Address: %s, Auth: %s %s %s\n",
					p.ID, p.Name, p.Type, p.Address, auth, rssDef, tgDef)
			}
			return nil
		},
	}
	return listCmd
}

// newProxyValidateCmd no longer takes appCfg.
func newProxyValidateCmd() *cobra.Command {
	var proxyID int64
	var targetURL string

	validateCmd := &cobra.Command{
		Use:   "validate <proxy_id>",
		Short: "Validate connectivity of a configured proxy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Sscan(args[0], &proxyID); err != nil {
				return fmt.Errorf("invalid proxy ID: %s", args[0])
			}

			// Use the global cli.AppCfg
			if AppCfg == nil {
				return fmt.Errorf("configuration not loaded for proxy validate")
			}
			db, err := database.Connect(AppCfg.DatabasePath, "internal/database/migrations")
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			proxyStore := database.NewProxyStore(db)

			p, err := proxyStore.GetProxyByID(cmd.Context(), proxyID)
			if err != nil {
				return fmt.Errorf("failed to get proxy %d: %w", proxyID, err)
			}
			if p == nil {
				return fmt.Errorf("proxy with ID %d not found", proxyID)
			}

			// proxy.NewHTTPClientFactory() does not take appCfg.
			// proxy.NewDefaultProxyValidator(clientFactory) also does not take appCfg.
			// They use the clientFactory.
			clientFactory := proxy.NewHTTPClientFactory()         // Uses proxy package
			validator := proxy.NewDefaultProxyValidator(clientFactory) // Uses proxy package

			fmt.Printf("Validating proxy %s (ID: %d, Address: %s) against target %s...\n", p.Name, p.ID, p.Address, targetURL)
			err = validator.Validate(cmd.Context(), p, targetURL)
			if err != nil {
				fmt.Printf("Validation failed: %v\n", err)
				return err
			}
			fmt.Println("Proxy validation successful.")
			return nil
		},
	}
	validateCmd.Flags().StringVar(&targetURL, "target-url", "https://www.google.com/generate_204", "URL to test proxy connectivity against")
	return validateCmd
}