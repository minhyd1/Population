package main

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"population-service/internal/handler"
	"population-service/internal/repository"
	"population-service/internal/service"
	citizensvc "population-service/internal/service/citizen"
	"population-service/pkg/crypto"
	jwtpkg "population-service/pkg/jwt"
	"population-service/pkg/ratelimit"
	redispkg "population-service/pkg/redis"
)

// Container gom toàn bộ dependency đã khởi tạo (repo/service/handler/hạ tầng).
//
// Đây là bước tách DI ra khỏi main.go: trước đây main.go vừa đọc config,
// vừa connect DB/Redis, vừa new tất cả repo/service/handler, vừa định nghĩa
// route — một "God Main" ~316 dòng khó bảo trì. Giờ container.go chỉ lo
// wiring (tương đương InitRepository → InitService → InitHandler), router.go
// lo route, main.go chỉ còn lắp ráp + chạy server.
type Container struct {
	Config *Config
	DB     *sqlx.DB

	RedisClient *redispkg.Client
	Limiter     *ratelimit.Limiter
	JWTManager  *jwtpkg.Manager

	CitizenHandler    *handler.CitizenHandler
	AuthHandler       *handler.AuthHandler
	AuditHandler      *handler.AuditHandler
	TransferHandler   *handler.TransferHandler
	AssignmentHandler *handler.AssignmentHandler
}

// NewContainer khởi tạo toàn bộ dependency của ứng dụng theo đúng thứ tự:
// hạ tầng (DB/Redis/crypto/JWT) → repository → service → handler.
func NewContainer(cfg *Config, db *sqlx.DB) (*Container, error) {
	enc, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	jwtManager := jwtpkg.New(
		cfg.JWTAccessSecret,
		cfg.JWTRefreshSecret,
		15*time.Minute,
		7*24*time.Hour,
	)

	// ── Redis (optional) ──────────────────────────────────────
	// Nếu REDIS_ENABLED=false hoặc Redis không kết nối được,
	// server vẫn chạy bình thường nhưng không có rate limiting và token blacklist.
	var redisClient *redispkg.Client
	if cfg.RedisEnabled {
		redisClient, err = redispkg.New(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			log.Printf("⚠️  Redis unavailable: %v — running without rate limiting", err)
			redisClient = nil
		}
	}

	// ── Rate limiter ───────────────────────────────────────────
	var limiter *ratelimit.Limiter
	if redisClient != nil {
		limiter = ratelimit.New(redisClient)
		log.Println("✅ Rate limiting enabled")
	} else {
		log.Println("⚠️  Rate limiting disabled (no Redis)")
	}

	// ── Repositories ──────────────────────────────────────────
	citizenRepo := repository.NewCitizenRepository(db)
	provinceRepo := repository.NewProvinceRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	householdRepo := repository.NewHouseholdRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	adminUnitRepo := repository.NewAdminUnitRepository(db)

	// ── Services ──────────────────────────────────────────────
	// NewAuthService nhận Redis optional — nếu nil, Logout vẫn hoạt động
	// nhưng không blacklist access token.
	citizenSvc := citizensvc.New(citizenRepo, provinceRepo, auditRepo, enc)
	authSvc := service.NewAuthService(userRepo, jwtManager, redisClient)
	auditSvc := service.NewAuditService(auditRepo)
	transferSvc := service.NewTransferService(transferRepo, householdRepo, citizenRepo, auditRepo)
	assignmentSvc := service.NewAssignmentService(adminUnitRepo, userRepo)

	// ── Handlers ──────────────────────────────────────────────
	c := &Container{
		Config:            cfg,
		DB:                db,
		RedisClient:       redisClient,
		Limiter:           limiter,
		JWTManager:        jwtManager,
		CitizenHandler:    handler.NewCitizenHandler(citizenSvc),
		AuthHandler:       handler.NewAuthHandler(authSvc),
		AuditHandler:      handler.NewAuditHandler(auditSvc),
		TransferHandler:   handler.NewTransferHandler(transferSvc),
		AssignmentHandler: handler.NewAssignmentHandler(assignmentSvc),
	}
	return c, nil
}

// Close giải phóng các kết nối hạ tầng (Redis). DB được main.go tự defer Close
// vì main.go là nơi mở nó.
func (c *Container) Close() {
	if c.RedisClient != nil {
		c.RedisClient.Close()
	}
}
