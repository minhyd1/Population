package citizen

import (
	"context"

	"population-service/internal/model"
)

// GetByID lấy thông tin chi tiết 1 công dân theo UUID.
// Trả nil nếu không tìm thấy (handler sẽ trả 404).
func (s *svc) GetByID(ctx context.Context, id string) (*model.CitizenResponse, error) {
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if citizen == nil {
		return nil, nil
	}
	return s.toResponse(citizen), nil
}

// List trả về danh sách công dân phân trang, có filter.
// Repository đã xử lý WHERE clause — service chỉ map kết quả sang DTO.
func (s *svc) List(ctx context.Context, filter model.ListCitizenFilter) (*model.CitizenListResponse, error) {
	citizens, total, err := s.citizenRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]model.CitizenResponse, len(citizens))
	for i, c := range citizens {
		responses[i] = *s.toResponse(c)
	}

	return &model.CitizenListResponse{
		Data:     responses,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}