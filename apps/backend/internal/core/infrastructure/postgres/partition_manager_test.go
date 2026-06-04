package postgres

import (
	"fmt"
	"testing"
	"time"
)

func TestManagePartitions_CreatesNextMonthPartition(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Simulate running on the last day of November 2025.
	// The manager should create the December 2025 partition.
	now := time.Date(2025, time.November, 30, 0, 0, 0, 0, time.UTC)
	managePartitions(db, 6, now)

	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = $1)",
		"audit_logs_y2025m12",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to query pg_tables: %v", err)
	}
	if !exists {
		t.Error("expected partition audit_logs_y2025m12 to be created, but it was not")
	}
}

func TestManagePartitions_CreatesCurrentMonthPartition(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Simulate running mid-month. The manager must ensure the current month's
	// partition exists, even if a previous run missed it.
	now := time.Date(2025, time.November, 15, 0, 0, 0, 0, time.UTC)
	managePartitions(db, 6, now)

	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = $1)",
		"audit_logs_y2025m11",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to query pg_tables: %v", err)
	}
	if !exists {
		t.Error("expected current-month partition audit_logs_y2025m11 to be created, but it was not")
	}
}

func TestManagePartitions_DropsExpiredPartition(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a partition that will be in retention scope.
	oldTable := "audit_logs_y2024m01"
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs
		FOR VALUES FROM ('2024-01-01') TO ('2024-02-01')
	`, oldTable))
	if err != nil {
		t.Fatalf("failed to create test partition: %v", err)
	}

	// Run with now = 2024-08-01 and retentionMonths = 6.
	// That means partitions before 2024-02 should be dropped.
	now := time.Date(2024, time.August, 1, 0, 0, 0, 0, time.UTC)
	managePartitions(db, 6, now)

	var exists bool
	err = db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = $1)",
		oldTable,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to query pg_tables: %v", err)
	}
	if exists {
		t.Errorf("expected expired partition %s to be dropped, but it still exists", oldTable)
	}
}

func TestManagePartitions_YearBoundary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Simulate running on December 31st: should create a January partition.
	now := time.Date(2025, time.December, 31, 23, 59, 59, 0, time.UTC)
	managePartitions(db, 6, now)

	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = $1)",
		"audit_logs_y2026m01",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to query pg_tables: %v", err)
	}
	if !exists {
		t.Error("expected partition audit_logs_y2026m01 to be created at year boundary, but it was not")
	}
}
