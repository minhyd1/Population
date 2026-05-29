package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"population-service/internal/model"
	"population-service/internal/repository"
	"population-service/pkg/crypto"
	"population-service/pkg/middleware"

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
	auditRepo    repository.AuditRepository
	encryptor    *crypto.Encryptor
}

// NewCitizenService tạo mới citizen service
func NewCitizenService(
	citizenRepo repository.CitizenRepository,
	provinceRepo repository.ProvinceRepository,
	auditRepo repository.AuditRepository,
	encryptor *crypto.Encryptor,
) CitizenService {
	return &citizenService{
		citizenRepo:  citizenRepo,
		provinceRepo: provinceRepo,
		auditRepo:    auditRepo,
		encryptor:    encryptor,
	}
}

// Create tạo mới công dân và ghi audit log
func (s *citizenService) Create(ctx context.Context, req model.CreateCitizenRequest) (*model.CitizenResponse, error) {
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("invalid date_of_birth format, expected YYYY-MM-DD")
	}

	// Kiểm tra trùng CCCD trước khi lưu
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

	// Mã hóa deterministic để lưu DB
	encNationalID, err := s.encryptor.EncryptDeterministic(req.NationalID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt national_id: %w", err)
	}
	encPhone, err := s.encryptor.Encrypt(req.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt phone_number: %w", err)
	}
	encEmail, err := s.encryptor.Encrypt(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt email: %w", err)
	}
	encAddress, err := s.encryptor.Encrypt(req.PermanentAddress)
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

	// Ghi audit log — dùng plaintext cho snapshot (không lưu ciphertext vào log)
	newSnapshot := buildSnapshot(req.NationalID, req.PhoneNumber, req.Email, req.PermanentAddress, citizen)
	s.writeAuditLog(ctx, citizen.ID, model.AuditActionCreate, nil, newSnapshot)

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
	return s.toResponse(citizen), nil
}

// Update cập nhật thông tin công dân và ghi audit log với old/new values
func (s *citizenService) Update(ctx context.Context, id string, req model.UpdateCitizenRequest) (*model.CitizenResponse, error) {
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if citizen == nil {
		return nil, nil
	}

	// Decrypt old values để lưu vào audit log (snapshot trước khi sửa)
	oldNationalID, _ := s.encryptor.Decrypt(citizen.NationalID)
	oldPhone, _ := s.encryptor.Decrypt(citizen.PhoneNumber)
	oldEmail, _ := s.encryptor.Decrypt(citizen.Email)
	oldAddress, _ := s.encryptor.Decrypt(citizen.PermanentAddress)
	oldSnapshot := buildSnapshot(oldNationalID, oldPhone, oldEmail, oldAddress, citizen)

	// Áp dụng các thay đổi
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
		encCheck, err := s.encryptor.EncryptDeterministic(*req.NationalID)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt national_id: %w", err)
		}
		exists, err := s.citizenRepo.ExistsByNationalID(ctx, encCheck, id)
		if err != nil {
			return nil, fmt.Errorf("failed to check duplicate national_id: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("national_id already exists")
		}
		citizen.NationalID = encCheck
	}
	if req.PhoneNumber != nil {
		enc, err := s.encryptor.Encrypt(*req.PhoneNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone_number: %w", err)
		}
		citizen.PhoneNumber = enc
	}
	if req.Email != nil {
		enc, err := s.encryptor.Encrypt(*req.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt email: %w", err)
		}
		citizen.Email = enc
	}
	if req.PermanentAddress != nil {
		enc, err := s.encryptor.Encrypt(*req.PermanentAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt permanent_address: %w", err)
		}
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

	// Decrypt new values để lưu snapshot sau khi sửa
	newNationalID, _ := s.encryptor.Decrypt(citizen.NationalID)
	newPhone, _ := s.encryptor.Decrypt(citizen.PhoneNumber)
	newEmail, _ := s.encryptor.Decrypt(citizen.Email)
	newAddress, _ := s.encryptor.Decrypt(citizen.PermanentAddress)
	newSnapshot := buildSnapshot(newNationalID, newPhone, newEmail, newAddress, citizen)

	s.writeAuditLog(ctx, citizen.ID, model.AuditActionUpdate, oldSnapshot, newSnapshot)

	return s.toResponse(citizen), nil
}

