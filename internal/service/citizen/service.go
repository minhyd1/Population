// Package citizen chứa toàn bộ business logic liên quan đến quản lý công dân.
// Được tách từ citizen_service.go (≈489 lines) thành 5 file nhỏ theo trách nhiệm:
//
//	service.go  — interface, struct, constructor (file này)
//	create.go   — tạo mới công dân
//	update.go   — cập nhật + xóa công dân
//	query.go    — đọc danh sách, đọc chi tiết
//	stats.go    — thống kê dân số
//	helpers.go  — toResponse, writeAuditLog, mask functions
package citizen

import (
	"context"

	"population-service/internal/model"
	"population-service/internal/repository"
	"population-service/pkg/crypto"
)

// Service định nghĩa toàn bộ business logic của module citizen.
// Handler chỉ phụ thuộc vào interface này, không biết implementation.
type Service interface {
	Create(ctx context.Context, req model.CreateCitizenRequest) (*model.CitizenResponse, error)
	GetByID(ctx context.Context, id string) (*model.CitizenResponse, error)
	Update(ctx context.Context, id string, req model.UpdateCitizenRequest) (*model.CitizenResponse, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter model.ListCitizenFilter) (*model.CitizenListResponse, error)
	GetPopulationStats(ctx context.Context) ([]*model.PopulationStatResponse, error)
	GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStatResponse, error)
}

// svc là implementation của Service.
// Tất cả dependencies được inject qua constructor — không dùng global variable.
type svc struct {
	citizenRepo  repository.CitizenRepository
	provinceRepo repository.ProvinceRepository
	auditRepo    repository.AuditRepository
	enc          *crypto.Encryptor
}

// New tạo Service mới với đầy đủ dependencies.
// Đặt tên New thay vì NewCitizenService vì đây đã nằm trong package citizen.
func New(
	citizenRepo repository.CitizenRepository,
	provinceRepo repository.ProvinceRepository,
	auditRepo repository.AuditRepository,
	enc *crypto.Encryptor,
) Service {
	return &svc{
		citizenRepo:  citizenRepo,
		provinceRepo: provinceRepo,
		auditRepo:    auditRepo,
		enc:          enc,
	}
}