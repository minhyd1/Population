package service

import (
	"context"

	"population-service/internal/model"
	"population-service/internal/repository"
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

// GetLogs tra cứu audit log với filter và phân trang
func (s *auditService) GetLogs(ctx context.Context, filter model.ListAuditLogFilter) (*model.AuditLogListResponse, error) {
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