package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"population-service/internal/model"
	"population-service/internal/repository"
)

// AssignmentService quản lý phân công cán bộ vào đơn vị hành chính.
// Đây là nguồn sự thật chính thức cho "ai phụ trách đâu" thay vì province/district/ward_code cứng.
type AssignmentService interface {
	// AssignUser phân công user vào đơn vị (tạo user_assignment mới)
	AssignUser(ctx context.Context, req model.AssignUserRequest, createdByUserID string) (*model.UserAssignment, error)
	// EndAssignment kết thúc phân công (điều chuyển / nghỉ việc)
	EndAssignment(ctx context.Context, assignmentID string, endDate time.Time, note *string) error
	// GetActiveAssignments trả về các phân công đang active của user
	GetActiveAssignments(ctx context.Context, userID string) ([]model.UserAssignmentResponse, error)
	// GetHistory trả về toàn bộ lịch sử phân công của user
	GetHistory(ctx context.Context, userID string) ([]model.UserAssignmentResponse, error)
	// GetActiveOfficersByUnit trả về cán bộ đang phụ trách đơn vị
	GetActiveOfficersByUnit(ctx context.Context, unitCode string) ([]model.UserAssignmentResponse, error)
	// GetOfficerAtTime "ai phụ trách unit_code vào ngày X?" — audit
	GetOfficerAtTime(ctx context.Context, unitCode string, at time.Time) ([]model.UserAssignmentResponse, error)
	// HasPermission kiểm tra role có quyền permissionCode không
	HasPermission(ctx context.Context, role, permissionCode string) (bool, error)
	// GetActiveUnitCodes lấy danh sách unit_code user đang phụ trách
	GetActiveUnitCodes(ctx context.Context, userID string) ([]string, error)
}

type assignmentService struct {
	adminRepo repository.AdminUnitRepository
	userRepo  repository.UserRepository
}

func NewAssignmentService(adminRepo repository.AdminUnitRepository, userRepo repository.UserRepository) AssignmentService {
	return &assignmentService{adminRepo: adminRepo, userRepo: userRepo}
}

func (s *assignmentService) AssignUser(ctx context.Context, req model.AssignUserRequest, createdByUserID string) (*model.UserAssignment, error) {
	// Kiểm tra unit_code tồn tại
	unit, err := s.adminRepo.FindUnitByCode(ctx, req.UnitCode)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, errors.New("unit_code không tồn tại trong administrative_units")
	}

	// Kiểm tra user tồn tại
	user, err := s.userRepo.FindByID(ctx, req.UserID)
	if err != nil || user == nil {
		return nil, errors.New("user không tồn tại")
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, errors.New("start_date không hợp lệ, định dạng: 2006-01-02")
	}

	createdBy := createdByUserID
	assignment := &model.UserAssignment{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		UnitCode:  req.UnitCode,
		Role:      string(user.Role),
		StartDate: startDate,
		Note:      req.Note,
		CreatedBy: &createdBy,
	}

	if err := s.adminRepo.CreateAssignment(ctx, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

func (s *assignmentService) EndAssignment(ctx context.Context, assignmentID string, endDate time.Time, note *string) error {
	return s.adminRepo.EndAssignment(ctx, assignmentID, endDate, note)
}

func (s *assignmentService) GetActiveAssignments(ctx context.Context, userID string) ([]model.UserAssignmentResponse, error) {
	list, err := s.adminRepo.GetActiveAssignments(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toResponses(ctx, list), nil
}

func (s *assignmentService) GetHistory(ctx context.Context, userID string) ([]model.UserAssignmentResponse, error) {
	list, err := s.adminRepo.GetAssignmentHistory(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toResponses(ctx, list), nil
}

func (s *assignmentService) GetActiveOfficersByUnit(ctx context.Context, unitCode string) ([]model.UserAssignmentResponse, error) {
	list, err := s.adminRepo.GetActiveOfficersByUnit(ctx, unitCode)
	if err != nil {
		return nil, err
	}
	return s.toResponses(ctx, list), nil
}

func (s *assignmentService) GetOfficerAtTime(ctx context.Context, unitCode string, at time.Time) ([]model.UserAssignmentResponse, error) {
	list, err := s.adminRepo.GetOfficerAtTime(ctx, unitCode, at)
	if err != nil {
		return nil, err
	}
	return s.toResponses(ctx, list), nil
}

func (s *assignmentService) HasPermission(ctx context.Context, role, permissionCode string) (bool, error) {
	return s.adminRepo.HasPermission(ctx, role, permissionCode)
}

func (s *assignmentService) GetActiveUnitCodes(ctx context.Context, userID string) ([]string, error) {
	assignments, err := s.adminRepo.GetActiveAssignments(ctx, userID)
	if err != nil {
		return nil, err
	}
	return model.GetActiveUnitCodes(assignments), nil
}

// toResponses chuyển đổi sang DTO, bổ sung UnitName và Username nếu có
func (s *assignmentService) toResponses(ctx context.Context, list []model.UserAssignment) []model.UserAssignmentResponse {
	result := make([]model.UserAssignmentResponse, 0, len(list))
	for _, a := range list {
		r := model.UserAssignmentResponse{
			ID:        a.ID,
			UserID:    a.UserID,
			UnitCode:  a.UnitCode,
			Role:      a.Role,
			StartDate: a.StartDate,
			EndDate:   a.EndDate,
			Note:      a.Note,
			IsActive:  a.IsActive(),
		}

		// Bổ sung UnitName
		if unit, err := s.adminRepo.FindUnitByCode(ctx, a.UnitCode); err == nil && unit != nil {
			r.UnitName = unit.Name
		}

		// Bổ sung Username
		if user, err := s.userRepo.FindByID(ctx, a.UserID); err == nil && user != nil {
			r.Username = user.Username
		}

		result = append(result, r)
	}
	return result
}
