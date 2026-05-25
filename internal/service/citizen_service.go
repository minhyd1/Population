package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"population-service/internal/model"
	"population-service/internal/repository"
	"population-service/pkg/crypto"

	"github.com/google/uuid"
)

// CitizenService định nghĩa business logic
type CitizenService interface {
	Create(ctx context.Context, req model.CreateCitizenRequest) (*model.CitizenResponse, error)
	GetByID(ctx context.Context, id string) (*model.CitizenResponse, error)
	Update(ctx context.Context, id string, req model.UpdateCitizenRequest) (*model.CitizenResponse, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter model.ListCitizenFilter) (*model.CitizenListResponse, error)
	GetPopulationStats(ctx context.Context) ([]*model.PopulationStatResponse, error)
	GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStatResponse, error)
}

type citizenService struct {
	citizenRepo  repository.CitizenRepository
	provinceRepo repository.ProvinceRepository
	encryptor    *crypto.Encryptor
}

// NewCitizenService tạo mới citizen service
func NewCitizenService(
	citizenRepo repository.CitizenRepository,
	provinceRepo repository.ProvinceRepository,
	encryptor *crypto.Encryptor,
) CitizenService {
	return &citizenService{
		citizenRepo:  citizenRepo,
		provinceRepo: provinceRepo,
		encryptor:    encryptor,
	}
}

// Create tạo mới công dân:
// 1. Mã hóa các trường nhạy cảm trước khi lưu DB
// 2. Lưu DB
// 3. Trả về response với các trường đã mã hóa (client decrypt)
func (s *citizenService) Create(ctx context.Context, req model.CreateCitizenRequest) (*model.CitizenResponse, error) {
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("invalid date_of_birth format, expected YYYY-MM-DD")
	}

	encNationalIDCheck, err := s.encryptor.EncryptDeterministic(req.NationalID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt national_id: %w", err)
	}
	exists, err := s.citizenRepo.ExistsByNationalID(ctx, encNationalIDCheck, "")
	if err != nil {
		return nil, fmt.Errorf("failed to check duplicate national_id: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("national_id already exists")
	}
	// Mã hóa các trường nhạy cảm để lưu DB
	encNationalID, err := s.encryptor.EncryptDeterministic(req.NationalID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt national_id: %w", err)
	}
	encPhone, _ := s.encryptor.Encrypt(req.PhoneNumber)
	encEmail, _ := s.encryptor.Encrypt(req.Email)
	encAddress, _ := s.encryptor.Encrypt(req.PermanentAddress)

	isAlive := true
	if req.IsAlive != nil {
		isAlive = *req.IsAlive
	}

	citizen := &model.Citizen{
		ID:               uuid.New().String(),
		FullName:         req.FullName,
		DateOfBirth:      dob,
		Gender:           req.Gender,
		NationalID:       encNationalID, // encrypted in DB
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

	// Trả về response với encrypted fields (client sẽ decrypt)
	return s.toResponse(citizen), nil
}

func (s *citizenService) GetByID(ctx context.Context, id string) (*model.CitizenResponse, error) {
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if citizen == nil {
		return nil, nil
	}
	// citizen.NationalID, PhoneNumber, Email, PermanentAddress đang là encrypted từ DB
	// Trả thẳng về response (vẫn encrypted → client decrypt)
	return s.toResponse(citizen), nil
}

func (s *citizenService) Update(ctx context.Context, id string, req model.UpdateCitizenRequest) (*model.CitizenResponse, error) {
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if citizen == nil {
		return nil, nil
	}

	// Update từng field nếu có
	if req.FullName != nil {
		citizen.FullName = *req.FullName
	}
	if req.DateOfBirth != nil {
		dob, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return nil, fmt.Errorf("invalid date_of_birth format")
		}
		citizen.DateOfBirth = dob
	}
	if req.Gender != nil {
		citizen.Gender = *req.Gender
	}
	if req.NationalID != nil {
		enc, err := s.encryptor.EncryptDeterministic(*req.NationalID)
		if err != nil {
			return nil, err
		}
		citizen.NationalID = enc
	}
	if req.PhoneNumber != nil {
		enc, _ := s.encryptor.Encrypt(*req.PhoneNumber)
		citizen.PhoneNumber = enc
	}
	if req.Email != nil {
		enc, _ := s.encryptor.Encrypt(*req.Email)
		citizen.Email = enc
	}
	if req.PermanentAddress != nil {
		enc, _ := s.encryptor.Encrypt(*req.PermanentAddress)
		citizen.PermanentAddress = enc
	}
	if req.Religion != nil {
		citizen.Religion = *req.Religion
	}
	if req.Ethnicity != nil {
		citizen.Ethnicity = *req.Ethnicity
	}
	if req.MaritalStatus != nil {
		citizen.MaritalStatus = *req.MaritalStatus
	}
	if req.ProvinceCode != nil {
		citizen.ProvinceCode = *req.ProvinceCode
	}
	if req.DistrictCode != nil {
		citizen.DistrictCode = *req.DistrictCode
	}
	if req.WardCode != nil {
		citizen.WardCode = *req.WardCode
	}
	if req.IsAlive != nil {
		citizen.IsAlive = *req.IsAlive
	}

	if err := s.citizenRepo.Update(ctx, citizen); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return s.toResponse(citizen), nil
}

func (s *citizenService) Delete(ctx context.Context, id string) error {
	err := s.citizenRepo.SoftDelete(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (s *citizenService) List(ctx context.Context, filter model.ListCitizenFilter) (*model.CitizenListResponse, error) {
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

func (s *citizenService) GetPopulationStats(ctx context.Context) ([]*model.PopulationStatResponse, error) {
	stats, err := s.citizenRepo.GetPopulationStatsByProvince(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*model.PopulationStatResponse, len(stats))
	for i, stat := range stats {
		result[i] = &model.PopulationStatResponse{
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
	return result, nil
}

func (s *citizenService) GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStatResponse, error) {
	stat, err := s.citizenRepo.GetPopulationStatByProvince(ctx, provinceCode)
	if err != nil {
		return nil, err
	}
	if stat == nil {
		return nil, nil
	}
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
	}, nil
}

// toResponse chuyển Citizen domain → CitizenResponse DTO.
// Các trường nhạy cảm đã được mã hóa từ DB, giữ nguyên (encrypted) trong response.
// Frontend nhận encrypted ciphertext và tự decrypt bằng shared AES key.
func (s *citizenService) toResponse(c *model.Citizen) *model.CitizenResponse {
	return &model.CitizenResponse{
		ID:               c.ID,
		FullName:         c.FullName,
		DateOfBirth:      c.DateOfBirth,
		Gender:           c.Gender,
		NationalID:       c.NationalID,       // encrypted ciphertext
		PhoneNumber:      c.PhoneNumber,      // encrypted ciphertext
		Email:            c.Email,            // encrypted ciphertext
		PermanentAddress: c.PermanentAddress, // encrypted ciphertext
		Religion:         c.Religion,
		Ethnicity:        c.Ethnicity,
		MaritalStatus:    c.MaritalStatus,
		ProvinceCode:     c.ProvinceCode,
		DistrictCode:     c.DistrictCode,
		WardCode:         c.WardCode,
		IsAlive:          c.IsAlive,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
		EncryptedFields:  crypto.SensitiveFields, // frontend biết field nào cần decrypt
	}
}
