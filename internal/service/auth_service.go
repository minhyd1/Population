package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"population-service/internal/model"
	"population-service/internal/repository"
	jwtpkg "population-service/pkg/jwt"
	redispkg "population-service/pkg/redis"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error)
	Refresh(ctx context.Context, refreshTokenStr string) (*model.TokenResponse, error)
	// Logout nhận cả accessToken để blacklist ngay lập tức
	Logout(ctx context.Context, refreshTokenStr, accessTokenJTI string) error
	LogoutAll(ctx context.Context, userID string) error
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	ResetPassword(ctx context.Context, targetUserID, newPassword string) error
	UpdateUser(ctx context.Context, targetUserID string, req model.UpdateUserRequest) error
	SetActive(ctx context.Context, targetUserID string, active bool) error
	ListUsers(ctx context.Context) ([]*model.User, error)
}

type authService struct {
	userRepo   repository.UserRepository
	jwtManager *jwtpkg.Manager
	redis      *redispkg.Client // nil nếu Redis không cấu hình
}

func NewAuthService(userRepo repository.UserRepository, jwtManager *jwtpkg.Manager, redis ...*redispkg.Client) AuthService {
	svc := &authService{userRepo: userRepo, jwtManager: jwtManager}
	if len(redis) > 0 {
		svc.redis = redis[0]
	}
	return svc
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	if !req.Role.IsValid() {
		return nil, errors.New("invalid role")
	}
	if err := validateScopeForRole(req.Role, req.ProvinceCode, req.DistrictCode, req.WardCode, req.CitizenID); err != nil {
		return nil, err
	}
	existing, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		ProvinceCode: req.ProvinceCode,
		DistrictCode: req.DistrictCode,
		WardCode:     req.WardCode,
		CitizenID:    req.CitizenID,
		IsActive:     true,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}
	return s.issueTokenPair(ctx, user)
}

func (s *authService) Refresh(ctx context.Context, refreshTokenStr string) (*model.TokenResponse, error) {
	claims, err := s.jwtManager.ValidateRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}
	tokenHash := hashToken(refreshTokenStr)
	stored, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, errors.New("refresh token has been revoked")
	}
	if time.Now().After(stored.ExpiresAt) {
		return nil, errors.New("refresh token has expired")
	}
	if err := s.userRepo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	return s.issueTokenPair(ctx, user)
}

// Logout revoke refresh token (DB) và blacklist access token (Redis).
// accessTokenJTI là claims.ID từ JWT — nếu rỗng, bỏ qua bước blacklist.
func (s *authService) Logout(ctx context.Context, refreshTokenStr, accessTokenJTI string) error {
	// 1. Revoke refresh token trong DB
	if refreshTokenStr != "" {
		_ = s.userRepo.RevokeRefreshToken(ctx, hashToken(refreshTokenStr))
	}
	// 2. Blacklist access token trong Redis cho đến khi nó tự hết hạn
	if s.redis != nil && accessTokenJTI != "" {
		_ = s.redis.SetBlacklist(ctx, accessTokenJTI, s.jwtManager.AccessTTL())
	}
	return nil
}

func (s *authService) LogoutAll(ctx context.Context, userID string) error {
	return s.userRepo.RevokeAllUserTokens(ctx, userID)
	// Lưu ý: access token đang lưu hành vẫn còn hiệu lực đến khi hết hạn (15 phút).
	// Nếu cần revoke ngay lập tức, cần lưu toàn bộ JTI đang active — phức tạp hơn.
}

func (s *authService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("incorrect current password")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_ = s.userRepo.RevokeAllUserTokens(ctx, userID)
	return s.userRepo.UpdatePassword(ctx, userID, string(newHash))
}

func (s *authService) ResetPassword(ctx context.Context, targetUserID, newPassword string) error {
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_ = s.userRepo.RevokeAllUserTokens(ctx, targetUserID)
	return s.userRepo.UpdatePassword(ctx, targetUserID, string(newHash))
}

func (s *authService) UpdateUser(ctx context.Context, targetUserID string, req model.UpdateUserRequest) error {
	user, err := s.userRepo.FindByID(ctx, targetUserID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	role := user.Role
	if req.Role != nil {
		if !req.Role.IsValid() {
			return errors.New("invalid role")
		}
		role = *req.Role
	}
	if err := s.userRepo.UpdateRole(ctx, targetUserID, role, req.ProvinceCode, req.DistrictCode, req.WardCode); err != nil {
		return err
	}
	if req.Role != nil {
		_ = s.userRepo.RevokeAllUserTokens(ctx, targetUserID)
	}
	return nil
}

func (s *authService) SetActive(ctx context.Context, targetUserID string, active bool) error {
	if err := s.userRepo.SetActive(ctx, targetUserID, active); err != nil {
		return err
	}
	if !active {
		_ = s.userRepo.RevokeAllUserTokens(ctx, targetUserID)
	}
	return nil
}

func (s *authService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.userRepo.ListUsers(ctx)
}

func (s *authService) issueTokenPair(ctx context.Context, user *model.User) (*model.TokenResponse, error) {
	pc := strVal(user.ProvinceCode)
	dc := strVal(user.DistrictCode)
	wc := strVal(user.WardCode)

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role, pc, dc, wc)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role, pc, dc, wc)
	if err != nil {
		return nil, err
	}
	rt := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
	}
	if err := s.userRepo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, err
	}
	return &model.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(AccessTokenTTL.Seconds()),
	}, nil
}

func validateScopeForRole(role jwtpkg.Role, province, district, ward, citizenID *string) error {
	switch role {
	case jwtpkg.RoleProvinceManager:
		if province == nil || *province == "" {
			return errors.New("province_code is required for role 'province_manager'")
		}
	case jwtpkg.RoleDistrictManager:
		if district == nil || *district == "" {
			return errors.New("district_code is required for role 'district_manager'")
		}
	case jwtpkg.RoleWardOfficer:
		if ward == nil || *ward == "" {
			return errors.New("ward_code is required for role 'ward_officer'")
		}
	case jwtpkg.RoleCitizenSelf:
		if citizenID == nil || *citizenID == "" {
			return errors.New("citizen_id is required for role 'citizen_self'")
		}
	}
	return nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}