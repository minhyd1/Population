package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"population-service/internal/model"
	"population-service/internal/service"
	"population-service/pkg/middleware"
	"population-service/pkg/response"
)

type AuthHandler struct {
	authSvc service.AuthService
}

func NewAuthHandler(authSvc service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// ──────────────────────────────────────────────────────────
// Public endpoints
// ──────────────────────────────────────────────────────────

// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	tokens, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	response.OK(c, tokens)
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req model.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	tokens, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	response.OK(c, tokens)
}

// ──────────────────────────────────────────────────────────
// Protected endpoints (cần JWT)
// ──────────────────────────────────────────────────────────

// GET /auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	// Lấy user đầy đủ từ DB để trả province/district/ward
	user, err := h.authSvc.ListUsers(c.Request.Context())
	_ = user
	_ = err
	response.OK(c, model.MeResponse{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
		ProvinceCode: nilStr(claims.ProvinceCode),
		DistrictCode: nilStr(claims.DistrictCode),
		WardCode:     nilStr(claims.WardCode),
	})
}

// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req model.RefreshRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "unauthorized",
		})
		return
	}

	if err := h.authSvc.Logout(
		c.Request.Context(),
		req.RefreshToken,
		claims.ID, // JTI của access token
	); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "logged out successfully",
	})
}

// POST /auth/logout-all
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	if err := h.authSvc.LogoutAll(c.Request.Context(), claims.UserID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "all sessions revoked"})
}

// POST /auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}
	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := h.authSvc.ChangePassword(c.Request.Context(), claims.UserID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	response.OK(c, gin.H{"message": "password changed, please login again"})
}

// ──────────────────────────────────────────────────────────
// Admin-only endpoints (super_admin)
// ──────────────────────────────────────────────────────────

// POST /admin/users — tạo tài khoản mới
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	user, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": err.Error()})
			return
		}
		response.BadRequest(c, "REGISTER_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"role":          user.Role,
			"province_code": user.ProvinceCode,
			"district_code": user.DistrictCode,
			"ward_code":     user.WardCode,
		},
	})
}

// GET /admin/users — danh sách tất cả user
func (h *AuthHandler) ListUsers(c *gin.Context) {
	users, err := h.authSvc.ListUsers(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	// Ẩn password_hash trước khi trả
	type safeUser struct {
		ID           string  `json:"id"`
		Username     string  `json:"username"`
		Role         string  `json:"role"`
		ProvinceCode *string `json:"province_code"`
		DistrictCode *string `json:"district_code"`
		WardCode     *string `json:"ward_code"`
		IsActive     bool    `json:"is_active"`
	}
	result := make([]safeUser, 0, len(users))
	for _, u := range users {
		result = append(result, safeUser{
			ID:           u.ID,
			Username:     u.Username,
			Role:         string(u.Role),
			ProvinceCode: u.ProvinceCode,
			DistrictCode: u.DistrictCode,
			WardCode:     u.WardCode,
			IsActive:     u.IsActive,
		})
	}
	response.OK(c, result)
}

// PATCH /admin/users/:id — gán role / cập nhật địa bàn
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	targetID := c.Param("id")
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := h.authSvc.UpdateUser(c.Request.Context(), targetID, req); err != nil {
		response.BadRequest(c, "UPDATE_ERROR", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "user updated"})
}

// POST /admin/users/:id/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	targetID := c.Param("id")
	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if err := h.authSvc.ResetPassword(c.Request.Context(), targetID, req.NewPassword); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "password reset successfully"})
}

// POST /admin/users/:id/lock — khóa tài khoản
func (h *AuthHandler) LockUser(c *gin.Context) {
	targetID := c.Param("id")
	if err := h.authSvc.SetActive(c.Request.Context(), targetID, false); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "user locked"})
}

// POST /admin/users/:id/unlock — mở khóa tài khoản
func (h *AuthHandler) UnlockUser(c *gin.Context) {
	targetID := c.Param("id")
	if err := h.authSvc.SetActive(c.Request.Context(), targetID, true); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "user unlocked"})
}

// ──────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────
func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}