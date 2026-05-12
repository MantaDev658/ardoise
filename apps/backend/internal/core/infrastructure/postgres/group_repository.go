package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opensplit/apps/backend/internal/core/domain"

	"github.com/lib/pq"
)

type GroupRepository struct {
	db *sql.DB
}

func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func saveGroup(ctx context.Context, tx *sql.Tx, group *domain.Group) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO groups (id, name) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
	`, group.ID, group.Name)
	if err != nil {
		return fmt.Errorf("failed to save group: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM group_members WHERE group_id = $1", group.ID); err != nil {
		return fmt.Errorf("failed to clear old members: %w", err)
	}

	for _, member := range group.Members {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, group.ID, member,
		); err != nil {
			return fmt.Errorf("failed to save member %s: %w", member, err)
		}
	}
	return nil
}

func (r *GroupRepository) Save(ctx context.Context, group *domain.Group) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return saveGroup(ctx, tx, group)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := saveGroup(ctx, tx, group); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupRepository) GetByID(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
	var name string
	err := r.db.QueryRowContext(ctx, "SELECT name FROM groups WHERE id = $1", id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrGroupNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, "SELECT user_id FROM group_members WHERE group_id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}
	defer rows.Close()

	var members []domain.UserID
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		members = append(members, domain.UserID(uid))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &domain.Group{ID: id, Name: name, Members: members}, nil
}

// ListForUser returns all groups the user belongs to, including each group's full member list.
// Single query with array_agg replaces the previous N+1 pattern.
func (r *GroupRepository) ListForUser(ctx context.Context, userID domain.UserID) ([]*domain.Group, error) {
	query := `
		SELECT g.id, g.name,
		       COALESCE(array_agg(gm.user_id ORDER BY gm.user_id), '{}') AS members
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE g.id IN (SELECT group_id FROM group_members WHERE user_id = $1)
		GROUP BY g.id, g.name, g.created_at
		ORDER BY g.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, string(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var id, name string
		var memberIDs pq.StringArray
		if err := rows.Scan(&id, &name, &memberIDs); err != nil {
			return nil, fmt.Errorf("failed to scan group row: %w", err)
		}
		members := make([]domain.UserID, len(memberIDs))
		for i, m := range memberIDs {
			members[i] = domain.UserID(m)
		}
		groups = append(groups, &domain.Group{
			ID:      domain.GroupID(id),
			Name:    name,
			Members: members,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	return groups, nil
}

func (r *GroupRepository) UpdateName(ctx context.Context, id domain.GroupID, newName string) error {
	res, err := execer(ctx, r.db).ExecContext(ctx, "UPDATE groups SET name = $1 WHERE id = $2", newName, string(id))
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrGroupNotFound
	}
	return nil
}

func (r *GroupRepository) Delete(ctx context.Context, id domain.GroupID) error {
	res, err := execer(ctx, r.db).ExecContext(ctx, "DELETE FROM groups WHERE id = $1", string(id))
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrGroupNotFound
	}
	return nil
}

// LockGroup acquires a PostgreSQL row-level exclusive lock on the group for the duration of the
// calling transaction. Must be called inside a RunInTx block.
func (r *GroupRepository) LockGroup(ctx context.Context, groupID domain.GroupID) error {
	var id string
	err := execer(ctx, r.db).QueryRowContext(ctx, "SELECT id FROM groups WHERE id = $1 FOR UPDATE", string(groupID)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrGroupNotFound
	}
	return err
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID domain.GroupID, userID domain.UserID) error {
	res, err := execer(ctx, r.db).ExecContext(ctx, "DELETE FROM group_members WHERE group_id = $1 AND user_id = $2", string(groupID), string(userID))
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrUserNotInGroup
	}
	return nil
}
