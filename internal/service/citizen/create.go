package citizen

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"population-service/internal/model"
)

// Create tạo mới công dân:
//  1. Parse và validate ngày sinh
//  2. Kiểm tra trùng CCCD
//  3. Mã hóa các trường nhạy cảm
//  4. Insert DB
//  5. Ghi audit log
func (s *svc) Create(ctx context.Context, req model.CreateCitizenRequest) (*model.CitizenResponse, error) {
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("invalid date_of_birth format, expected YYYY-MM-DD")
	}

	// Dùng deterministic encryption để có thể kiểm tra trùng mà không cần decrypt toàn bộ
	encNationalID, err := s.enc.EncryptDeterministic(req.NationalID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt national_id: %w", err)
	}
	exists, err := s.citizenRepo.ExistsByNationalID(ctx, encNationalID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to check duplicate national_id: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("national_id already exists")
	}

	// Random encryption cho các trường chỉ cần đọc, không cần tìm kiếm
	encPhone, err := s.enc.Encrypt(req.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt phone_number: %w", err)
	}
	encEmail, err := s.enc.Encrypt(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt email: %w", err)
	}
	encAddress, err := s.enc.Encrypt(req.PermanentAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt permanent_address: %w", err)
	}

	isAlive := true
	if req.IsAlive != nil {
		isAlive = *req.IsAlive
	}

	citizen := &model.Citizen{
		ID:               uuid.New().String(),
		FullName:         req.FullName,
		DateOfBirth:      dob,
		Gender:           req.Gender,
		NationalID:       encNationalID,
		PhoneNumber:      encPhone,
		Email:            encEmail,
		PermanentAddress: encAddress,
		Religion:         req.Religion,
		Ethnicity:        req.Ethnicity,
		MaritalStatus:    req.MaritalStatus,
		ProvinceCode:     req.ProvinceCode,
		DistrictCode:     req.DistrictCode,
		WardCode:         req.WardCode,
		IsAlive:          isAlive,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.citizenRepo.Create(ctx, citizen); err != nil {
		return nil, fmt.Errorf("failed to create citizen: %w", err)
	}

	newSnap := buildSnapshot(req.NationalID, req.PhoneNumber, req.Email, req.PermanentAddress, citizen)
	s.writeAuditLog(ctx, citizen.ID, model.AuditActionCreate, nil, newSnap)

	return s.toResponse(citizen), nil
}