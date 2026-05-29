package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"population-service/internal/model"

	"github.com/jmoiron/sqlx"
)

// AuditRepository định nghĩa interface ghi và tra cứu audit log
type AuditRepository interface {
	Insert(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, filter model.ListAuditLogFilter) ([]*model.AuditLog, int64, error)
}

type auditRepo struct {
	db *sqlx.DB
}

// NewAuditRepository tạo mới audit repository
func NewAuditRepository(db *sqlx.DB) AuditRepository {
	return &auditRepo{db: db}
}

// Insert ghi một bản ghi audit log mới vào DB
func (r *auditRepo) Insert(ctx context.Context, log *model.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, citizen_id, action,
			changed_by, changed_by_name, changed_by_role,
			old_values, new_values, changed_at
		) VALUES (
			:id, :citizen_id, :action,
			:changed_by, :changed_by_name, :changed_by_role,
			:old_values, :new_values, :changed_at
		)
	`
	_, err := r.db.NamedExecContext(ctx, query, log)
	return err
}

// List tra cứu audit log với filter, phân trang
func (r *auditRepo) List(ctx context.Context, filter model.ListAuditLogFilter) ([]*model.AuditLog, int64, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filter.CitizenID != "" {
		conditions = append(conditions, fmt.Sprintf("citizen_id = $%d", argIdx))
		args = append(args, filter.CitizenID)
		argIdx++
	}

	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, string(filter.Action))
		argIdx++
	}

	if filter.ChangedBy != "" {
		conditions = append(conditions, fmt.Sprintf("changed_by = $%d", argIdx))
		args = append(args, filter.ChangedBy)
		argIdx++
	}

	if filter.From != "" {
		t, err := parseFlexibleTime(filter.From)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("changed_at >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	if filter.To != "" {
		t, err := parseFlexibleTime(filter.To)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("changed_at <= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	where := strings.Join(conditions, " AND ")

	// Đếm tổng
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit count query: %w", err)
	}

	// Pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	query := fmt.Sprintf(`
		SELECT id, citizen_id, action,
		       changed_by, changed_by_name, changed_by_role,
		       old_values, new_values, changed_at
		FROM audit_logs
		WHERE %s
		ORDER BY changed_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	var logs []*model.AuditLog
	if err := r.db.SelectContext(ctx, &logs, query, args...); err != nil {
		return nil, 0, fmt.Errorf("audit list query: %w", err)
	}

	return logs, total, nil
}

// parseFlexibleTime nhận "2006-01-02" hoặc "2006-01-02T15:04:05Z"
func parseFlexibleTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", s)
}