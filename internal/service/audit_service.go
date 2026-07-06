package service

import (
	"context"

	jwtpkg "population-service/pkg/jwt"
	"population-service/internal/model"
	"population-service/internal/repository"
	"population-service/pkg/middleware"
)

// AuditService định nghĩa business logic cho audit log
type AuditService interface {
	GetLogs(ctx context.Context, filter model.ListAuditLogFilter) (*model.AuditLogListResponse, error)
}

type auditService struct {
	auditRepo repository.AuditRepository
}

// NewAuditService tạo mới audit service
func NewAuditService(auditRepo repository.AuditRepository) AuditService {
	return &auditService{auditRepo: auditRepo}
}

// GetLogs tra cứu audit log với filter và phân trang.
// Vấn đề 2: tự động inject UnitCode từ JWT claims để enforce visibility.
// - super_admin / national_manager / auditor: xem tất cả (UnitCode = "")
// - ward_officer / district_manager / province_manager: chỉ xem log có visibility
//   tương ứng với unit_code của mình
func (s *auditService) GetLogs(ctx context.Context, filter model.ListAuditLogFilter) (*model.AuditLogListResponse, error) {
	// Inject UnitCode từ JWT context nếu user không phải admin/auditor
	unitCode, _ := ctx.Value(middleware.ContextKeyWardCode).(string)
	districtCode, _ := ctx.Value(middleware.ContextKeyDistrictCode).(string)
	provinceCode, _ := ctx.Value(middleware.ContextKeyProvinceCode).(string)
	role, _ := ctx.Value(middleware.ContextKeyUserRole).(string)

	switch jwtpkg.Role(role) {
	case jwtpkg.RoleSuperAdmin, jwtpkg.RoleNationalManager, jwtpkg.RoleAuditor:
		// Xem tất cả — không lọc theo visibility
		filter.UnitCode = ""
	case jwtpkg.RoleWardOfficer:
		filter.UnitCode = unitCode
	case jwtpkg.RoleDistrictManager:
		filter.UnitCode = districtCode
	case jwtpkg.RoleProvinceManager:
		filter.UnitCode = provinceCode
	default:
		// Các role khác không có quyền xem audit log
		// Handler sẽ guard bằng RequireRole nên đây là fallback
		filter.UnitCode = unitCode
	}

	logs, total, err := s.auditRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	data := make([]model.AuditLogResponse, len(logs))
	for i, l := range logs {
		data[i] = toAuditLogResponse(l)
	}

	return &model.AuditLogListResponse{
		Data:     data,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func toAuditLogResponse(l *model.AuditLog) model.AuditLogResponse {
	return model.AuditLogResponse{
		ID:            l.ID,
		CitizenID:     l.CitizenID,
		Action:        l.Action,
		ChangedBy:     l.ChangedBy,
		ChangedByName: l.ChangedByName,
		ChangedByRole: l.ChangedByRole,
		OldValues:     l.OldValues,
		NewValues:     l.NewValues,
		ChangedAt:     l.ChangedAt,
	}
}
