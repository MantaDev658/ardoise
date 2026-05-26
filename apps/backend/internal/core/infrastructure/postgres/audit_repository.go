package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"ardoise/apps/backend/internal/core/domain"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Save(ctx context.Context, log domain.AuditLog) error {
	query := `INSERT INTO audit_logs (id, group_id, user_id, action, target_id, details)
	          VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := execer(ctx, r.db).ExecContext(ctx, query, log.ID, log.GroupID, log.UserID, log.Action, log.TargetID, log.Details)
	return err
}

func (r *AuditRepository) ListByGroup(ctx context.Context, groupID domain.GroupID, page domain.Page) ([]domain.AuditLog, error) {
	args := []any{string(groupID)}
	query := `SELECT id, group_id, user_id, action, target_id, details, created_at
	          FROM audit_logs WHERE group_id = $1`
	if !page.Cursor.IsZero() {
		args = append(args, page.Cursor)
		query += fmt.Sprintf(" AND created_at < $%d", len(args))
	}
	query += " ORDER BY created_at DESC"
	if page.Limit > 0 {
		args = append(args, page.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		var action string
		if err := rows.Scan(&l.ID, &l.GroupID, &l.UserID, &action, &l.TargetID, &l.Details, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Action = domain.AuditAction(action)
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	return logs, nil
}
