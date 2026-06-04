package model

import "time"

// ============================================================
// Administrative Units — cây hành chính thống nhất
// ============================================================

// AdminUnitLevel cấp hành chính
type AdminUnitLevel string

const (
	AdminUnitLevelProvince AdminUnitLevel = "province"
	AdminUnitLevelDistrict AdminUnitLevel = "district"
	AdminUnitLevelWard     AdminUnitLevel = "ward"
)

// AdministrativeUnit đại diện một đơn vị hành chính trong cây
// province → district → ward
type AdministrativeUnit struct {
	Code       string         `db:"code"`
	Name       string         `db:"name"`
	Level      AdminUnitLevel `db:"level"`
	ParentCode *string        `db:"parent_code"` // NULL = cấp tỉnh
	CreatedAt  time.Time      `db:"created_at"`
}

// ============================================================
// User Assignments — phân công cán bộ vào đơn vị (có lịch sử)
// ============================================================

// UserAssignment gắn user với một đơn vị hành chính trong một khoảng thời gian.
// Thay thế province_code/district_code/ward_code cố định trên bảng users.
// Hỗ trợ: điều chuyển công tác, kiêm nhiệm, truy vết "ai phụ trách năm nào".
type UserAssignment struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	UnitCode  string     `db:"unit_code"`
	Role      string     `db:"role"`       // snapshot role lúc phân công
	StartDate time.Time  `db:"start_date"`
	EndDate   *time.Time `db:"end_date"`   // NULL = đang còn phụ trách
	Note      *string    `db:"note"`
	CreatedBy *string    `db:"created_by"`
	CreatedAt time.Time  `db:"created_at"`
}

// IsActive kiểm tra phân công còn hiệu lực không
func (a *UserAssignment) IsActive() bool {
	return a.EndDate == nil
}

// ============================================================
// Permissions + Role-Permissions — RBAC thực sự
// ============================================================

// Permission là một quyền cụ thể trong hệ thống
// vd: "citizens:read", "transfers:approve"
type Permission struct {
	ID          string    `db:"id"`
	Code        string    `db:"code"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

// RolePermission ánh xạ role → permission
type RolePermission struct {
	Role         string `db:"role"`
	PermissionID string `db:"permission_id"`
}

// ============================================================
// DTOs
// ============================================================

// AssignUserRequest yêu cầu phân công cán bộ vào đơn vị
type AssignUserRequest struct {
	UserID    string  `json:"user_id"    binding:"required"`
	UnitCode  string  `json:"unit_code"  binding:"required"`
	StartDate string  `json:"start_date" binding:"required"` // "2006-01-02"
	Note      *string `json:"note"`
}

// EndAssignmentRequest kết thúc phân công (điều chuyển / nghỉ việc)
type EndAssignmentRequest struct {
	EndDate string  `json:"end_date" binding:"required"` // "2006-01-02"
	Note    *string `json:"note"`
}

// UserAssignmentResponse DTO trả về
type UserAssignmentResponse struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Username  string     `json:"username,omitempty"`
	UnitCode  string     `json:"unit_code"`
	UnitName  string     `json:"unit_name,omitempty"`
	Role      string     `json:"role"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Note      *string    `json:"note,omitempty"`
	IsActive  bool       `json:"is_active"`
}

// GetActiveUnitCodes trả về các unit_code mà user đang phụ trách (active)
// Dùng trong middleware để thay thế province_code/district_code/ward_code cứng
func GetActiveUnitCodes(assignments []UserAssignment) []string {
	var codes []string
	for _, a := range assignments {
		if a.IsActive() {
			codes = append(codes, a.UnitCode)
		}
	}
	return codes
}
