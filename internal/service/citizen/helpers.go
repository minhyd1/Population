package citizen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"population-service/internal/model"
	"population-service/pkg/middleware"
)

// ── Response mapping ──────────────────────────────────────

// toResponse chuyển Citizen domain model → CitizenResponse DTO.
// Decrypt các trường nhạy cảm tại đây — client nhận plaintext qua HTTPS.
func (s *svc) toResponse(c *model.Citizen) *model.CitizenResponse {
	nationalID, _      := s.enc.Decrypt(c.NationalID)
	phoneNumber, _     := s.enc.Decrypt(c.PhoneNumber)
	email, _           := s.enc.Decrypt(c.Email)
	permanentAddress, _ := s.enc.Decrypt(c.PermanentAddress)

	return &model.CitizenResponse{
		ID:               c.ID,
		FullName:         c.FullName,
		DateOfBirth:      c.DateOfBirth,
		Gender:           c.Gender,
		NationalID:       nationalID,
		PhoneNumber:      phoneNumber,
		Email:            email,
		PermanentAddress: permanentAddress,
		Religion:         c.Religion,
		Ethnicity:        c.Ethnicity,
		MaritalStatus:    c.MaritalStatus,
		ProvinceCode:     c.ProvinceCode,
		DistrictCode:     c.DistrictCode,
		WardCode:         c.WardCode,
		IsAlive:          c.IsAlive,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

// ── Audit log ─────────────────────────────────────────────

// writeAuditLog ghi audit log bất đồng bộ.
// Lỗi chỉ in ra stderr — không làm fail request chính.
func (s *svc) writeAuditLog(
	ctx context.Context,
	citizenID string,
	action model.AuditAction,
	oldSnap *model.AuditCitizenSnapshot,
	newSnap *model.AuditCitizenSnapshot,
) {
	callerID, _   := ctx.Value(middleware.ContextKeyUserID).(string)
	callerName, _ := ctx.Value(middleware.ContextKeyUsername).(string)
	callerRole, _ := ctx.Value(middleware.ContextKeyUserRole).(string)

	if callerID == "" {
		callerID, callerName, callerRole = "system", "system", "system"
	}

	var oldJSON, newJSON json.RawMessage
	if oldSnap != nil {
		if b, err := json.Marshal(oldSnap); err == nil {
			oldJSON = b
		}
	}
	if newSnap != nil {
		if b, err := json.Marshal(newSnap); err == nil {
			newJSON = b
		}
	}

	entry := &model.AuditLog{
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

	// Background context: audit log không bị hủy khi request context kết thúc
	if err := s.auditRepo.Insert(context.Background(), entry); err != nil {
		fmt.Printf("[AUDIT ERROR] citizen=%s action=%s err=%v\n", citizenID, action, err)
	}
}

// ── Snapshot builder ──────────────────────────────────────

// buildSnapshot tạo plaintext snapshot của citizen để lưu vào audit log.
// Dữ liệu nhạy cảm được mask — tránh lưu số CCCD / SĐT đầy đủ vào log.
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

// ── Mask helpers ──────────────────────────────────────────
// Mỗi hàm mask giữ lại đủ thông tin để nhận dạng nhưng không lộ toàn bộ.

// maskNationalID: "123456789012" → "12********12"
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
	if len(local) <= 2 {
		return strings.Repeat("*", len(local)) + s[at:]
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + s[at:]
}

// maskAddress: giữ 10 ký tự đầu
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