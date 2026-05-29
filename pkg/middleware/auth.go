package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jwtpkg "population-service/pkg/jwt"
)

const (
	// Key để lấy claims từ gin context
	ClaimsKey = "jwt_claims"
)

// JWTAuth xác thực access token, đặt claims vào context
func JWTAuth(jwtManager *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "missing Authorization header",
			})
			return
		}

		// Format: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "invalid Authorization format, expected 'Bearer <token>'",
			})
			return
		}

		claims, err := jwtManager.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "invalid or expired access token",
			})
			return
		}

		// Lưu claims vào context để handler dùng
		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

// RequireRole chỉ cho phép các role được chỉ định đi qua
// Dùng SAU middleware JWTAuth
func RequireRole(roles ...jwtpkg.Role) gin.HandlerFunc {
	roleSet := make(map[jwtpkg.Role]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		claimsRaw, exists := c.Get(ClaimsKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized",
			})
			return
		}

		claims, ok := claimsRaw.(*jwtpkg.Claims)
		if !ok || !roleSet[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "forbidden: insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

// GetClaims là helper để lấy claims từ gin context trong handler
func GetClaims(c *gin.Context) *jwtpkg.Claims {
	claimsRaw, _ := c.Get(ClaimsKey)
	claims, _ := claimsRaw.(*jwtpkg.Claims)
	return claims
}