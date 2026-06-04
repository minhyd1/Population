package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"population-service/internal/model"
)

// AdminUnitRepository truy cập bảng administrative_units và user_assignments
type AdminUnitRepository interface {
	// Administrative Units
	FindUnitByCode(ctx context.Context, code string) (*model.AdministrativeUnit, error)
	GetParentUnit(ctx context.Context, code string) (*model.AdministrativeUnit, error)
	GetChildUnits(ctx context.Context, parentCode string) ([]model.AdministrativeUnit, error)
	// GetAncestorCodes trả về tất cả code của cấp cha (dùng cho audit visibility)
	GetAncestorCodes(ctx context.Context, code string) ([]string, error)
	// GetDescendantCodes trả về tất cả code con cháu (dùng để filter dữ liệu theo scope)
	GetDescendantCodes(ctx context.Context, code string) ([]string, error)

	// User Assignments
	CreateAssignment(ctx context.Context, a *model.UserAssignment) error
	EndAssignment(ctx context.Context, id string, endDate time.Time, note *string) error
	GetActiveAssignments(ctx context.Context, userID string) ([]model.UserAssignment, error)
	GetAssignmentHistory(ctx context.Context, userID string) ([]model.UserAssignment, error)
	GetActiveOfficersByUnit(ctx context.Context, unitCode string) ([]model.UserAssignment, error)
	// GetOfficerAtTime: "ai phụ trách unit_code vào thời điểm t?" — dùng cho audit
	GetOfficerAtTime(ctx context.Context, unitCode string, at time.Time) ([]model.UserAssignment, error)

	// Permissions
	GetRolePermissions(ctx context.Context, role string) ([]string, error)
	HasPermission(ctx context.Context, role, permissionCode string) (bool, error)
}

type adminUnitRepository struct {
	db *sqlx.DB
}

func NewAdminUnitRepository(db *sqlx.DB) AdminUnitRepository {
	return &adminUnitRepository{db: db}
}

// ── Administrative Units ────────────────────────────────────

func (r *adminUnitRepository) FindUnitByCode(ctx context.Context, code string) (*model.AdministrativeUnit, error) {
	var u model.AdministrativeUnit
	err := r.db.GetContext(ctx, &u,
		`SELECT code, name, level, parent_code, created_at FROM administrative_units WHERE code = $1`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *adminUnitRepository) GetParentUnit(ctx context.Context, code string) (*model.AdministrativeUnit, error) {
	var u model.AdministrativeUnit
	err := r.db.GetContext(ctx, &u, `
		SELECT p.code, p.name, p.level, p.parent_code, p.created_at
		FROM administrative_units child
		JOIN administrative_units p ON child.parent_code = p.code
		WHERE child.code = $1
	`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *adminUnitRepository) GetChildUnits(ctx context.Context, parentCode string) ([]model.AdministrativeUnit, error) {
	var units []model.AdministrativeUnit
	err := r.db.SelectContext(ctx, &units,
		`SELECT code, name, level, parent_code, created_at
		 FROM administrative_units WHERE parent_code = $1 ORDER BY code`, parentCode)
	return units, err
}

// GetAncestorCodes đi từ code lên đến gốc (province).
// vd: ward "00001" → ["001", "01"]
func (r *adminUnitRepository) GetAncestorCodes(ctx context.Context, code string) ([]string, error) {
	rows, err := r.db.QueryxContext(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT code, parent_code FROM administrative_units WHERE code = $1
			UNION ALL
			SELECT a.code, a.parent_code
			FROM administrative_units a
			JOIN ancestors anc ON a.code = anc.parent_code
		)
		SELECT code FROM ancestors WHERE code != $1
	`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

// GetDescendantCodes lấy toàn bộ con cháu (dùng để scope filter).
// vd: district "001" → tất cả ward bên dưới
func (r *adminUnitRepository) GetDescendantCodes(ctx context.Context, code string) ([]string, error) {
	rows, err := r.db.QueryxContext(ctx, `
		WITH RECURSIVE descendants AS (
			SELECT code FROM administrative_units WHERE parent_code = $1
			UNION ALL
			SELECT a.code
			FROM administrative_units a
			JOIN descendants d ON a.parent_code = d.code
		)
		SELECT code FROM descendants
	`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

// ── User Assignments ────────────────────────────────────────

func (r *adminUnitRepository) CreateAssignment(ctx context.Context, a *model.UserAssignment) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO user_assignments (id, user_id, unit_code, role, start_date, end_date, note, created_by)
		VALUES (:id, :user_id, :unit_code, :role, :start_date, :end_date, :note, :created_by)
	`, a)
	return err
}

func (r *adminUnitRepository) EndAssignment(ctx context.Context, id string, endDate time.Time, note *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_assignments SET end_date=$1, note=COALESCE($2, note) WHERE id=$3 AND end_date IS NULL`,
		endDate, note, id)
	return err
}

func (r *adminUnitRepository) GetActiveAssignments(ctx context.Context, userID string) ([]model.UserAssignment, error) {
	var list []model.UserAssignment
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, unit_code, role, start_date, end_date, note, created_by, created_at
		FROM user_assignments
		WHERE user_id = $1 AND end_date IS NULL
		ORDER BY start_date DESC
	`, userID)
	return list, err
}

