package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Config holds all application config
type Config struct {
	AppPort    string
	AppEnv     string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	EncryptionKey string
	JWTAccessSecret  string
    JWTRefreshSecret string
}

// LoadConfig loads config from .env and environment variables
func LoadConfig() *Config {
	// Load .env file (ignore error in production where env vars are set directly)
	_ = godotenv.Load("configs/.env")

	cfg := &Config{
		AppPort:       getEnv("APP_PORT", "8080"),
		AppEnv:        getEnv("APP_ENV", "development"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "5432"),
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "population_db"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", "your-access-secret-key"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-refresh-secret-key"),
	}

	if cfg.EncryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
	}

	return cfg
}

// ConnectDB tạo kết nối PostgreSQL với retry
func ConnectDB(cfg *Config) *sqlx.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	var db *sqlx.DB
	var err error

	// Retry up to 5 times
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

	// Connection pool settings
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
