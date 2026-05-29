package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"population-service/internal/model"
	jwtpkg "population-service/pkg/jwt"
)

type UserRepository interface {
	// CRUD user
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id string) (*model.User, error)
	ListUsers(ctx context.Context) ([]*model.User, error)
	UpdateRole(ctx context.Context, userID string, role jwtpkg.Role, provinceCode, districtCode, wardCode *string) error
	SetActive(ctx context.Context, userID string, active bool) error
	UpdatePassword(ctx context.Context, userID, newHash string) error

	// Refresh token
	SaveRefreshToken(ctx context.Context, rt *model.RefreshToken) error
	FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (id, username, password_hash, role, province_code, district_code, ward_code, citizen_id, is_active)
		VALUES (:id, :username, :password_hash, :role, :province_code, :district_code, :ward_code, :citizen_id, :is_active)
	`
	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user,
		`SELECT * FROM users WHERE username = $1 AND is_active = true`, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	var users []*model.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY created_at DESC`)
	return users, err
}

func (r *userRepository) UpdateRole(ctx context.Context, userID string, role jwtpkg.Role, provinceCode, districtCode, wardCode *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET role=$1, province_code=$2, district_code=$3, ward_code=$4, updated_at=NOW() WHERE id=$5`,
		role, provinceCode, districtCode, wardCode, userID)
	return err
}

func (r *userRepository) SetActive(ctx context.Context, userID string, active bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_active=$1, updated_at=NOW() WHERE id=$2`,
		active, userID)
	return err
}

func (r *userRepository) UpdatePassword(ctx context.Context, userID, newHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`,
		newHash, userID)
	return err
}

func (r *userRepository) SaveRefreshToken(ctx context.Context, rt *model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES (:id, :user_id, :token_hash, :expires_at)`
	_, err := r.db.NamedExecContext(ctx, query, rt)
	return err
}

func (r *userRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.db.GetContext(ctx, &rt,
		`SELECT * FROM refresh_tokens WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &rt, err
}

func (r *userRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *userRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}