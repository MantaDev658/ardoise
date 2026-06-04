package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// StartPartitionManager runs a daily background job to manage audit log partition lifecycle.
func StartPartitionManager(ctx context.Context, db *sql.DB, retentionMonths int) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("partition manager panic: %v", r)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				managePartitions(db, retentionMonths, time.Now().UTC())
			}
		}
	}()
	managePartitions(db, retentionMonths, time.Now().UTC())
}

func managePartitions(db *sql.DB, retentionMonths int, now time.Time) {
	// Ensure partitions exist for the current month and next month.
	// Creating both guards against the manager having been offline for a period.
	for _, offset := range []int{0, 1} {
		target := now.AddDate(0, offset, 0)
		start := time.Date(target.Year(), target.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		tableName := fmt.Sprintf("audit_logs_y%dm%02d", start.Year(), start.Month())

		// #nosec G201 -- DDL statements cannot use parameters; inputs are strictly controlled internal dates
		createSQL := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs
			FOR VALUES FROM ('%s') TO ('%s');
		`, tableName, start.Format("2006-01-02"), end.Format("2006-01-02"))

		if _, err := db.Exec(createSQL); err != nil {
			log.Printf("Failed to create audit partition %s: %v", tableName, err)
		}
	}

	// Drop all partitions whose month falls before the retention cutoff.
	cutoff := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -retentionMonths, 0)
	rows, err := db.Query(`SELECT tablename FROM pg_tables WHERE tablename ~ '^audit_logs_y[0-9]+m[0-9]+$'`)
	if err != nil {
		log.Printf("Failed to list audit partitions: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err != nil {
			continue
		}
		var year, month int
		if _, err := fmt.Sscanf(tbl, "audit_logs_y%dm%d", &year, &month); err != nil {
			continue
		}
		partStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		if partStart.Before(cutoff) {
			// #nosec G201 -- tbl validated above via Sscanf pattern; no user input
			if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", tbl)); err != nil {
				log.Printf("Failed to drop expired partition %s: %v", tbl, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating audit partitions: %v", err)
	}
}
