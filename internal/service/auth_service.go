package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"population-service/internal/model"
	"population-service/internal/repository"
	jwtpkg "population-service/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

// Thời gian sống của token (có thể đưa vào config)
const (
	AccessTokenTTL  = 15 * time.Minute    // access token ngắn
	RefreshTokenTTL = 7 * 24 * time.Hour  // refresh token dài
)

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error)
	Refresh(ctx context.Context, refreshTokenStr string) (*model.TokenResponse, error)
	Logout(ctx context.Context, refreshTokenStr string) error
	LogoutAll(ctx context.Context, userID string) error
}

type authService struct {
	userRepo   repository.UserRepository
	jwtManager *jwtpkg.Manager
}

func NewAuthService(userRepo repository.UserRepository, jwtManager *jwtpkg.Manager) AuthService {
	return &authService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

func (s *authService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	// Kiểm tra username đã tồn tại chưa
	existing, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username already exists")
	}

	// Validate: citizen phải có citizen_id
	if req.Role == jwtpkg.RoleCitizen && (req.CitizenID == nil || *req.CitizenID == "") {
		return nil, errors.New("citizen_id is required for role 'citizen'")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		CitizenID:    req.CitizenID,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (*model.TokenResponse, error) {
	// Tìm user
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid username or password")
	}

	// Kiểm tra password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return s.issueTokenPair(ctx, user)
}

func (s *authService) Refresh(ctx context.Context, refreshTokenStr string) (*model.TokenResponse, error) {
	// Xác thực chữ ký refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Kiểm tra token còn trong DB không (chưa bị revoke)
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

	// Revoke token cũ (rotation: mỗi lần refresh = đổi token mới)
	if err := s.userRepo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	// Lấy thông tin user mới nhất
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	return s.issueTokenPair(ctx, user)
}

func (s *authService) Logout(ctx context.Context, refreshTokenStr string) error {
	tokenHash := hashToken(refreshTokenStr)
	return s.userRepo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *authService) LogoutAll(ctx context.Context, userID string) error {
	return s.userRepo.RevokeAllUserTokens(ctx, userID)
}

// issueTokenPair tạo cặp access + refresh token và lưu refresh token vào DB
func (s *authService) issueTokenPair(ctx context.Context, user *model.User) (*model.TokenResponse, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	// Lưu hash của refresh token vào DB
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

// hashToken tạo SHA-256 hash của token để lưu DB (không lưu plaintext)
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}