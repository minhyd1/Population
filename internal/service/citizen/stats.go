package citizen

import (
	"context"

	"population-service/internal/model"
)

// GetPopulationStats trả về thống kê dân số toàn quốc, nhóm theo tỉnh.
func (s *svc) GetPopulationStats(ctx context.Context) ([]*model.PopulationStatResponse, error) {
	stats, err := s.citizenRepo.GetPopulationStatsByProvince(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*model.PopulationStatResponse, len(stats))
	for i, stat := range stats {
		result[i] = toStatResponse(stat)
	}
	return result, nil
}

// GetPopulationStatByProvince trả về thống kê của 1 tỉnh cụ thể.
// Trả nil nếu không tìm thấy tỉnh.
func (s *svc) GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStatResponse, error) {
	stat, err := s.citizenRepo.GetPopulationStatByProvince(ctx, provinceCode)
	if err != nil {
		return nil, err
	}
	if stat == nil {
		return nil, nil
	}
	return toStatResponse(stat), nil
}

// toStatResponse map domain stat → response DTO.
// Tách ra function riêng vì cả 2 method trên đều dùng cùng mapping.
func toStatResponse(stat *model.PopulationStat) *model.PopulationStatResponse {
	return &model.PopulationStatResponse{
		ProvinceCode: stat.ProvinceCode,
		ProvinceName: stat.ProvinceName,
		Total:        stat.Total,
		Male:         stat.Male,
		Female:       stat.Female,
		Other:        stat.Other,
		Alive:        stat.Alive,
		Deceased:     stat.Deceased,
		AverageAge:   stat.AverageAge,
	}
}