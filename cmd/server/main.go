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
	"population-service/pkg/crypto"
	jwtpkg "population-service/pkg/jwt"
	"population-service/pkg/middleware"
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

	citizenRepo  := repository.NewCitizenRepository(db)
	provinceRepo := repository.NewProvinceRepository(db)
	userRepo     := repository.NewUserRepository(db)

	citizenSvc := service.NewCitizenService(citizenRepo, provinceRepo, enc)
	authSvc    := service.NewAuthService(userRepo, jwtManager)

	citizenHandler := handler.NewCitizenHandler(citizenSvc, enc)
	authHandler    := handler.NewAuthHandler(authSvc)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "population-service"})
	})

	v1 := r.Group("/api/v1")

	// ── Public ────────────────────────────────────────────────
	auth := v1.Group("/auth")
	{
		auth.POST("/login",   authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// ── Protected — tất cả route bên dưới cần JWT ────────────
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(jwtManager))
	{
		// Mọi user đã login đều dùng được
		authGroup := protected.Group("/auth")
		{
			authGroup.GET("/me",             authHandler.Me)
			authGroup.POST("/logout",        authHandler.Logout)
			authGroup.POST("/logout-all",    authHandler.LogoutAll)
			authGroup.POST("/change-password", authHandler.ChangePassword)
		}

		// ── /admin — chỉ super_admin ─────────────────────────
		admin := protected.Group("/admin")
		admin.Use(middleware.RequireRole(jwtpkg.RoleSuperAdmin))
		{
			admin.POST("/users",                  authHandler.Register)
			admin.GET("/users",                   authHandler.ListUsers)
			admin.PATCH("/users/:id",             authHandler.UpdateUser)
			admin.POST("/users/:id/reset-password", authHandler.ResetPassword)
			admin.POST("/users/:id/lock",         authHandler.LockUser)
			admin.POST("/users/:id/unlock",       authHandler.UnlockUser)
			admin.GET("/encryption/meta",         citizenHandler.GetEncryptionMeta)
		}

		// ── /citizens ─────────────────────────────────────────
		citizens := protected.Group("/citizens")
		citizens.Use(middleware.RequireScopeMatch()) // lọc địa bàn tự động
		{
			// Xem danh sách: từ ward_officer trở lên
			citizens.GET("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor,
			), citizenHandler.List)

			// Xem chi tiết: tương tự, thêm citizen_self (chỉ xem của mình — logic trong handler)
			citizens.GET("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleCitizenSelf,
			), citizenHandler.GetByID)

			// Tạo mới: từ ward_officer trở lên (data_entry cũng được)
			citizens.POST("", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleDataEntry,
			), citizenHandler.Create)

			// Cập nhật: từ ward_officer trở lên, không cho data_entry (cần duyệt)
			citizens.PATCH("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer,
			), citizenHandler.Update)

			// Xóa: chỉ super_admin và national_manager
			citizens.DELETE("/:id", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
			), citizenHandler.Delete)
		}

		// ── /population — thống kê ───────────────────────────
		population := protected.Group("/population")
		{
			// Thống kê toàn quốc: super_admin, national_manager, auditor, analytics_viewer
			population.GET("/stats", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleAuditor, jwtpkg.RoleAnalyticsViewer,
			), citizenHandler.GetPopulationStats)

			// Thống kê theo tỉnh: thêm province/district/ward manager
			population.GET("/stats/:province_code",
				middleware.RequireRole(
					jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
					jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
					jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleAnalyticsViewer,
				),
				middleware.RequireScopeMatch(),
				citizenHandler.GetPopulationStatByProvince,
			)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("🚀 Population Service running on http://localhost%s", addr)
	log.Printf("👥 Roles: super_admin | national_manager | province_manager | district_manager | ward_officer | data_entry | auditor | analytics_viewer | citizen_self")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}