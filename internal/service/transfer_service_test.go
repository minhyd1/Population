package service

import (
	"context"
	"testing"

	"population-service/pkg/middleware"
)

// TestClaimsFromCtx_IncludesGeographicScope là test hồi quy (regression test)
// cho một bug đã từng tồn tại: claimsFromCtx() chỉ đọc UserID/Role/Username
// từ context mà QUÊN đọc WardCode/DistrictCode/ProvinceCode — khiến các role
// bị giới hạn theo địa bàn (ward_officer, district_manager, province_manager)
// luôn có scope rỗng và mọi scope-check lặng lẽ fail.
//
// Code hiện tại đã đọc đủ 3 field này (xem transfer_service.go), test này chỉ
// để đảm bảo không ai vô tình xóa lại 3 dòng đó trong tương lai.
func TestClaimsFromCtx_IncludesGeographicScope(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, "user-1")
	ctx = context.WithValue(ctx, middleware.ContextKeyUsername, "an.nguyen")
	ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, "ward_officer")
	ctx = context.WithValue(ctx, middleware.ContextKeyWardCode, "HN-BD")
	ctx = context.WithValue(ctx, middleware.ContextKeyDistrictCode, "HN-01")
	ctx = context.WithValue(ctx, middleware.ContextKeyProvinceCode, "HN")

	claims := claimsFromCtx(ctx)
	if claims == nil {
		t.Fatal("expected claims, got nil")
	}
	if claims.WardCode != "HN-BD" {
		t.Errorf("expected WardCode=HN-BD, got %q", claims.WardCode)
	}
	if claims.DistrictCode != "HN-01" {
		t.Errorf("expected DistrictCode=HN-01, got %q", claims.DistrictCode)
	}
	if claims.ProvinceCode != "HN" {
		t.Errorf("expected ProvinceCode=HN, got %q", claims.ProvinceCode)
	}
}

func TestClaimsFromCtx_NoUserID_ReturnsNil(t *testing.T) {
	claims := claimsFromCtx(context.Background())
	if claims != nil {
		t.Errorf("expected nil claims when no user_id in context, got %+v", claims)
	}
}
