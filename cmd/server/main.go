// @title           Population Service API
// @version         1.0
// @description     API quản lý dân số Việt Nam. Các trường nhạy cảm được mã hóa AES-256-GCM.
// @host            localhost:8080
// @BasePath        /api/v1
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"population-service/internal/handler"
	"population-service/internal/repository"
	"population-service/internal/service"
	"population-service/pkg/crypto"
	"population-service/pkg/middleware"
)

func main() {
	// 1. Load config
	cfg := LoadConfig()

	// 2. Connect DB
	db := ConnectDB(cfg)
	defer db.Close()

	// 3. Init encryptor
	enc, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to init encryptor: %v", err)
	}

	// 4. Wire up layers
	citizenRepo  := repository.NewCitizenRepository(db)
	provinceRepo := repository.NewProvinceRepository(db)
	citizenSvc   := service.NewCitizenService(citizenRepo, provinceRepo, enc)
	citizenHandler := handler.NewCitizenHandler(citizenSvc, enc)

	// 5. Setup router
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "population-service",
		})
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		citizens := v1.Group("/citizens")
		{
			citizens.POST("",      citizenHandler.Create)
			citizens.GET("",       citizenHandler.List)
			citizens.GET("/:id",   citizenHandler.GetByID)
			citizens.PATCH("/:id", citizenHandler.Update)
			citizens.DELETE("/:id",citizenHandler.Delete)
		}

		population := v1.Group("/population")
		{
			population.GET("/stats",                citizenHandler.GetPopulationStats)
			population.GET("/stats/:province_code", citizenHandler.GetPopulationStatByProvince)
		}

		encryption := v1.Group("/encryption")
		{
			encryption.GET("/meta", citizenHandler.GetEncryptionMeta)
		}
	}

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("🚀 Population Service running on http://localhost%s", addr)
	log.Printf("📋 Test: http://localhost%s/health", addr)
	log.Printf("📊 Stats: http://localhost%s/api/v1/population/stats", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
