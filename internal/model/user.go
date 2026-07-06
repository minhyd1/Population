package model

import (
	"time"

	jwtpkg "population-service/pkg/jwt"
)

// User ánh xạ với bảng users trong PostgreSQL.
//
// NOTE: ProvinceCode, DistrictCode, WardCode vẫn tồn tại trên bảng
// để backward-compatible với seed cũ và JWT claims hiện tại.
// Nhưng nguồn sự thật chính thức là bảng user_assignments.
// Khi cần "user đang phụ trách đâu", luôn query user_assignments WHERE end_date IS NULL.
type User struct {
	ID           string      `db:"id"`
	Username     string      `db:"username"`
	PasswordHash string      `db:"password_hash"`
	Role         jwtpkg.Role `db:"role"`
	// Deprecated: dùng user_assignments thay thế.
	// Giữ lại để JWT claims vẫn hoạt động với code cũ trong thời gian chuyển đổi.
	ProvinceCode *string `db:"province_code"`
	DistrictCode *string `db:"district_code"`
	WardCode     *string `db:"ward_code"`
	// Liên kết với hồ sơ dân cư — chỉ dùng cho role citizen_self
	CitizenID *string   `db:"citizen_id"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// RefreshToken lưu hash của refresh token để có thể revoke
type RefreshToken struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	TokenHash string     `db:"token_hash"` // SHA-256, không lưu plaintext
	ExpiresAt time.Time  `db:"expires_at"`
	CreatedAt time.Time  `db:"created_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}

// ──────────────────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────────────────

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest — chỉ super_admin mới được gọi endpoint register để tạo tài khoản mới.
// province_code/district_code/ward_code bắt buộc tùy theo role.
// Sau khi tạo user, nên gọi thêm POST /admin/assignments để tạo user_assignment chính thức.
type RegisterRequest struct {
	Username     string      `json:"username"      binding:"required,min=3,max=50"`
	Password     string      `json:"password"      binding:"required,min=6"`
	Role         jwtpkg.Role `json:"role"          binding:"required"`
	ProvinceCode *string     `json:"province_code"` // bắt buộc với province_manager
	DistrictCode *string     `json:"district_code"` // bắt buộc với district_manager
	WardCode     *string     `json:"ward_code"`     // bắt buộc với ward_officer
	CitizenID    *string     `json:"citizen_id"`    // bắt buộc với citizen_self
	// UnitCode nếu cung cấp sẽ tự động tạo user_assignment luôn
	UnitCode *string `json:"unit_code"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // giây
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// MeResponse trả về thông tin user hiện tại từ JWT claims
type MeResponse struct {
	UserID       string      `json:"user_id"`
	Username     string      `json:"username"`
	Role         jwtpkg.Role `json:"role"`
	ProvinceCode *string     `json:"province_code,omitempty"`
	DistrictCode *string     `json:"district_code,omitempty"`
	WardCode     *string     `json:"ward_code,omitempty"`
	CitizenID    *string     `json:"citizen_id,omitempty"`
	// ActiveUnits: danh sách đơn vị đang phụ trách (từ user_assignments)
	ActiveUnits []string `json:"active_units,omitempty"`
}

// UpdateUserRequest dùng cho super_admin quản lý tài khoản
type UpdateUserRequest struct {
	Role         *jwtpkg.Role `json:"role"`
	ProvinceCode *string      `json:"province_code"`
	DistrictCode *string      `json:"district_code"`
	WardCode     *string      `json:"ward_code"`
	IsActive     *bool        `json:"is_active"`
}

// ChangePasswordRequest dùng khi user tự đổi mật khẩu
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPasswordRequest dùng khi super_admin reset mật khẩu cho user khác
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}
