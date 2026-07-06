package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Config struct {
	AppPort       string
	AppEnv        string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	EncryptionKey    string
	JWTAccessSecret  string
	JWTRefreshSecret string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	RedisEnabled  bool // false = chạy không có Redis (fallback in-memory)
}

func LoadConfig() *Config {
	_ = godotenv.Load("configs/.env")

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	cfg := &Config{
		AppPort:          getEnv("APP_PORT", "8080"),
		AppEnv:           getEnv("APP_ENV", "development"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBUser:           getEnv("DB_USER", "postgres"),
		DBPassword:       getEnv("DB_PASSWORD", ""),
		DBName:           getEnv("DB_NAME", "population_db"),
		DBSSLMode:        getEnv("DB_SSLMODE", "disable"),
		EncryptionKey:    getEnv("ENCRYPTION_KEY", ""),
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", "your-access-secret-key"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-refresh-secret-key"),
		RedisHost:        getEnv("REDIS_HOST", "localhost"),
		RedisPort:        getEnv("REDIS_PORT", "6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          redisDB,
		RedisEnabled:     getEnv("REDIS_ENABLED", "true") != "false",
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	return cfg
}

// Validate kiểm tra các ràng buộc bắt buộc và cảnh báo/chặn cấu hình không an
// toàn. Trước đây chỉ có ENCRYPTION_KEY được kiểm tra; JWT secret có thể âm
// thầm chạy với giá trị mặc định "your-access-secret-key" ở production mà
// không ai biết cho tới khi bị khai thác.
func (c *Config) Validate() error {
	var missing []string

	if c.EncryptionKey == "" {
		missing = append(missing, "ENCRYPTION_KEY")
	}
	if len(missing) > 0 {
		return fmt.Errorf("thiếu biến môi trường bắt buộc: %v", missing)
	}

	insecureDefaults := map[string]string{
		"JWT_ACCESS_SECRET":  "your-access-secret-key",
		"JWT_REFRESH_SECRET": "your-refresh-secret-key",
	}
	current := map[string]string{
		"JWT_ACCESS_SECRET":  c.JWTAccessSecret,
		"JWT_REFRESH_SECRET": c.JWTRefreshSecret,
	}
	for key, defaultVal := range insecureDefaults {
		if current[key] == defaultVal {
			if c.AppEnv == "production" {
				return fmt.Errorf("%s vẫn đang dùng giá trị mặc định không an toàn — bắt buộc phải set khi APP_ENV=production", key)
			}
			log.Printf("⚠️  %s đang dùng giá trị mặc định — CHỈ chấp nhận ở development, không được dùng ở production", key)
		}
	}

	if len(c.JWTAccessSecret) < 16 {
		return fmt.Errorf("JWT_ACCESS_SECRET quá ngắn (%d ký tự) — cần tối thiểu 16 ký tự", len(c.JWTAccessSecret))
	}
	if len(c.JWTRefreshSecret) < 16 {
		return fmt.Errorf("JWT_REFRESH_SECRET quá ngắn (%d ký tự) — cần tối thiểu 16 ký tự", len(c.JWTRefreshSecret))
	}

	return nil
}

func ConnectDB(cfg *Config) *sqlx.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	var db *sqlx.DB
	var err error
	for i := 0; i < 5; i++ {
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			break
		}
		log.Printf("DB connection attempt %d failed: %v. Retrying in 2s...", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Cannot connect to PostgreSQL after 5 attempts: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("✅ Connected to PostgreSQL")
	return db
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}