package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role định nghĩa vai trò người dùng — 9 role theo đặc tả
type Role string

const (
	RoleSuperAdmin       Role = "super_admin"
	RoleNationalManager  Role = "national_manager"
	RoleProvinceManager  Role = "province_manager"
	RoleDistrictManager  Role = "district_manager"
	RoleWardOfficer      Role = "ward_officer"
	RoleDataEntry        Role = "data_entry"
	RoleAuditor          Role = "auditor"
	RoleAnalyticsViewer  Role = "analytics_viewer"
	RoleCitizenSelf      Role = "citizen_self"
)

// AllRoles là danh sách toàn bộ role hợp lệ — dùng để validate trong register
var AllRoles = []Role{
	RoleSuperAdmin, RoleNationalManager, RoleProvinceManager,
	RoleDistrictManager, RoleWardOfficer, RoleDataEntry,
	RoleAuditor, RoleAnalyticsViewer, RoleCitizenSelf,
}

// IsValid kiểm tra role có hợp lệ không
func (r Role) IsValid() bool {
	for _, v := range AllRoles {
		if r == v {
			return true
		}
	}
	return false
}

// Claims là payload bên trong JWT token.
// Ngoài role, lưu thêm province/district/ward code để middleware
// lọc dữ liệu theo địa bàn mà không cần query DB mỗi request.
type Claims struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         Role   `json:"role"`
	ProvinceCode string `json:"province_code,omitempty"` // dùng cho province_manager
	DistrictCode string `json:"district_code,omitempty"` // dùng cho district_manager
	WardCode     string `json:"ward_code,omitempty"`     // dùng cho ward_officer
	jwt.RegisteredClaims
}

// Manager quản lý việc tạo và xác thực JWT
type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func New(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (m *Manager) GenerateAccessToken(userID, username string, role Role, provinceCode, districtCode, wardCode string) (string, error) {
	claims := &Claims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		ProvinceCode: provinceCode,
		DistrictCode: districtCode,
		WardCode:     wardCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "population-service",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.accessSecret)
}

func (m *Manager) GenerateRefreshToken(userID, username string, role Role, provinceCode, districtCode, wardCode string) (string, error) {
	claims := &Claims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		ProvinceCode: provinceCode,
		DistrictCode: districtCode,
		WardCode:     wardCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "population-service",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.refreshSecret)
}

func (m *Manager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return m.validate(tokenStr, m.accessSecret)
}

func (m *Manager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return m.validate(tokenStr, m.refreshSecret)
}

func (m *Manager) validate(tokenStr string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}