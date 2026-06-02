package ratelimit

import (
	"context"
	"fmt"
	"time"

	redispkg "population-service/pkg/redis"
)

// Rule định nghĩa giới hạn cho một nhóm endpoint
type Rule struct {
	// Limit là số request tối đa cho phép trong Window
	Limit int64
	// Window là khoảng thời gian áp dụng giới hạn
	Window time.Duration
}

// Result là kết quả kiểm tra rate limit cho 1 request
type Result struct {
	Allowed    bool          // true = được phép tiếp tục
	Limit      int64         // giới hạn tối đa
	Remaining  int64         // còn lại bao nhiêu request trong window này
	RetryAfter time.Duration // chờ bao lâu nếu bị chặn
}

// Limiter thực hiện thuật toán Fixed Window Counter trên Redis.
// Mỗi (key, window) có 1 counter riêng, tự xóa sau khi window kết thúc.
type Limiter struct {
	redis *redispkg.Client
}

func New(redis *redispkg.Client) *Limiter {
	return &Limiter{redis: redis}
}

// Check kiểm tra xem key có vượt giới hạn không.
// key thường là tổ hợp: "rl:<group>:<identifier>"
// ví dụ: "rl:login:192.168.1.1" hoặc "rl:api:user-uuid-xxx"
func (l *Limiter) Check(ctx context.Context, key string, rule Rule) (Result, error) {
	// Incr tăng counter lên 1, trả về giá trị sau khi tăng
	count, err := l.redis.Incr(ctx, key)
	if err != nil {
		// Nếu Redis lỗi, cho phép request qua (fail open) để tránh
		// Redis chết kéo theo cả API. Có thể đổi thành fail closed tùy yêu cầu.
		return Result{Allowed: true, Limit: rule.Limit, Remaining: rule.Limit}, nil
	}

	// Lần đầu tạo key — đặt TTL = 1 window
	// Những lần sau Expire vẫn gọi nhưng không ảnh hưởng vì key đã có TTL
	if count == 1 {
		_ = l.redis.Expire(ctx, key, rule.Window)
	}

	remaining := rule.Limit - count
	if remaining < 0 {
		remaining = 0
	}

	if count > rule.Limit {
		// Lấy TTL còn lại để trả Retry-After cho client
		ttl, _ := l.redis.TTL(ctx, key)
		return Result{
			Allowed:    false,
			Limit:      rule.Limit,
			Remaining:  0,
			RetryAfter: ttl,
		}, nil
	}

	return Result{
		Allowed:   true,
		Limit:     rule.Limit,
		Remaining: remaining,
	}, nil
}

// ── Preset keys ───────────────────────────────────────────
// Các hàm tạo key chuẩn hóa, dùng xuyên suốt middleware

// KeyByIP tạo key theo IP — dùng cho endpoint public (login, refresh)
func KeyByIP(group, ip string) string {
	return fmt.Sprintf("rl:%s:ip:%s", group, ip)
}

// KeyByUser tạo key theo userID — dùng cho endpoint authenticated
func KeyByUser(group, userID string) string {
	return fmt.Sprintf("rl:%s:user:%s", group, userID)
}

// KeyByIPAndUser tạo key kết hợp — tránh 1 user dùng nhiều IP
func KeyByIPAndUser(group, ip, userID string) string {
	return fmt.Sprintf("rl:%s:ip:%s:user:%s", group, ip, userID)
}