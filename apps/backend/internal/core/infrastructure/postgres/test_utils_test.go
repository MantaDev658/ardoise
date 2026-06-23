package postgres

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set. Skipping integration test.")
	}
	if !strings.Contains(dbURL, "localhost") && !strings.Contains(dbURL, "127.0.0.1") {
		t.Fatalf("TEST_DB_URL must point to a local database (localhost/127.0.0.1); refusing to run tests against a remote host")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Clean all tables in reverse FK dependency order so no constraint blocks
	// the user delete (expenses.payer_id → users has no ON DELETE CASCADE).
	for _, table := range []string{
		"group_invitations",
		"splits",
		"expenses",
		"group_members",
		"groups",
		"users",
	} {
		var execErr error
		if _, execErr = db.Exec("DELETE FROM " + table); execErr != nil {
			t.Fatalf("failed to clean %s: %v", table, execErr)
		}
	}

	// Seed basic users needed for most tests
	_, err = db.Exec("INSERT INTO users (id, display_name) VALUES ('Alice', 'Alice'), ('Bob', 'Bob')")
	if err != nil {
		t.Fatalf("failed to seed users: %v", err)
	}

	return db
}
