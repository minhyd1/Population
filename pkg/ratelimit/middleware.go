package ratelimit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"population-service/pkg/middleware"
)

// Rules định nghĩa giới hạn cho từng nhóm endpoint trong dự án.
// Điều chỉnh các con số này theo nhu cầu thực tế.
var (
	// RuleLogin: 10 lần / 15 phút theo IP — bảo vệ brute-force mật khẩu
	RuleLogin = Rule{Limit: 10, Window: 15 * time.Minute}

	// RuleRefresh: 30 lần / 15 phút theo IP — client thường auto-refresh
	RuleRefresh = Rule{Limit: 30, Window: 15 * time.Minute}

	// RuleRegister: 5 lần / giờ theo IP — tránh tạo hàng loạt tài khoản
	RuleRegister = Rule{Limit: 5, Window: time.Hour}

	// RuleAPI: 200 request / phút theo userID — giới hạn API thông thường
	RuleAPI = Rule{Limit: 200, Window: time.Minute}

	// RuleWrite: 60 lần / phút theo userID — POST/PATCH/DELETE
	RuleWrite = Rule{Limit: 60, Window: time.Minute}

	// RuleExport: 10 lần / giờ theo userID — export tốn tài nguyên
	RuleExport = Rule{Limit: 10, Window: time.Hour}

	// RuleAdmin: 100 lần / phút theo userID — admin thao tác nhiều hơn
	RuleAdmin = Rule{Limit: 100, Window: time.Minute}
)

// ByIP áp dụng rate limit theo địa chỉ IP.
// Dùng cho endpoint public chưa có authentication (login, refresh, register).
func (l *Limiter) ByIP(rule Rule, group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c)
		key := KeyByIP(group, ip)

		result, err := l.Check(c.Request.Context(), key, rule)
		if err != nil {
			// Redis lỗi → log rồi cho qua, không chặn user
			c.Next()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "too many requests, please try again later",
				"retry_after": int(result.RetryAfter.Seconds()),
			})
			return
		}

		c.Next()
	}
}

// ByUser áp dụng rate limit theo userID từ JWT claims.
// Dùng cho endpoint đã có authentication.
// Nếu không có claims (chưa login), fallback về ByIP.
func (l *Limiter) ByUser(rule Rule, group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := middleware.GetClaims(c)

		var key string
		if claims != nil {
			key = KeyByUser(group, claims.UserID)
		} else {
			// Fallback về IP nếu chưa có JWT
			key = KeyByIP(group, clientIP(c))
		}

		result, err := l.Check(c.Request.Context(), key, rule)
		if err != nil {
			c.Next()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "too many requests, please slow down",
				"retry_after": int(result.RetryAfter.Seconds()),
			})
			return
		}

		c.Next()
	}
}

// ── Header helpers ────────────────────────────────────────

// setRateLimitHeaders thêm các header chuẩn RateLimit vào response.
// Client (frontend, Postman) nhìn vào đây để biết còn bao nhiêu quota.
func setRateLimitHeaders(c *gin.Context, r Result) {
	c.Header("X-RateLimit-Limit",     strconv.FormatInt(r.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(r.Remaining, 10))
	if !r.Allowed && r.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(int(r.RetryAfter.Seconds())))
	}
}

// clientIP lấy IP thực của client, ưu tiên X-Forwarded-For (qua proxy/nginx).
func clientIP(c *gin.Context) string {
	// X-Forwarded-For: ip1, ip2, ... — ip1 là client thật
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// Lấy phần đầu trước dấu phẩy
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}
	return c.ClientIP()
}