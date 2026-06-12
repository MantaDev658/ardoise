package postgres

import (
	"context"
	"errors"
	"testing"

	"ardoise/apps/backend/internal/core/domain"

	"github.com/google/uuid"
)

func TestInvitationRepository_Save_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gRepo := NewGroupRepository(db)
	invRepo := NewInvitationRepository(db)
	ctx := context.Background()

	gID := domain.GroupID(uuid.NewString())
	g, _ := domain.NewGroup(gID, "Road Trip", "Alice")
	if err := gRepo.Save(ctx, g); err != nil {
		t.Fatalf("failed to save group: %v", err)
	}

	inv := domain.Invitation{
		ID:        uuid.NewString(),
		GroupID:   gID,
		InviterID: "Alice",
		InviteeID: "Bob",
	}
	if err := invRepo.Save(ctx, inv); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := invRepo.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != inv.ID {
		t.Errorf("ID: want %s, got %s", inv.ID, got.ID)
	}
	if got.GroupID != gID {
		t.Errorf("GroupID: want %s, got %s", gID, got.GroupID)
	}
	if got.GroupName != "Road Trip" {
		t.Errorf("GroupName: want 'Road Trip', got %s", got.GroupName)
	}
	if got.InviterID != "Alice" {
		t.Errorf("InviterID: want Alice, got %s", got.InviterID)
	}
	if got.InviteeID != "Bob" {
		t.Errorf("InviteeID: want Bob, got %s", got.InviteeID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set by the database")
	}
}

func TestInvitationRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	invRepo := NewInvitationRepository(db)

	_, err := invRepo.GetByID(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("expected ErrInvitationNotFound, got %v", err)
	}
}

func TestInvitationRepository_Save_DuplicateReturnsErrAlreadyInvited(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gRepo := NewGroupRepository(db)
	invRepo := NewInvitationRepository(db)
	ctx := context.Background()

	gID := domain.GroupID(uuid.NewString())
	g, _ := domain.NewGroup(gID, "Duplicate Test Group", "Alice")
	if err := gRepo.Save(ctx, g); err != nil {
		t.Fatalf("failed to save group: %v", err)
	}

	inv := domain.Invitation{
		ID:        uuid.NewString(),
		GroupID:   gID,
		InviterID: "Alice",
		InviteeID: "Bob",
	}
	if err := invRepo.Save(ctx, inv); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	inv2 := domain.Invitation{
		ID:        uuid.NewString(), // different invitation ID
		GroupID:   gID,
		InviterID: "Alice",
		InviteeID: "Bob", // same group + invitee
	}
	err := invRepo.Save(ctx, inv2)
	if !errors.Is(err, domain.ErrAlreadyInvited) {
		t.Errorf("expected ErrAlreadyInvited, got %v", err)
	}
}

func TestInvitationRepository_GetPendingForUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gRepo := NewGroupRepository(db)
	invRepo := NewInvitationRepository(db)
	ctx := context.Background()

	// Create two groups, invite Bob to both; invite Alice to one (should not appear in Bob's list).
	gID1 := domain.GroupID(uuid.NewString())
	gID2 := domain.GroupID(uuid.NewString())
	g1, _ := domain.NewGroup(gID1, "Group One", "Alice")
	g2, _ := domain.NewGroup(gID2, "Group Two", "Alice")
	_ = g2.AddMember("Bob") // Bob is a member of g2 already but we'll still invite to test isolation
	for _, g := range []*domain.Group{g1, g2} {
		if err := gRepo.Save(ctx, g); err != nil {
			t.Fatalf("failed to save group: %v", err)
		}
	}

	inv1 := domain.Invitation{ID: uuid.NewString(), GroupID: gID1, InviterID: "Alice", InviteeID: "Bob"}
	inv2 := domain.Invitation{ID: uuid.NewString(), GroupID: gID2, InviterID: "Alice", InviteeID: "Bob"}
	for _, inv := range []domain.Invitation{inv1, inv2} {
		if err := invRepo.Save(ctx, inv); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	pending, err := invRepo.GetPendingForUser(ctx, "Bob")
	if err != nil {
		t.Fatalf("GetPendingForUser: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending invitations for Bob, got %d", len(pending))
	}

	// GroupName should be populated via JOIN
	names := map[string]bool{pending[0].GroupName: true, pending[1].GroupName: true}
	if !names["Group One"] || !names["Group Two"] {
		t.Errorf("unexpected group names: %v", names)
	}
}

func TestInvitationRepository_GetPendingForUser_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	invRepo := NewInvitationRepository(db)

	pending, err := invRepo.GetPendingForUser(context.Background(), "Alice")
	if err != nil {
		t.Fatalf("GetPendingForUser: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending invitations, got %d", len(pending))
	}
}

func TestInvitationRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gRepo := NewGroupRepository(db)
	invRepo := NewInvitationRepository(db)
	ctx := context.Background()

	gID := domain.GroupID(uuid.NewString())
	g, _ := domain.NewGroup(gID, "Delete Test Group", "Alice")
	if err := gRepo.Save(ctx, g); err != nil {
		t.Fatalf("failed to save group: %v", err)
	}

	inv := domain.Invitation{
		ID:        uuid.NewString(),
		GroupID:   gID,
		InviterID: "Alice",
		InviteeID: "Bob",
	}
	if err := invRepo.Save(ctx, inv); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := invRepo.Delete(ctx, inv.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := invRepo.GetByID(ctx, inv.ID)
	if !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("expected ErrInvitationNotFound after delete, got %v", err)
	}
}
