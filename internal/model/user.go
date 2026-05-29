package model

import (
	"time"

	jwtpkg "population-service/pkg/jwt"
)

// User lưu thông tin tài khoản đăng nhập
type User struct {
	ID           string          `db:"id"`
	Username     string          `db:"username"`
	PasswordHash string          `db:"password_hash"`
	Role         jwtpkg.Role     `db:"role"`
	CitizenID    *string         `db:"citizen_id"` // NULL nếu là admin
	IsActive     bool            `db:"is_active"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

// RefreshToken lưu refresh token để có thể revoke
type RefreshToken struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"` // hash của refresh token (không lưu plaintext)
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}

// ==============================
// Request / Response DTOs
// ==============================

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterRequest struct {
	Username   string      `json:"username" binding:"required,min=3,max=50"`
	Password   string      `json:"password" binding:"required,min=6"`
	Role       jwtpkg.Role `json:"role" binding:"required,oneof=admin citizen"`
	CitizenID  *string     `json:"citizen_id"` // bắt buộc nếu role=citizen
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

type MeResponse struct {
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	Role      jwtpkg.Role `json:"role"`
	CitizenID *string     `json:"citizen_id,omitempty"`
}