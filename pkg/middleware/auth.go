package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jwtpkg "population-service/pkg/jwt"
	redispkg "population-service/pkg/redis"
)

const ClaimsKey = "jwt_claims"

// JWTAuth xác thực access token.
// Nếu redisClient != nil, còn kiểm tra thêm token có bị blacklist không
// (xảy ra khi user logout trước khi token hết hạn).
func JWTAuth(jwtManager *jwtpkg.Manager, redisClient ...*redispkg.Client) gin.HandlerFunc {
	// redisClient là variadic để không bắt buộc — backward compatible
	var rdb *redispkg.Client
	if len(redisClient) > 0 {
		rdb = redisClient[0]
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "missing Authorization header",
			})
			return
		}

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

		// Kiểm tra token có bị blacklist không (chỉ khi Redis available)
		// TokenID = JTI claim — định danh duy nhất của token này
		if rdb != nil && claims.ID != "" {
			blacklisted, err := rdb.IsBlacklisted(c.Request.Context(), claims.ID)
			if err == nil && blacklisted {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "token has been revoked",
				})
				return
			}
		}

		// Lưu claims vào gin context để handler và middleware sau dùng
		c.Set(ClaimsKey, claims)
		c.Set(string(ContextKeyUserID), claims.UserID)
		c.Set(string(ContextKeyUsername), claims.Username)
		c.Set(string(ContextKeyUserRole), string(claims.Role))

		// Inject vào request context để service layer lấy qua ctx.Value()
		reqCtx := c.Request.Context()
		reqCtx = context.WithValue(reqCtx, ContextKeyUserID, claims.UserID)
		reqCtx = context.WithValue(reqCtx, ContextKeyUsername, claims.Username)
		reqCtx = context.WithValue(reqCtx, ContextKeyUserRole, string(claims.Role))
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}

// RequireRole cho phép các role được chỉ định đi qua
func RequireRole(roles ...jwtpkg.Role) gin.HandlerFunc {
	roleSet := make(map[jwtpkg.Role]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized",
			})
			return
		}
		if !roleSet[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":   false,
				"error":     "forbidden: insufficient permissions",
				"your_role": string(claims.Role),
			})
			return
		}
		c.Next()
	}
}

// RequireMinRole dùng role hierarchy
func RequireMinRole(minRole jwtpkg.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
			return
		}
		if roleLevel(claims.Role) < roleLevel(minRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":   false,
				"error":     "forbidden: insufficient permissions",
				"your_role": string(claims.Role),
				"required":  string(minRole),
			})
			return
		}
		c.Next()
	}
}

// RequireScopeMatch đảm bảo user chỉ truy cập đúng địa bàn của mình
func RequireScopeMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
			return
		}

		if claims.Role == jwtpkg.RoleSuperAdmin || claims.Role == jwtpkg.RoleNationalManager {
			c.Next()
			return
		}

		if pc := paramOrQuery(c, "province_code"); pc != "" && claims.ProvinceCode != "" {
			if claims.ProvinceCode != pc {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "forbidden: outside your province scope",
				})
				return
			}
		}

		if dc := paramOrQuery(c, "district_code"); dc != "" && claims.DistrictCode != "" {
			if claims.DistrictCode != dc {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "forbidden: outside your district scope",
				})
				return
			}
		}

		if wc := paramOrQuery(c, "ward_code"); wc != "" && claims.WardCode != "" {
			if claims.WardCode != wc {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "forbidden: outside your ward scope",
				})
				return
			}
		}

		c.Next()
	}
}

// GetClaims lấy claims từ gin context
func GetClaims(c *gin.Context) *jwtpkg.Claims {
	raw, _ := c.Get(ClaimsKey)
	claims, _ := raw.(*jwtpkg.Claims)
	return claims
}

func roleLevel(r jwtpkg.Role) int {
	switch r {
	case jwtpkg.RoleSuperAdmin:      return 8
	case jwtpkg.RoleNationalManager: return 7
	case jwtpkg.RoleProvinceManager: return 6
	case jwtpkg.RoleDistrictManager: return 5
	case jwtpkg.RoleWardOfficer:     return 4
	case jwtpkg.RoleDataEntry:       return 3
	case jwtpkg.RoleAuditor:         return 3
	case jwtpkg.RoleAnalyticsViewer: return 2
	case jwtpkg.RoleCitizenSelf:     return 1
	default:                          return 0
	}
}

func paramOrQuery(c *gin.Context, key string) string {
	if v := c.Param(key); v != "" {
		return v
	}
	return c.Query(key)
}