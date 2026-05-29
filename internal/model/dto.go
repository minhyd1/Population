package model

import "time"

// ========== REQUEST DTOs ==========

// CreateCitizenRequest - dùng để tạo mới công dân
type CreateCitizenRequest struct {
	FullName         string        `json:"full_name" binding:"required"`
	DateOfBirth      string        `json:"date_of_birth" binding:"required"` // "2006-01-02"
	Gender           Gender        `json:"gender" binding:"required,oneof=male female other"`
	NationalID       string        `json:"national_id" binding:"required,min=9,max=12"` // CCCD/CMND
	PhoneNumber      string        `json:"phone_number"`
	Email            string        `json:"email"`
	PermanentAddress string        `json:"permanent_address" binding:"required"`
	Religion         string        `json:"religion"`
	Ethnicity        string        `json:"ethnicity"`
	MaritalStatus    MaritalStatus `json:"marital_status" binding:"required,oneof=single married divorced widowed"`
	ProvinceCode     string        `json:"province_code" binding:"required"`
	DistrictCode     string        `json:"district_code" binding:"required"`
	WardCode         string        `json:"ward_code" binding:"required"`
	IsAlive          *bool         `json:"is_alive"`
}

// UpdateCitizenRequest - dùng để cập nhật thông tin công dân
type UpdateCitizenRequest struct {
	FullName         *string        `json:"full_name"`
	DateOfBirth      *string        `json:"date_of_birth"`
	Gender           *Gender        `json:"gender"`
	NationalID       *string        `json:"national_id"`
	PhoneNumber      *string        `json:"phone_number"`
	Email            *string        `json:"email"`
	PermanentAddress *string        `json:"permanent_address"`
	Religion         *string        `json:"religion"`
	Ethnicity        *string        `json:"ethnicity"`
	MaritalStatus    *MaritalStatus `json:"marital_status"`
	ProvinceCode     *string        `json:"province_code"`
	DistrictCode     *string        `json:"district_code"`
	WardCode         *string        `json:"ward_code"`
	IsAlive          *bool          `json:"is_alive"`
}

// ListCitizenFilter - filter khi list
type ListCitizenFilter struct {
	ProvinceCode  string `form:"province_code"`
	DistrictCode  string `form:"district_code"`
	WardCode      string `form:"ward_code"`
	Gender        string `form:"gender"`
	MaritalStatus string `form:"marital_status"`
	IsAlive       *bool  `form:"is_alive"`
	Search        string `form:"search"` // search by full_name
	Page          int    `form:"page,default=1"`
	PageSize      int    `form:"page_size,default=20"`
}

// ========== RESPONSE DTOs ==========

// CitizenResponse - response trả về cho client.
// Các trường nhạy cảm (NationalID, PhoneNumber, Email, PermanentAddress)
// đã được server decrypt — client nhận plaintext qua HTTPS, không cần xử lý thêm.
type CitizenResponse struct {
	ID               string        `json:"id"`
	FullName         string        `json:"full_name"`
	DateOfBirth      time.Time     `json:"date_of_birth"`
	Gender           Gender        `json:"gender"`
	NationalID       string        `json:"national_id"`
	PhoneNumber      string        `json:"phone_number"`
	Email            string        `json:"email"`
	PermanentAddress string        `json:"permanent_address"`
	Religion         string        `json:"religion"`
	Ethnicity        string        `json:"ethnicity"`
	MaritalStatus    MaritalStatus `json:"marital_status"`
	ProvinceCode     string        `json:"province_code"`
	DistrictCode     string        `json:"district_code"`
	WardCode         string        `json:"ward_code"`
	IsAlive          bool          `json:"is_alive"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// CitizenListResponse - paginated list
type CitizenListResponse struct {
	Data     []CitizenResponse `json:"data"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// PopulationStatResponse - thống kê dân số theo tỉnh
type PopulationStatResponse struct {
	ProvinceCode string  `json:"province_code"`
	ProvinceName string  `json:"province_name"`
	Total        int64   `json:"total"`
	Male         int64   `json:"male"`
	Female       int64   `json:"female"`
	Other        int64   `json:"other"`
	Alive        int64   `json:"alive"`
	Deceased     int64   `json:"deceased"`
	AverageAge   float64 `json:"average_age"`
}

// ProvinceResponse
type ProvinceResponse struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	NameEn string `json:"name_en"`
}

// EncryptionMetaResponse đã được xóa.
// Server tự decrypt trước khi trả response — client không cần biết thông tin mã hóa.