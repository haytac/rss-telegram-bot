package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haytac/rss-telegram-bot/internal/config"    // Module path
	"github.com/haytac/rss-telegram-bot/internal/database" // Module path
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestAppCfg creates a temporary AppConfig for testing CLI commands.
func setupTestAppCfg(t *testing.T) (*config.AppConfig, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "testcliconfig_*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "cli_test.db")
	// Ensure migrations are run for the CLI test DB if commands interact with schema
    // Reusing setupTestDB's logic for connecting and migrating is a good idea.
    // For simplicity here, we'll just set the path.
	
	// Initialize a dummy logger config
    logCfg := logging.Config{Level: "error", Console: true} // Quiet logger for tests


	cfg := &config.AppConfig{
		DatabasePath: dbPath,
		Log: logCfg,
		// Set other necessary fields if commands depend on them
	}
    
    // Initialize global AppCfg for CLI commands that use it
    AppCfg = cfg 

	cleanup := func() {
		os.RemoveAll(tempDir)
        AppCfg = nil // Reset global
	}
	return cfg, cleanup
}

// executeCommand captures the output of a Cobra command.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf) // Capture stderr as well
	root.SetArgs(args)
	
	// Need to simulate PersistentPreRunE if commands rely on it
    // Or ensure the test setup handles what PersistentPreRunE would do.
    // For tests, it's often cleaner to have the setup function (like setupTestAppCfg)
    // initialize everything that PersistentPreRunE would.

	err := root.ExecuteContext(context.Background()) // Use ExecuteContext for cancellable commands
	return strings.TrimSpace(buf.String()), err
}


func TestProxyAddCmd(t *testing.T) {
	cfg, cleanup := setupTestAppCfg(t)
	defer cleanup()

    // Initialize the database for this test run
    testDB, dbCleanup := database.setupTestDB(t) // Use the helper from database tests
    defer dbCleanup()
    cfg.DatabasePath = testDB.DBName() // Update AppCfg with the actual test DB path
    AppCfg = cfg // Ensure global AppCfg is updated with test DB path

	rootCmd := &cobra.Command{Use: "root"} // Dummy root
	proxyCmd := NewProxyCmd() // This will use the global AppCfg
	rootCmd.AddCommand(proxyCmd)

	// Test adding a proxy
	output, err := executeCommand(rootCmd, "proxy", "add", "my-cli-proxy", "http", "1.2.3.4:9090", "--username", "user", "--password", "pass")
	assert.NoError(t, err, "proxy add command failed")
	assert.Contains(t, output, "Proxy 'my-cli-proxy' added successfully with ID:")

	// Verify in DB
	proxyStore := database.NewProxyStore(testDB) // Use the already connected testDB
	proxies, errDb := proxyStore.ListProxies(context.Background())
	require.NoError(t, errDb)
	found := false
	for _, p := range proxies {
		if p.Name == "my-cli-proxy" {
			found = true
			assert.Equal(t, "http", p.Type)
			assert.Equal(t, "1.2.3.4:9090", p.Address)
			require.NotNil(t, p.Username)
			assert.Equal(t, "user", *p.Username)
			require.NotNil(t, p.Password)
			assert.Equal(t, "pass", *p.Password) // Passwords should not be compared directly if hashed
			break
		}
	}
	assert.True(t, found, "Proxy added via CLI not found in database")
}

// Add TestProxyListCmd, TestProxyValidateCmd etc.