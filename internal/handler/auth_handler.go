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

// Register godoc
// @Summary      Đăng ký tài khoản
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RegisterRequest true "Thông tin đăng ký"
// @Success      201 {object} model.User
// @Router       /auth/register [post]
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
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// Login godoc
// @Summary      Đăng nhập
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.LoginRequest true "Thông tin đăng nhập"
// @Success      200 {object} model.TokenResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	tokens, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	response.OK(c, tokens)
}

// Refresh godoc
// @Summary      Làm mới access token bằng refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RefreshRequest true "Refresh token"
// @Success      200 {object} model.TokenResponse
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req model.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	tokens, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	response.OK(c, tokens)
}

// Logout godoc
// @Summary      Đăng xuất (revoke refresh token hiện tại)
// @Tags         auth
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body model.RefreshRequest true "Refresh token cần revoke"
// @Success      200
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req model.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.authSvc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "logged out successfully"})
}

// LogoutAll godoc
// @Summary      Đăng xuất tất cả thiết bị (revoke mọi refresh token)
// @Tags         auth
// @Security     BearerAuth
// @Success      200
// @Router       /auth/logout-all [post]
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

// Me godoc
// @Summary      Lấy thông tin user hiện tại
// @Tags         auth
// @Security     BearerAuth
// @Success      200 {object} model.MeResponse
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	response.OK(c, model.MeResponse{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	})
}