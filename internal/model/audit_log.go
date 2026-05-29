package model

import (
	"encoding/json"
	"time"
)

// AuditAction là loại thao tác được ghi log
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
)

// AuditLog là bản ghi lịch sử thay đổi thông tin công dân
type AuditLog struct {
	ID             string          `db:"id"`
	CitizenID      string          `db:"citizen_id"`
	Action         AuditAction     `db:"action"`
	ChangedBy      string          `db:"changed_by"`      // user_id
	ChangedByName  string          `db:"changed_by_name"` // username
	ChangedByRole  string          `db:"changed_by_role"`
	OldValues      json.RawMessage `db:"old_values"`  // NULL với create
	NewValues      json.RawMessage `db:"new_values"`  // NULL với delete
	ChangedAt      time.Time       `db:"changed_at"`
}

// AuditCitizenSnapshot là snapshot các field của Citizen để lưu vào JSONB.
// Các field nhạy cảm được masked, không lưu ciphertext vào log.
type AuditCitizenSnapshot struct {
	FullName         string `json:"full_name"`
	DateOfBirth      string `json:"date_of_birth"`
	Gender           string `json:"gender"`
	NationalID       string `json:"national_id"`        // masked: "****1234"
	PhoneNumber      string `json:"phone_number"`       // masked: "****5678"
	Email            string `json:"email"`              // masked: "us***@example.com"
	PermanentAddress string `json:"permanent_address"`  // masked: "123 ****"
	Religion         string `json:"religion"`
	Ethnicity        string `json:"ethnicity"`
	MaritalStatus    string `json:"marital_status"`
	ProvinceCode     string `json:"province_code"`
	DistrictCode     string `json:"district_code"`
	WardCode         string `json:"ward_code"`
	IsAlive          bool   `json:"is_alive"`
}

// ──────────────────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────────────────

// ListAuditLogFilter — tham số lọc khi tra cứu audit log
type ListAuditLogFilter struct {
	CitizenID string      `form:"citizen_id"`
	Action    AuditAction `form:"action"`
	ChangedBy string      `form:"changed_by"` // lọc theo user_id
	From      string      `form:"from"`        // "2006-01-02" hoặc "2006-01-02T15:04:05Z"
	To        string      `form:"to"`
	Page      int         `form:"page,default=1"`
	PageSize  int         `form:"page_size,default=20"`
}

// AuditLogResponse — DTO trả về cho client
type AuditLogResponse struct {
	ID             string          `json:"id"`
	CitizenID      string          `json:"citizen_id"`
	Action         AuditAction     `json:"action"`
	ChangedBy      string          `json:"changed_by"`
	ChangedByName  string          `json:"changed_by_name"`
	ChangedByRole  string          `json:"changed_by_role"`
	OldValues      json.RawMessage `json:"old_values"`
	NewValues      json.RawMessage `json:"new_values"`
	ChangedAt      time.Time       `json:"changed_at"`
}

// AuditLogListResponse — paginated list
type AuditLogListResponse struct {
	Data     []AuditLogResponse `json:"data"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}