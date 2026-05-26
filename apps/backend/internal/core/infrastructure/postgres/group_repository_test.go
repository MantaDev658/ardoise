package postgres

import (
	"context"
	"testing"

	"ardoise/apps/backend/internal/core/domain"

	"github.com/google/uuid"
)

func TestGroupRepository_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewGroupRepository(db)
	ctx := context.Background()

	groupID := domain.GroupID(uuid.NewString())

	group, _ := domain.NewGroup(groupID, "Integration Test Group", "Alice")
	_ = group.AddMember("Bob")

	err := repo.Save(ctx, group)
	if err != nil {
		t.Fatalf("failed to save group: %v", err)
	}

	fetched, err := repo.GetByID(ctx, groupID)
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}
	if fetched.Name != "Integration Test Group" {
		t.Errorf("expected name 'Integration Test Group', got %s", fetched.Name)
	}
	if len(fetched.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(fetched.Members))
	}

	aliceGroups, err := repo.ListForUser(ctx, "Alice")
	if err != nil {
		t.Fatalf("failed to list groups for Alice: %v", err)
	}
	if len(aliceGroups) != 1 {
		t.Errorf("expected Alice to be in 1 group, got %d", len(aliceGroups))
	}

	err = repo.UpdateName(ctx, groupID, "Tahoe Ski Trip")
	if err != nil {
		t.Fatalf("failed to update group name: %v", err)
	}

	err = repo.RemoveMember(ctx, groupID, "Bob")
	if err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	err = repo.Delete(ctx, groupID)
	if err != nil {
		t.Fatalf("failed to delete group: %v", err)
	}

	_, err = repo.GetByID(ctx, groupID)
	if err == nil {
		t.Errorf("expected error getting deleted group, got nil")
	}
}
