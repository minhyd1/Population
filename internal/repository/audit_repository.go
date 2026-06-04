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
	// Vấn đề 2: InsertWithVisibility ghi log kèm danh sách unit được xem
	InsertWithVisibility(ctx context.Context, log *model.AuditLog, unitCodes []string) error
	List(ctx context.Context, filter model.ListAuditLogFilter) ([]*model.AuditLog, int64, error)
}

type auditRepo struct {
	db *sqlx.DB
}

// NewAuditRepository tạo mới audit repository
func NewAuditRepository(db *sqlx.DB) AuditRepository {
	return &auditRepo{db: db}
}

// Insert ghi một bản ghi audit log mới vào DB (không có visibility control)
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

// InsertWithVisibility ghi audit log và tạo danh sách visibility trong một transaction.
// unitCodes là tập hợp các unit_code được phép xem log này.
// Vấn đề 2: Ward C không nên thấy log của transfer Ward A ↔ Ward B.
func (r *auditRepo) InsertWithVisibility(ctx context.Context, log *model.AuditLog, unitCodes []string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	insertLog := `
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
	if _, err := tx.NamedExecContext(ctx, insertLog, log); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert audit log: %w", err)
	}

	// Ghi visibility records
	for _, unitCode := range unitCodes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO audit_visibility (audit_id, unit_code) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			log.ID, unitCode,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert visibility unit=%s: %w", unitCode, err)
		}
	}

	return tx.Commit()
}

// List tra cứu audit log với filter, phân trang.
// Điểm 4 nâng cao: nếu filter.UnitCode != "" thì dùng cây administrative_units
// để bao gồm tất cả đơn vị con cháu (district_manager xem được ward bên dưới).
func (r *auditRepo) List(ctx context.Context, filter model.ListAuditLogFilter) ([]*model.AuditLog, int64, error) {
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	// Điểm 4: Audit Visibility scope với cây hành chính
	// - Nếu có UnitCode: lấy toàn bộ unit_code con cháu, JOIN audit_visibility
	// - Nếu không có UnitCode (super_admin / auditor quốc gia): xem tất cả
	baseTable := "audit_logs al"
	unitJoin := ""
	if filter.UnitCode != "" {
		// Dùng recursive CTE để lấy tất cả con cháu của unit này
		// → district_manager thấy log của tất cả ward bên dưới
		unitJoin = fmt.Sprintf(`
			INNER JOIN audit_visibility av ON av.audit_id = al.id
			INNER JOIN (
				WITH RECURSIVE scope AS (
					SELECT code FROM administrative_units WHERE code = $%d
					UNION ALL
					SELECT au.code FROM administrative_units au
					JOIN scope s ON au.parent_code = s.code
				)
				SELECT code FROM scope
			) scoped_units ON av.unit_code = scoped_units.code`,
			argIdx,
		)
		args = append(args, filter.UnitCode)
		argIdx++
	}

	if filter.CitizenID != "" {
		conditions = append(conditions, fmt.Sprintf("al.citizen_id = $%d", argIdx))
		args = append(args, filter.CitizenID)
		argIdx++
	}

	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("al.action = $%d", argIdx))
		args = append(args, string(filter.Action))
		argIdx++
	}

	if filter.ChangedBy != "" {
		conditions = append(conditions, fmt.Sprintf("al.changed_by = $%d", argIdx))
		args = append(args, filter.ChangedBy)
		argIdx++
	}

	if filter.From != "" {
		t, err := parseFlexibleTime(filter.From)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("al.changed_at >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	if filter.To != "" {
		t, err := parseFlexibleTime(filter.To)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf("al.changed_at <= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	// Đếm tổng
	countQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM %s%s WHERE %s",
		baseTable, unitJoin, where,
	)
	var total int64
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
		SELECT al.id, al.citizen_id, al.action,
		       al.changed_by, al.changed_by_name, al.changed_by_role,
		       al.old_values, al.new_values, al.changed_at
		FROM %s%s
		WHERE %s
		ORDER BY al.changed_at DESC
		LIMIT $%d OFFSET $%d
	`, baseTable, unitJoin, where, argIdx, argIdx+1)
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