func (r *adminUnitRepository) GetAssignmentHistory(ctx context.Context, userID string) ([]model.UserAssignment, error) {
	var list []model.UserAssignment
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, unit_code, role, start_date, end_date, note, created_by, created_at
		FROM user_assignments
		WHERE user_id = $1
		ORDER BY start_date DESC
	`, userID)
	return list, err
}

func (r *adminUnitRepository) GetActiveOfficersByUnit(ctx context.Context, unitCode string) ([]model.UserAssignment, error) {
	var list []model.UserAssignment
	err := r.db.SelectContext(ctx, &list, `
		SELECT ua.id, ua.user_id, ua.unit_code, ua.role, ua.start_date, ua.end_date, ua.note, ua.created_by, ua.created_at
		FROM user_assignments ua
		WHERE ua.unit_code = $1 AND ua.end_date IS NULL
		ORDER BY ua.start_date DESC
	`, unitCode)
	return list, err
}

// GetOfficerAtTime tìm cán bộ phụ trách unit_code tại thời điểm `at`.
// Dùng khi xem lại lịch sử audit: "ai đã xử lý request này?"
func (r *adminUnitRepository) GetOfficerAtTime(ctx context.Context, unitCode string, at time.Time) ([]model.UserAssignment, error) {
	var list []model.UserAssignment
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, unit_code, role, start_date, end_date, note, created_by, created_at
		FROM user_assignments
		WHERE unit_code = $1
		  AND start_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
		ORDER BY start_date DESC
	`, unitCode, at)
	return list, err
}

// ── Permissions ─────────────────────────────────────────────

func (r *adminUnitRepository) GetRolePermissions(ctx context.Context, role string) ([]string, error) {
	var codes []string
	err := r.db.SelectContext(ctx, &codes, `
		SELECT p.code
		FROM role_permissions rp
		JOIN permissions p ON rp.permission_id = p.id
		WHERE rp.role = $1
	`, role)
	return codes, err
}

func (r *adminUnitRepository) HasPermission(ctx context.Context, role, permissionCode string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(1)
		FROM role_permissions rp
		JOIN permissions p ON rp.permission_id = p.id
		WHERE rp.role = $1 AND p.code = $2
	`, role, permissionCode)
	return count > 0, err
}

// ── Helpers ──────────────────────────────────────────────────

// BuildInClause tạo $1,$2,... cho IN clause (dùng trong các hàm scope filter)
func BuildInClause(codes []string) (string, []interface{}) {
	placeholders := make([]string, len(codes))
	args := make([]interface{}, len(codes))
	for i, c := range codes {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = c
	}
	return strings.Join(placeholders, ","), args
}
