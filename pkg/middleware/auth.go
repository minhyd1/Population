package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jwtpkg "population-service/pkg/jwt"
)

const ClaimsKey = "jwt_claims"

// JWTAuth xác thực access token, đặt claims vào gin context
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

		c.Set(ClaimsKey, claims)
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
				"success": false,
				"error":   "forbidden: insufficient permissions",
				"your_role": string(claims.Role),
			})
			return
		}
		c.Next()
	}
}

// RequireMinRole dùng role hierarchy — role có cấp >= minRole thì qua
// Thứ tự ưu tiên từ cao xuống thấp:
// super_admin > national_manager > province_manager > district_manager
// > ward_officer > data_entry | auditor | analytics_viewer | citizen_self
func RequireMinRole(minRole jwtpkg.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
			return
		}
		if roleLevel(claims.Role) < roleLevel(minRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":    false,
				"error":      "forbidden: insufficient permissions",
				"your_role":  string(claims.Role),
				"required":   string(minRole),
			})
			return
		}
		c.Next()
	}
}

// RequireScopeMatch đảm bảo user chỉ truy cập đúng địa bàn của mình.
// super_admin và national_manager không bị giới hạn địa bàn.
// Dùng cho các endpoint nhận :province_code, :district_code, :ward_code.
func RequireScopeMatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
			return
		}

		// super_admin và national_manager: không giới hạn địa bàn
		if claims.Role == jwtpkg.RoleSuperAdmin || claims.Role == jwtpkg.RoleNationalManager {
			c.Next()
			return
		}

		// Kiểm tra province_code nếu có trong path/query
		if pc := paramOrQuery(c, "province_code"); pc != "" && claims.ProvinceCode != "" {
			if claims.ProvinceCode != pc {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "forbidden: outside your province scope",
				})
				return
			}
		}

		// Kiểm tra district_code
		if dc := paramOrQuery(c, "district_code"); dc != "" && claims.DistrictCode != "" {
			if claims.DistrictCode != dc {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "forbidden: outside your district scope",
				})
				return
			}
		}

		// Kiểm tra ward_code
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

// roleLevel trả về cấp bậc của role (số càng cao = càng nhiều quyền)
func roleLevel(r jwtpkg.Role) int {
	switch r {
	case jwtpkg.RoleSuperAdmin:
		return 8
	case jwtpkg.RoleNationalManager:
		return 7
	case jwtpkg.RoleProvinceManager:
		return 6
	case jwtpkg.RoleDistrictManager:
		return 5
	case jwtpkg.RoleWardOfficer:
		return 4
	case jwtpkg.RoleDataEntry:
		return 3
	case jwtpkg.RoleAuditor:
		return 3
	case jwtpkg.RoleAnalyticsViewer:
		return 2
	case jwtpkg.RoleCitizenSelf:
		return 1
	default:
		return 0
	}
}

func paramOrQuery(c *gin.Context, key string) string {
	if v := c.Param(key); v != "" {
		return v
	}
	return c.Query(key)
}