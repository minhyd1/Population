package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	jwtpkg "population-service/pkg/jwt"
	"population-service/pkg/middleware"
	"population-service/pkg/ratelimit"
)

// withRL trả về rate limit middleware nếu limiter != nil,
// ngược lại trả về middleware no-op để router không bị nil handler.
func withRL(limiter *ratelimit.Limiter, rule ratelimit.Rule, group string) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return limiter.ByUser(rule, group)
}

// NewRouter build toàn bộ route của service từ Container đã wiring sẵn.
// Tách khỏi main.go để main.go chỉ còn việc chạy server.
func NewRouter(c *Container) *gin.Engine {
	if c.Config.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":        "ok",
			"service":       "population-service",
			"redis_enabled": c.RedisClient != nil,
		})
	})

	limiter := c.Limiter
	jwtManager := c.JWTManager
	redisClient := c.RedisClient
	citizenHandler := c.CitizenHandler
	authHandler := c.AuthHandler
	auditHandler := c.AuditHandler
	transferHandler := c.TransferHandler
	assignmentHandler := c.AssignmentHandler

	v1 := r.Group("/api/v1")

	// ── Public ────────────────────────────────────────────────
	auth := v1.Group("/auth")
	{
		// Login: 10 request / 15 phút theo IP
		auth.POST("/login", withRL(limiter, ratelimit.RuleLogin, "login"), authHandler.Login)
		// Refresh: 30 request / 15 phút theo IP
		auth.POST("/refresh", withRL(limiter, ratelimit.RuleRefresh, "refresh"), authHandler.Refresh)
	}

	// ── Protected — cần JWT ───────────────────────────────────
	// JWTAuth nhận redisClient để kiểm tra token blacklist
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(jwtManager, redisClient))
	{
		authGroup := protected.Group("/auth")
		// Rate limit chung cho API: 200 req/phút theo userID
		authGroup.Use(withRL(limiter, ratelimit.RuleAPI, "auth"))
		{
			authGroup.GET("/me", authHandler.Me)
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.POST("/logout-all", authHandler.LogoutAll)
			authGroup.POST("/change-password", authHandler.ChangePassword)
		}

		// ── /admin — chỉ super_admin ──────────────────────────
		admin := protected.Group("/admin")
		admin.Use(middleware.RequireRole(jwtpkg.RoleSuperAdmin))
		admin.Use(withRL(limiter, ratelimit.RuleAdmin, "admin"))
		{
			admin.POST("/users", authHandler.Register)
			admin.GET("/users", authHandler.ListUsers)
			admin.PATCH("/users/:id", authHandler.UpdateUser)
			admin.POST("/users/:id/reset-password", authHandler.ResetPassword)
			admin.POST("/users/:id/lock", authHandler.LockUser)
			admin.POST("/users/:id/unlock", authHandler.UnlockUser)

			// ── User Assignments (điểm 1: tách scope khỏi users) ────
			admin.POST("/assignments", assignmentHandler.AssignUser)
			admin.POST("/assignments/:id/end", assignmentHandler.EndAssignment)
			admin.GET("/users/:id/assignments", assignmentHandler.GetUserAssignments)
			// GET /admin/units/:code/officers?at=2024-01-01
			admin.GET("/units/:code/officers", assignmentHandler.GetUnitOfficers)
		}

		// ── /citizens ─────────────────────────────────────────
		citizens := protected.Group("/citizens")
		citizens.Use(middleware.RequireScopeMatch())
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

			// ── Lịch sử cư trú ────────────────────────────────
			citizens.GET("/:id/residence-history", middleware.RequireRole(
				jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager,
				jwtpkg.RoleProvinceManager, jwtpkg.RoleDistrictManager,
				jwtpkg.RoleWardOfficer, jwtpkg.RoleAuditor, jwtpkg.RoleCitizenSelf,
			), withRL(limiter, ratelimit.RuleAPI, "citizens_read"), transferHandler.GetResidenceHistory)
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
	}

	return r
}
