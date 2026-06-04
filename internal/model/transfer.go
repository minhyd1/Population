package model

import "time"

// TransferStatus trạng thái yêu cầu chuyển hộ khẩu
type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusApproved  TransferStatus = "approved"
	TransferStatusRejected  TransferStatus = "rejected"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusCancelled TransferStatus = "cancelled"
)

// ApprovalLevel cấp phê duyệt cần thiết dựa trên phạm vi chuyển
type ApprovalLevel string

const (
	ApprovalLevelNone     ApprovalLevel = "none"     // cùng phường — không cần workflow
	ApprovalLevelWard     ApprovalLevel = "ward"     // khác phường cùng quận
	ApprovalLevelDistrict ApprovalLevel = "district" // khác quận cùng tỉnh
	ApprovalLevelProvince ApprovalLevel = "province" // khác tỉnh
)

// ApprovalDecision quyết định của một đơn vị hành chính
type ApprovalDecision string

const (
	ApprovalDecisionPending  ApprovalDecision = "pending"
	ApprovalDecisionApproved ApprovalDecision = "approved"
	ApprovalDecisionRejected ApprovalDecision = "rejected"
)

// TransferRequest yêu cầu chuyển hộ khẩu.
//
// Thiết kế snapshot:
//   - from_unit_code / to_unit_code được lưu ngay khi tạo request
//   - Nếu household sau này đổi địa chỉ, lịch sử request vẫn bất biến
//   - Workflow phê duyệt (transfer_approvals) cũng dùng unit_code, không dùng user_id
type TransferRequest struct {
	ID              string         `db:"id"`
	CitizenID       string         `db:"citizen_id"`
	FromHouseholdID string         `db:"from_household_id"`
	ToHouseholdID   string         `db:"to_household_id"`
	// Snapshot đơn vị tại thời điểm tạo — bất biến (điểm 3)
	FromUnitCode    *string        `db:"from_unit_code"` // ward_code nơi đi
	ToUnitCode      *string        `db:"to_unit_code"`   // ward_code nơi đến
	ApprovalLevel   ApprovalLevel  `db:"approval_level"`
	Status          TransferStatus `db:"status"`
	Reason          string         `db:"reason"`
	CreatedBy       string         `db:"created_by"`    // user_id người tạo
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
	CompletedAt     *time.Time     `db:"completed_at"`
	// Escalation
	DeadlineAt      *time.Time     `db:"deadline_at"`  // 7 ngày sau khi tạo
	EscalatedTo     *string        `db:"escalated_to"` // unit_code cấp trên sau escalate
	// Optimistic locking
	LockVersion     int            `db:"lock_version"`
}

// TransferApproval phiếu phê duyệt của một đơn vị hành chính.
// unit_code lưu theo đơn vị, không theo cá nhân → bền vững với thay đổi nhân sự.
// Kết hợp với user_assignments để biết ai phụ trách unit_code đó lúc duyệt.
type TransferApproval struct {
	ID           string           `db:"id"`
	RequestID    string           `db:"request_id"`
	UnitCode     string           `db:"unit_code"`    // mã đơn vị hành chính
	UnitRole     string           `db:"unit_role"`    // "source" hoặc "destination"
	Decision     ApprovalDecision `db:"decision"`
	ApprovedBy   *string          `db:"approved_by"`  // user_id người duyệt
	RejectReason *string          `db:"reject_reason"`
	ApprovedAt   *time.Time       `db:"approved_at"`
}

// ─── DTOs ───────────────────────────────────────────────────

type CreateTransferRequest struct {
	CitizenID       string `json:"citizen_id"        binding:"required"`
	FromHouseholdID string `json:"from_household_id" binding:"required"`
	ToHouseholdID   string `json:"to_household_id"   binding:"required"`
	Reason          string `json:"reason"            binding:"required"`
}

type ApproveTransferRequest struct {
	Decision     ApprovalDecision `json:"decision"      binding:"required"`
	RejectReason string           `json:"reject_reason"` // bắt buộc khi rejected
}

type ForceApproveRequest struct {
	Reason string `json:"reason" binding:"required"` // bắt buộc — ghi audit riêng
}

type TransferApprovalResponse struct {
	ID           string           `json:"id"`
	UnitCode     string           `json:"unit_code"`
	UnitRole     string           `json:"unit_role"`
	Decision     ApprovalDecision `json:"decision"`
	ApprovedBy   *string          `json:"approved_by,omitempty"`
	RejectReason *string          `json:"reject_reason,omitempty"`
	ApprovedAt   *time.Time       `json:"approved_at,omitempty"`
}

type TransferRequestResponse struct {
	ID              string                     `json:"id"`
	CitizenID       string                     `json:"citizen_id"`
	CitizenName     string                     `json:"citizen_name,omitempty"`
	FromHouseholdID string                     `json:"from_household_id"`
	ToHouseholdID   string                     `json:"to_household_id"`
	// Snapshot đơn vị — bất biến
	FromUnitCode    *string                    `json:"from_unit_code,omitempty"`
	ToUnitCode      *string                    `json:"to_unit_code,omitempty"`
	ApprovalLevel   ApprovalLevel              `json:"approval_level"`
	Status          TransferStatus             `json:"status"`
	Reason          string                     `json:"reason"`
	CreatedBy       string                     `json:"created_by"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	CompletedAt     *time.Time                 `json:"completed_at,omitempty"`
	DeadlineAt      *time.Time                 `json:"deadline_at,omitempty"`
	EscalatedTo     *string                    `json:"escalated_to,omitempty"`
	Approvals       []TransferApprovalResponse `json:"approvals,omitempty"`
}

type ListTransferFilter struct {
	CitizenID    string         `form:"citizen_id"`
	Status       TransferStatus `form:"status"`
	ProvinceCode string         `form:"province_code"`
	DistrictCode string         `form:"district_code"`
	WardCode     string         `form:"ward_code"`
	Page         int            `form:"page,default=1"`
	PageSize     int            `form:"page_size,default=20"`
}

type TransferListResponse struct {
	Data     []TransferRequestResponse `json:"data"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type ResidenceHistoryResponse struct {
	ID                string     `json:"id"`
	CitizenID         string     `json:"citizen_id"`
	FromHouseholdID   *string    `json:"from_household_id,omitempty"`
	ToHouseholdID     string     `json:"to_household_id"`
	TransferRequestID *string    `json:"transfer_request_id,omitempty"`
	Reason            string     `json:"reason"`
	EffectiveDate     time.Time  `json:"effective_date"`
	CreatedAt         time.Time  `json:"created_at"`
}
