package postgres

import (
	"context"
	"testing"

	"ardoise/apps/backend/internal/core/domain"

	"github.com/google/uuid"
)

// TestAuditRepository_SaveWithoutDedicatedPartition guards the production bug
// where a group expense / settle-up failed because audit_logs had no partition
// covering the insert's created_at. A DEFAULT partition must catch any date so
// the write (and the surrounding expense transaction) always succeeds.
func TestAuditRepository_SaveWithoutDedicatedPartition(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Drop every explicit monthly partition so only the DEFAULT remains. This
	// reproduces the scenario where the partition manager has not (yet) created
	// a partition for the current date.
	if _, err := db.Exec(`
		DO $$
		DECLARE p text;
		BEGIN
			FOR p IN SELECT tablename FROM pg_tables WHERE tablename ~ '^audit_logs_y[0-9]+m[0-9]+$'
			LOOP EXECUTE format('DROP TABLE %I', p); END LOOP;
		END $$;`); err != nil {
		t.Fatalf("failed to drop monthly partitions: %v", err)
	}

	repo := NewAuditRepository(db)
	err := repo.Save(context.Background(), domain.AuditLog{
		ID:       uuid.NewString(),
		GroupID:  "g1",
		UserID:   "Alice",
		Action:   domain.AuditActionCreatedExpense,
		TargetID: "x1",
		Details:  "no dedicated partition",
	})
	if err != nil {
		t.Fatalf("audit save must succeed via DEFAULT partition, got: %v", err)
	}
}
