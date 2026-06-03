package model

import "time"

// Household đại diện cho một hộ gia đình
type Household struct {
	ID            string     `db:"id"`
	HouseholdNo   string     `db:"household_no"`   // Số sổ hộ khẩu
	ProvinceCode  string     `db:"province_code"`
	DistrictCode  string     `db:"district_code"`
	WardCode      string     `db:"ward_code"`
	Address       string     `db:"address"`          // Địa chỉ cụ thể (ENCRYPTED)
	HeadCitizenID *string    `db:"head_citizen_id"`  // Chủ hộ
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

// HouseholdMember liên kết công dân với hộ gia đình
type HouseholdMember struct {
	HouseholdID  string    `db:"household_id"`
	CitizenID    string    `db:"citizen_id"`
	Relationship string    `db:"relationship"` // chủ hộ, vợ/chồng, con, cha/mẹ, ...
	JoinedAt     time.Time `db:"joined_at"`
}

// ResidenceHistory lưu lịch sử cư trú của công dân
type ResidenceHistory struct {
	ID               string     `db:"id"`
	CitizenID        string     `db:"citizen_id"`
	FromHouseholdID  *string    `db:"from_household_id"` // NULL = đăng ký lần đầu
	ToHouseholdID    string     `db:"to_household_id"`
	TransferRequestID *string   `db:"transfer_request_id"` // NULL = đăng ký mới
	Reason           string     `db:"reason"`
	EffectiveDate    time.Time  `db:"effective_date"`
	CreatedAt        time.Time  `db:"created_at"`
}

// ─── DTOs ───────────────────────────────────────────────────

type CreateHouseholdRequest struct {
	HouseholdNo   string  `json:"household_no"    binding:"required"`
	ProvinceCode  string  `json:"province_code"   binding:"required"`
	DistrictCode  string  `json:"district_code"   binding:"required"`
	WardCode      string  `json:"ward_code"       binding:"required"`
	Address       string  `json:"address"         binding:"required"`
	HeadCitizenID *string `json:"head_citizen_id"`
}

type AddHouseholdMemberRequest struct {
	CitizenID    string `json:"citizen_id"    binding:"required"`
	Relationship string `json:"relationship"  binding:"required"`
}

type HouseholdResponse struct {
	ID            string                   `json:"id"`
	HouseholdNo   string                   `json:"household_no"`
	ProvinceCode  string                   `json:"province_code"`
	DistrictCode  string                   `json:"district_code"`
	WardCode      string                   `json:"ward_code"`
	Address       string                   `json:"address"`
	HeadCitizenID *string                  `json:"head_citizen_id,omitempty"`
	Members       []HouseholdMemberResponse `json:"members,omitempty"`
	CreatedAt     time.Time                `json:"created_at"`
}

type HouseholdMemberResponse struct {
	CitizenID    string    `json:"citizen_id"`
	FullName     string    `json:"full_name"`
	Relationship string    `json:"relationship"`
	JoinedAt     time.Time `json:"joined_at"`
}

type ListHouseholdFilter struct {
	ProvinceCode string `form:"province_code"`
	DistrictCode string `form:"district_code"`
	WardCode     string `form:"ward_code"`
	Page         int    `form:"page,default=1"`
	PageSize     int    `form:"page_size,default=20"`
}

type HouseholdListResponse struct {
	Data     []HouseholdResponse `json:"data"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}