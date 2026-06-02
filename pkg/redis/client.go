package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client bọc redis.Client, cung cấp các method cần dùng trong dự án
type Client struct {
	rdb *redis.Client
}

// New tạo Redis client và kiểm tra kết nối
func New(host, port, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,

		// Connection pool — tránh tạo connection mới mỗi request
		PoolSize:     10,
		MinIdleConns: 2,

		// Timeout
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cannot connect to Redis: %w", err)
	}

	log.Println("✅ Connected to Redis")
	return &Client{rdb: rdb}, nil
}

// Close đóng kết nối Redis khi server shutdown
func (c *Client) Close() error {
	return c.rdb.Close()
}

// ── Rate limiting primitives ──────────────────────────────

// Incr tăng counter của key lên 1, trả về giá trị mới.
// Nếu key chưa tồn tại, Redis tự tạo với giá trị 0 rồi tăng lên 1.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// Expire đặt TTL cho key. Dùng sau Incr để key tự xóa sau 1 window.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// TTL trả về thời gian sống còn lại của key (âm nếu không có TTL).
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// ── Token blacklist (dùng cho logout trước hạn) ───────────

// SetBlacklist đánh dấu một JWT access token là đã bị revoke.
// ttl nên bằng thời gian còn lại đến khi token hết hạn — sau đó key tự xóa.
func (c *Client) SetBlacklist(ctx context.Context, tokenID string, ttl time.Duration) error {
	key := blacklistKey(tokenID)
	return c.rdb.Set(ctx, key, "1", ttl).Err()
}

// IsBlacklisted kiểm tra token có bị blacklist không.
func (c *Client) IsBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := blacklistKey(tokenID)
	err := c.rdb.Get(ctx, key).Err()
	if err == redis.Nil {
		// Key không tồn tại = token chưa bị blacklist
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ── Session cache (optional) ──────────────────────────────

// SetJSON lưu giá trị JSON string với TTL.
func (c *Client) SetJSON(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// GetJSON lấy giá trị theo key. Trả ("", nil) nếu không tồn tại.
func (c *Client) GetJSON(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Del xóa key.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// ── Key helpers ───────────────────────────────────────────

func blacklistKey(tokenID string) string {
	return "blacklist:" + tokenID
}