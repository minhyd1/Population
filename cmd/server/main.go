package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"population-service/internal/handler"
	"population-service/internal/repository"
	"population-service/internal/service"
	citizensvc "population-service/internal/service/citizen"
	"population-service/pkg/crypto"
	jwtpkg "population-service/pkg/jwt"
	"population-service/pkg/middleware"
	"population-service/pkg/ratelimit"
	redispkg "population-service/pkg/redis"
)

func main() {
	cfg := LoadConfig()
	db := ConnectDB(cfg)
	defer db.Close()

	enc, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to init encryptor: %v", err)
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
		} else {
			defer redisClient.Close()
		}
	}

	// ── Rate limiter ───────────────────────────────────────────
	// limiter sẽ là nil nếu Redis không kết nối được
	var limiter *ratelimit.Limiter
	if redisClient != nil {
		limiter = ratelimit.New(redisClient)
		log.Println("✅ Rate limiting enabled")
	} else {
		log.Println("⚠️  Rate limiting disabled (no Redis)")
	}

	// ── Repositories ──────────────────────────────────────────
	citizenRepo  := repository.NewCitizenRepository(db)
	provinceRepo := repository.NewProvinceRepository(db)
	userRepo     := repository.NewUserRepository(db)
	auditRepo    := repository.NewAuditRepository(db)
	householdRepo := repository.NewHouseholdRepository(db)
	transferRepo  := repository.NewTransferRepository(db)

	// ── Services ──────────────────────────────────────────────
	// NewAuthService nhận Redis optional — nếu nil, Logout vẫn hoạt động
	// nhưng không blacklist access token
	citizenSvc  := citizensvc.New(citizenRepo, provinceRepo, auditRepo, enc)
	authSvc     := service.NewAuthService(userRepo, jwtManager, redisClient)
	auditSvc    := service.NewAuditService(auditRepo)
	transferSvc := service.NewTransferService(db, transferRepo, householdRepo, citizenRepo, auditRepo)

	// ── Handlers ──────────────────────────────────────────────
	citizenHandler  := handler.NewCitizenHandler(citizenSvc)
	authHandler     := handler.NewAuthHandler(authSvc)
	auditHandler    := handler.NewAuditHandler(auditSvc)
	transferHandler := handler.NewTransferHandler(transferSvc)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		redisOK := redisClient != nil
		c.JSON(http.StatusOK, gin.H{
			"status":        "ok",
			"service":       "population-service",
			"redis_enabled": redisOK,
		})
	})

	v1 := r.Group("/api/v1")

	// ── Public ────────────────────────────────────────────────
	auth := v1.Group("/auth")
	{
		// Login: 10 request / 15 phút theo IP
		if limiter != nil {
			auth.POST("/login", limiter.ByIP(ratelimit.RuleLogin, "login"), authHandler.Login)
		} else {
			auth.POST("/login", authHandler.Login)
		}

		// Refresh: 30 request / 15 phút theo IP
		if limiter != nil {
			auth.POST("/refresh", limiter.ByIP(ratelimit.RuleRefresh, "refresh"), authHandler.Refresh)
		} else {
			auth.POST("/refresh", authHandler.Refresh)
		}
	}

	// ── Protected — cần JWT ───────────────────────────────────
	// JWTAuth nhận redisClient để kiểm tra token blacklist
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(jwtManager, redisClient))
	{
		authGroup := protected.Group("/auth")
		// Rate limit chung cho API: 200 req/phút theo userID
		if limiter != nil {
			authGroup.Use(limiter.ByUser(ratelimit.RuleAPI, "auth"))
		}
		{
			authGroup.GET("/me",               authHandler.Me)
			authGroup.POST("/logout",          authHandler.Logout)
			authGroup.POST("/logout-all",      authHandler.LogoutAll)
			authGroup.POST("/change-password", authHandler.ChangePassword)
		}

		// ── /admin — chỉ super_admin ──────────────────────────
		admin := protected.Group("/admin")
		admin.Use(middleware.RequireRole(jwtpkg.RoleSuperAdmin))
		if limiter != nil {
			admin.Use(limiter.ByUser(ratelimit.RuleAdmin, "admin"))
		}
		{
			admin.POST("/users",                    authHandler.Register)
			admin.GET("/users",                     authHandler.ListUsers)
			admin.PATCH("/users/:id",               authHandler.UpdateUser)
			admin.POST("/users/:id/reset-password", authHandler.ResetPassword)
			admin.POST("/users/:id/lock",           authHandler.LockUser)
			admin.POST("/users/:id/unlock",         authHandler.UnlockUser)
		}

		// ── /citizens ─────────────────────────────────────────
		citizens := protected.Group("/citizens")
		citizens.Use(middleware.RequireScopeMatch())
		if limiter != nil {
			// Write (POST/PATCH/DELETE): 60 req/phút per user
			// Read (GET): 200 req/phút per user — dùng RuleAPI
		}
		{
			citizens.GET("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor,
			), withRL(limiter, ratelimit.RuleAPI, "citizens_read"), citizenHandler.List)

			citizens.GET("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleCitizenSelf,
			), withRL(limiter, ratelimit.RuleAPI, "citizens_read"), citizenHandler.GetByID)

			citizens.POST("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleDataEntry,
			), withRL(limiter, ratelimit.RuleWrite, "citizens_write"), citizenHandler.Create)

			citizens.PATCH("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer,
			), withRL(limiter, ratelimit.RuleWrite, "citizens_write"), citizenHandler.Update)

			citizens.DELETE("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
			), withRL(limiter, ratelimit.RuleWrite, "citizens_write"), citizenHandler.Delete)
		}

		// ── /population — thống kê ────────────────────────────
		population := protected.Group("/population")
		{
			population.GET("/stats", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleAuditor, jwtpkg.RoleAnalyticsViewer,
			), withRL(limiter, ratelimit.RuleAPI, "stats"), citizenHandler.GetPopulationStats)

			population.GET("/stats/:province_code",
				middleware.RequireRole(
					jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
					jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
					jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleAnalyticsViewer,
				),
				middleware.RequireScopeMatch(),
				withRL(limiter, ratelimit.RuleAPI, "stats"),
				citizenHandler.GetPopulationStatByProvince,
			)
		}

		// ── /audit-logs ───────────────────────────────────────
		protected.GET("/audit-logs", middleware.RequireRole(
			jwtpkg.RoleSuperAdmin,
			jwtpkg.RoleNationalManager,
			jwtpkg.RoleAuditor,
		), withRL(limiter, ratelimit.RuleAPI, "audit"), auditHandler.List)

		// ── /households ───────────────────────────────────────
		households := protected.Group("/households")
		households.Use(middleware.RequireScopeMatch())
		{
			households.GET("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor,
			), withRL(limiter, ratelimit.RuleAPI, "households_read"), transferHandler.ListHouseholds)

			households.GET("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor,
			), withRL(limiter, ratelimit.RuleAPI, "households_read"), transferHandler.GetHousehold)

			households.POST("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer,
			), withRL(limiter, ratelimit.RuleWrite, "households_write"), transferHandler.CreateHousehold)

			households.POST("/:id/members", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer,
			), withRL(limiter, ratelimit.RuleWrite, "households_write"), transferHandler.AddHouseholdMember)
		}

		// ── /transfers ────────────────────────────────────────
		transfers := protected.Group("/transfers")
		{
			// Xem danh sách yêu cầu: các cán bộ địa bàn và auditor
			transfers.GET("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor,
			), withRL(limiter, ratelimit.RuleAPI, "transfers_read"), transferHandler.ListTransfers)

			transfers.GET("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleCitizenSelf,
			), withRL(limiter, ratelimit.RuleAPI, "transfers_read"), transferHandler.GetTransfer)

			// Tạo yêu cầu chuyển hộ khẩu
			transfers.POST("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleDataEntry,
			), withRL(limiter, ratelimit.RuleWrite, "transfers_write"), transferHandler.CreateTransfer)

			// Phê duyệt/từ chối (cán bộ địa bàn liên quan)
			transfers.POST("/:id/approve", middleware.RequireRole(
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager, jwtpkg.RoleWardOfficer,
			), withRL(limiter, ratelimit.RuleWrite, "transfers_write"), transferHandler.ApproveTransfer)

			// Force approve: chỉ super_admin, ghi audit log riêng
			transfers.POST("/:id/force-approve", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin,
			), withRL(limiter, ratelimit.RuleWrite, "transfers_write"), transferHandler.ForceApproveTransfer)
		}

		// ── Lịch sử cư trú ────────────────────────────────────
		citizens.GET("/:id/residence-history", middleware.RequireRole(
			jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
			jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
			jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleCitizenSelf,
		), withRL(limiter, ratelimit.RuleAPI, "citizens_read"), transferHandler.GetResidenceHistory)
	}

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("🚀 Population Service running on http://localhost%s", addr)
	log.Printf("🔒 Rate limiting: login=10/15min | api=200/min | write=60/min")
	log.Printf("👥 Roles: super_admin | national_manager | province_manager | district_manager | ward_officer | data_entry | auditor | analytics_viewer | citizen_self")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// withRL là helper — trả về rate limit middleware nếu limiter != nil,
// ngược lại trả về middleware no-op để router không bị nil handler.
func withRL(limiter *ratelimit.Limiter, rule ratelimit.Rule, group string) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return limiter.ByUser(rule, group)
}