package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ardoise/apps/backend/internal/core/domain"

	"github.com/lib/pq"
)

type InvitationRepository struct {
	db *sql.DB
}

func NewInvitationRepository(db *sql.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

func (r *InvitationRepository) Save(ctx context.Context, inv domain.Invitation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO group_invitations (id, group_id, inviter_id, invitee_id)
		VALUES ($1, $2, $3, $4)
	`, inv.ID, string(inv.GroupID), string(inv.InviterID), string(inv.InviteeID))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" &&
			strings.Contains(pqErr.Constraint, "group_invitee") {
			return domain.ErrAlreadyInvited
		}
		return fmt.Errorf("save invitation: %w", err)
	}
	return nil
}

func (r *InvitationRepository) GetByID(ctx context.Context, id string) (domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.QueryRowContext(ctx, `
		SELECT gi.id, gi.group_id, g.name, gi.inviter_id, gi.invitee_id, gi.created_at
		FROM group_invitations gi
		JOIN groups g ON g.id = gi.group_id
		WHERE gi.id = $1
	`, id).Scan(&inv.ID, &inv.GroupID, &inv.GroupName, &inv.InviterID, &inv.InviteeID, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Invitation{}, domain.ErrInvitationNotFound
		}
		return domain.Invitation{}, fmt.Errorf("get invitation: %w", err)
	}
	return inv, nil
}

func (r *InvitationRepository) GetPendingForUser(ctx context.Context, userID domain.UserID) ([]domain.Invitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT gi.id, gi.group_id, g.name, gi.inviter_id, gi.invitee_id, gi.created_at
		FROM group_invitations gi
		JOIN groups g ON g.id = gi.group_id
		WHERE gi.invitee_id = $1
		ORDER BY gi.created_at ASC
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invitations []domain.Invitation
	for rows.Next() {
		var inv domain.Invitation
		if err := rows.Scan(&inv.ID, &inv.GroupID, &inv.GroupName, &inv.InviterID, &inv.InviteeID, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		invitations = append(invitations, inv)
	}
	return invitations, rows.Err()
}

func (r *InvitationRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM group_invitations WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}
	return nil
}
