package middleware

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestID thêm X-Request-ID vào mỗi request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger middleware log request/response
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		requestID, _ := c.Get("request_id")

		fmt.Printf("[%s] %s %s%s %d %v request_id=%v\n",
			time.Now().Format("2006-01-02 15:04:05"),
			c.Request.Method,
			path,
			func() string {
				if query != "" {
					return "?" + query
				}
				return ""
			}(),
			statusCode,
			latency,
			requestID,
		)
	}
}

// CORS middleware cho phép frontend call API
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID, X-Encryption-Key-Version")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// EncryptionKeyVersion thêm header thông báo version của encryption key
// Frontend dùng header này để biết dùng key nào để decrypt
func EncryptionKeyVersion(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		c.Header("X-Encryption-Key-Version", version)
	}
}

func generateRequestID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
