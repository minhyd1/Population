package citizen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"population-service/internal/model"
)

// Update cập nhật thông tin công dân.
// Chỉ các field được gửi lên (non-nil) mới bị thay đổi — partial update.
// Ghi audit log với old/new snapshot để rollback hoặc kiểm tra về sau.
func (s *svc) Update(ctx context.Context, id string, req model.UpdateCitizenRequest) (*model.CitizenResponse, error) {
	citizen, err := s.citizenRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if citizen == nil {
		return nil, nil
	}

	// Snapshot TRƯỚC khi sửa — decrypt để lưu plaintext vào audit log
	oldNID, _     := s.enc.Decrypt(citizen.NationalID)
	oldPhone, _   := s.enc.Decrypt(citizen.PhoneNumber)
	oldEmail, _   := s.enc.Decrypt(citizen.Email)
	oldAddress, _ := s.enc.Decrypt(citizen.PermanentAddress)
	oldSnap := buildSnapshot(oldNID, oldPhone, oldEmail, oldAddress, citizen)

	// Áp dụng thay đổi — chỉ field non-nil
	if err := applyUpdates(ctx, s, citizen, req); err != nil {
		return nil, err
	}

	if err := s.citizenRepo.Update(ctx, citizen); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Snapshot SAU khi sửa
	newNID, _     := s.enc.Decrypt(citizen.NationalID)
	newPhone, _   := s.enc.Decrypt(citizen.PhoneNumber)
	newEmail, _   := s.enc.Decrypt(citizen.Email)
	newAddress, _ := s.enc.Decrypt(citizen.PermanentAddress)
	newSnap := buildSnapshot(newNID, newPhone, newEmail, newAddress, citizen)

	s.writeAuditLog(ctx, citizen.ID, model.AuditActionUpdate, oldSnap, newSnap)

	return s.toResponse(citizen), nil
}

// Delete xóa mềm công dân (soft delete) và ghi audit log.
func (s *svc) Delete(ctx context.Context, id string) error {
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

	if citizen != nil {
		oldNID, _     := s.enc.Decrypt(citizen.NationalID)
		oldPhone, _   := s.enc.Decrypt(citizen.PhoneNumber)
		oldEmail, _   := s.enc.Decrypt(citizen.Email)
		oldAddress, _ := s.enc.Decrypt(citizen.PermanentAddress)
		oldSnap := buildSnapshot(oldNID, oldPhone, oldEmail, oldAddress, citizen)
		s.writeAuditLog(ctx, id, model.AuditActionDelete, oldSnap, nil)
	}

	return nil
}

// applyUpdates áp dụng từng field từ request vào citizen struct.
// Tách ra hàm riêng để Update() không bị quá dài.
func applyUpdates(ctx context.Context, s *svc, c *model.Citizen, req model.UpdateCitizenRequest) error {
	if req.FullName != nil {
		c.FullName = *req.FullName
	}
	if req.DateOfBirth != nil {
		dob, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return fmt.Errorf("invalid date_of_birth format")
		}
		c.DateOfBirth = dob
	}
	if req.Gender != nil {
		c.Gender = *req.Gender
	}
	if req.NationalID != nil {
		enc, err := s.enc.EncryptDeterministic(*req.NationalID)
		if err != nil {
			return fmt.Errorf("failed to encrypt national_id: %w", err)
		}
		exists, err := s.citizenRepo.ExistsByNationalID(ctx, enc, c.ID)
		if err != nil {
			return fmt.Errorf("failed to check duplicate national_id: %w", err)
		}
		if exists {
			return fmt.Errorf("national_id already exists")
		}
		c.NationalID = enc
	}
	if req.PhoneNumber != nil {
		enc, err := s.enc.Encrypt(*req.PhoneNumber)
		if err != nil {
			return fmt.Errorf("failed to encrypt phone_number: %w", err)
		}
		c.PhoneNumber = enc
	}
	if req.Email != nil {
		enc, err := s.enc.Encrypt(*req.Email)
		if err != nil {
			return fmt.Errorf("failed to encrypt email: %w", err)
		}
		c.Email = enc
	}
	if req.PermanentAddress != nil {
		enc, err := s.enc.Encrypt(*req.PermanentAddress)
		if err != nil {
			return fmt.Errorf("failed to encrypt permanent_address: %w", err)
		}
		c.PermanentAddress = enc
	}
	if req.Religion != nil {
		c.Religion = *req.Religion
	}
	if req.Ethnicity != nil {
		c.Ethnicity = *req.Ethnicity
	}
	if req.MaritalStatus != nil {
		c.MaritalStatus = *req.MaritalStatus
	}
	if req.ProvinceCode != nil {
		c.ProvinceCode = *req.ProvinceCode
	}
	if req.DistrictCode != nil {
		c.DistrictCode = *req.DistrictCode
	}
	if req.WardCode != nil {
		c.WardCode = *req.WardCode
	}
	if req.IsAlive != nil {
		c.IsAlive = *req.IsAlive
	}
	return nil
}