// Delete xóa mềm công dân và ghi audit log
func (s *citizenService) Delete(ctx context.Context, id string) error {
	// Lấy thông tin trước khi xóa để ghi vào old_values
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.citizenRepo.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	// Ghi audit log nếu citizen tồn tại
	if citizen != nil {
		oldNationalID, _ := s.encryptor.Decrypt(citizen.NationalID)
		oldPhone, _ := s.encryptor.Decrypt(citizen.PhoneNumber)
		oldEmail, _ := s.encryptor.Decrypt(citizen.Email)
		oldAddress, _ := s.encryptor.Decrypt(citizen.PermanentAddress)
		oldSnapshot := buildSnapshot(oldNationalID, oldPhone, oldEmail, oldAddress, citizen)
		s.writeAuditLog(ctx, id, model.AuditActionDelete, oldSnapshot, nil)
	}

	return nil
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

// ──────────────────────────────────────────────────────────
// Private helpers
// ──────────────────────────────────────────────────────────

// toResponse chuyển Citizen domain → CitizenResponse DTO
func (s *citizenService) toResponse(c *model.Citizen) *model.CitizenResponse {
	return &model.CitizenResponse{
		ID:               c.ID,
		FullName:         c.FullName,
		DateOfBirth:      c.DateOfBirth,
		Gender:           c.Gender,
		NationalID:       c.NationalID,
		PhoneNumber:      c.PhoneNumber,
		Email:            c.Email,
		PermanentAddress: c.PermanentAddress,
		Religion:         c.Religion,
		Ethnicity:        c.Ethnicity,
		MaritalStatus:    c.MaritalStatus,
		ProvinceCode:     c.ProvinceCode,
		DistrictCode:     c.DistrictCode,
		WardCode:         c.WardCode,
		IsAlive:          c.IsAlive,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
		EncryptedFields:  crypto.SensitiveFields,
	}
}

// writeAuditLog ghi audit log bất đồng bộ — lỗi chỉ log ra stderr, không làm fail request
func (s *citizenService) writeAuditLog(
	ctx context.Context,
	citizenID string,
	action model.AuditAction,
	oldSnapshot *model.AuditCitizenSnapshot,
	newSnapshot *model.AuditCitizenSnapshot,
) {
	// Lấy thông tin người thực hiện từ context (được inject bởi JWTAuth middleware)
	callerID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	callerName, _ := ctx.Value(middleware.ContextKeyUsername).(string)
	callerRole, _ := ctx.Value(middleware.ContextKeyUserRole).(string)

	if callerID == "" {
		callerID = "system"
		callerName = "system"
		callerRole = "system"
	}

	var oldJSON, newJSON json.RawMessage
	if oldSnapshot != nil {
		b, err := json.Marshal(oldSnapshot)
		if err == nil {
			oldJSON = json.RawMessage(b)
		}
	}
	if newSnapshot != nil {
		b, err := json.Marshal(newSnapshot)
		if err == nil {
			newJSON = json.RawMessage(b)
		}
	}

	log := &model.AuditLog{
		ID:            uuid.New().String(),
		CitizenID:     citizenID,
		Action:        action,
		ChangedBy:     callerID,
		ChangedByName: callerName,
		ChangedByRole: callerRole,
		OldValues:     oldJSON,
		NewValues:     newJSON,
		ChangedAt:     time.Now(),
	}

	// Dùng background context để log không bị hủy nếu request context kết thúc
	bgCtx := context.Background()
	if err := s.auditRepo.Insert(bgCtx, log); err != nil {
		fmt.Printf("[AUDIT ERROR] failed to write audit log: citizen=%s action=%s err=%v\n",
			citizenID, action, err)
	}
}

// buildSnapshot tạo snapshot plaintext của citizen để lưu vào audit log.
// Các field nhạy cảm được mask để tránh lộ thông tin trong log.
func buildSnapshot(nationalID, phone, email, address string, c *model.Citizen) *model.AuditCitizenSnapshot {
	return &model.AuditCitizenSnapshot{
		FullName:         c.FullName,
		DateOfBirth:      c.DateOfBirth.Format("2006-01-02"),
		Gender:           string(c.Gender),
		NationalID:       maskNationalID(nationalID),
		PhoneNumber:      maskPhone(phone),
		Email:            maskEmail(email),
		PermanentAddress: maskAddress(address),
		Religion:         c.Religion,
		Ethnicity:        c.Ethnicity,
		MaritalStatus:    string(c.MaritalStatus),
		ProvinceCode:     c.ProvinceCode,
		DistrictCode:     c.DistrictCode,
		WardCode:         c.WardCode,
		IsAlive:          c.IsAlive,
	}
}

// maskNationalID: "123456789012" → "12**********12" (chỉ giữ 2 đầu, 2 cuối)
func maskNationalID(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// maskPhone: "0912345678" → "091*****78"
func maskPhone(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 5 {
		return strings.Repeat("*", len(s))
	}
	return s[:3] + strings.Repeat("*", len(s)-5) + s[len(s)-2:]
}

// maskEmail: "user@example.com" → "us**@example.com"
func maskEmail(s string) string {
	if s == "" {
		return ""
	}
	at := strings.Index(s, "@")
	if at < 0 {
		return strings.Repeat("*", len(s))
	}
	local := s[:at]
	domain := s[at:]
	if len(local) <= 2 {
		return strings.Repeat("*", len(local)) + domain
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + domain
}

// maskAddress: chỉ giữ 10 ký tự đầu
func maskAddress(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= 10 {
		return s
	}
	return string(runes[:10]) + "..."
}