package main

import (
	"fmt"
	"log"
)

// main chỉ còn 3 việc: load config + connect DB, wiring container, chạy server.
// Toàn bộ DI nằm ở container.go, toàn bộ route nằm ở router.go.
func main() {
	cfg := LoadConfig()

	db := ConnectDB(cfg)
	defer db.Close()

	c, err := NewContainer(cfg, db)
	if err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}
	defer c.Close()

	r := NewRouter(c)

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	log.Printf("🚀 Population Service running on http://localhost%s", addr)
	log.Printf("🔒 Rate limiting: login=10/15min | api=200/min | write=60/min")
	log.Printf("👥 Roles: super_admin | national_manager | province_manager | district_manager | ward_officer | data_entry | auditor | analytics_viewer | citizen_self")
	log.Printf("🏛️  Admin units tree + user_assignments + permissions enabled")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
