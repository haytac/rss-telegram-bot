package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary SQLite DB for testing.
func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "testdb_*")
	require.NoError(t, err, "Failed to create temp dir for test DB")

	dbPath := filepath.Join(tempDir, "test.db")
	// Use a relative path to where your migrations are located from the test file
	// Adjust this path based on your project structure and where the test is run from.
	// If tests are run from project root, this might be "internal/database/migrations"
	migrationsPath := "migrations" // Assuming migrations are in a 'migrations' subdir relative to this test file
                                     // Or provide an absolute path or make it configurable.


	// Check if migrations directory exists relative to test execution
	// This is often tricky. A common way is to set an env var or find path relative to go.mod.
	// For now, let's assume it can be found or this test is run from a dir where 'migrations' is accessible.
	// If `go test ./internal/database/...` is run from project root:
	projectRootMigrationsPath := filepath.Join("..", "..", "internal", "database", "migrations") // Adjust based on test file location
    if _, statErr := os.Stat(projectRootMigrationsPath); statErr == nil {
        migrationsPath = projectRootMigrationsPath
    } else {
         t.Logf("Could not find migrations at %s, trying relative 'migrations/' path. Error: %v", projectRootMigrationsPath, statErr)
         // If the relative path 'migrations' also fails, Connect might error out or skip migrations.
    }


	db, err := Connect(dbPath, migrationsPath)
	require.NoError(t, err, "Failed to connect to test DB")

	cleanup := func() {
		errClose := db.Close()
		assert.NoError(t, errClose, "Failed to close test DB")
		errRemove := os.RemoveAll(tempDir) // Remove the entire temp directory
		assert.NoError(t, errRemove, "Failed to remove temp DB dir")
	}
	return db, cleanup
}

func TestProxyStore_GetProxyByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewProxyStore(db)
	ctx := context.Background()

	// 1. Test Get non-existent proxy
	proxy, err := store.GetProxyByID(ctx, 999)
	assert.NoError(t, err)
	assert.Nil(t, proxy, "Expected nil for non-existent proxy")

	// 2. Create a proxy and then get it
	newProxy := &Proxy{
		Name:    "test-proxy",
		Type:    "http",
		Address: "127.0.0.1:8080",
	}
	id, err := store.CreateProxy(ctx, newProxy)
	require.NoError(t, err)
	require.NotZero(t, id)

	retrievedProxy, err := store.GetProxyByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, retrievedProxy)
	assert.Equal(t, id, retrievedProxy.ID)
	assert.Equal(t, "test-proxy", retrievedProxy.Name)
	assert.Equal(t, "http", retrievedProxy.Type)
	assert.Equal(t, "127.0.0.1:8080", retrievedProxy.Address)
	assert.WithinDuration(t, time.Now(), retrievedProxy.CreatedAt, 5*time.Second) // Check timestamp
}

// Add more tests for CreateProxy (with unique name constraint), ListProxies, GetDefaultProxy etc.