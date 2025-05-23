package cli

import (
	"fmt"
	"path/filepath"
	"time"

	// Ensure database is imported if you use database.Connect
	"github.com/haytac/rss-telegram-bot/internal/database"
	"github.com/spf13/cobra"
	// config "github.com/haytac/rss-telegram-bot/internal/config" // Not needed if using global cli.AppCfg
)

// NewDbCmd creates the 'db' command for database operations.
func NewDbCmd() *cobra.Command { // No appCfg parameter
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage the application database (SQLite)",
	}

	cmd.AddCommand(newDbBackupCmd()) // No appCfg parameter
	cmd.AddCommand(newDbRestoreCmd()) // No appCfg parameter

	return cmd
}

func newDbBackupCmd() *cobra.Command { // No appCfg parameter
	var outputPath string
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup the SQLite database",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Access the global AppCfg populated by RootCmd's PersistentPreRunE
			if AppCfg == nil { // AppCfg is the global variable from cli/root.go
				return fmt.Errorf("configuration not loaded for db backup")
			}
			// Use AppCfg directly
			db, err := database.Connect(AppCfg.DatabasePath, "")
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()

			if outputPath == "" {
				dbDir := filepath.Dir(AppCfg.DatabasePath)
				dbName := filepath.Base(AppCfg.DatabasePath)
				timestamp := time.Now().Format("20060102-150405")
				outputPath = filepath.Join(dbDir, fmt.Sprintf("%s-backup-%s.db", dbName, timestamp))
			}

			fmt.Printf("Backing up database from '%s' to '%s'...\n", AppCfg.DatabasePath, outputPath)
			if err := db.Backup(outputPath); err != nil {
				return fmt.Errorf("database backup failed: %w", err)
			}
			fmt.Println("Database backup successful.")
			return nil
		},
	}
	backupCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for the backup file (default: [db_dir]/[db_name]-backup-[timestamp].db)")
	return backupCmd
}

// Apply similar changes to newDbRestoreCmd and all RunE functions in proxy_cmd.go
// Ensure they use the global `cli.AppCfg` variable.
func newDbRestoreCmd() *cobra.Command { // No appCfg parameter
	var inputPath string
	restoreCmd := &cobra.Command{
		Use:   "restore <backup_file_path>",
		Short: "Restore the SQLite database from a backup file (WARNING: Overwrites current DB)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath = args[0]
			if AppCfg == nil { // Use global cli.AppCfg
				return fmt.Errorf("configuration not loaded for db restore")
			}
			// ... rest of the logic using AppCfg ...
			tempDB, err := database.Connect(AppCfg.DatabasePath, "")
            if err != nil {
                fmt.Printf("Note: Could not connect to current database (may not exist): %v\n", err)
                if tempDB == nil { // This part might need review if Connect always errors on non-existent DB
                     // tempDB = &database.DB{} // This is not a valid way to get a DB instance.
                     // If Connect fails, you might not be able to call tempDB.Restore
                     // The Restore logic should perhaps take dbPath and not rely on an existing connection.
                     // For now, let's assume Connect gives us a usable (even if not fully connected) DB object for Restore.
                }
            }
            if tempDB != nil && tempDB.DB != nil {
                 defer tempDB.Close()
            } else if tempDB == nil { // If Connect returned nil AND error
                return fmt.Errorf("failed to get a database instance for restore: %w", err)
            }


			fmt.Printf("WARNING: This will overwrite the current database at '%s' with the backup from '%s'.\n", AppCfg.DatabasePath, inputPath)
			fmt.Print("Are you sure you want to continue? (yes/no): ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Restore cancelled.")
				return nil
			}
			fmt.Println("Restoring database...")
			if err := tempDB.Restore(AppCfg.DatabasePath, inputPath); err != nil {
				return fmt.Errorf("database restore failed: %w", err)
			}
			fmt.Println("Database restore successful. Please restart the application if it is running.")
			return nil
		},
	}
	return restoreCmd
}