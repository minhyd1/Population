package model

import (
	"time"
)

// Gender represents citizen gender
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// MaritalStatus represents citizen marital status
type MaritalStatus string

const (
	MaritalSingle   MaritalStatus = "single"
	MaritalMarried  MaritalStatus = "married"
	MaritalDivorced MaritalStatus = "divorced"
	MaritalWidowed  MaritalStatus = "widowed"
)

// Citizen is the raw domain model (stored in DB, sensitive fields are encrypted at rest)
type Citizen struct {
	ID              string        `db:"id"`
	FullName        string        `db:"full_name"`          // Plain text
	DateOfBirth     time.Time     `db:"date_of_birth"`      // Plain text
	Gender          Gender        `db:"gender"`             // Plain text
	NationalID      string        `db:"national_id"`        // ENCRYPTED (CCCD/CMND)
	PhoneNumber     string        `db:"phone_number"`       // ENCRYPTED
	Email           string        `db:"email"`              // ENCRYPTED
	PermanentAddress string       `db:"permanent_address"`  // ENCRYPTED (địa chỉ thường trú)
	Religion        string        `db:"religion"`           // Plain text
	Ethnicity       string        `db:"ethnicity"`          // Plain text
	MaritalStatus   MaritalStatus `db:"marital_status"`     // Plain text
	ProvinceCode    string        `db:"province_code"`      // FK to provinces
	DistrictCode    string        `db:"district_code"`      // FK to districts
	WardCode        string        `db:"ward_code"`          // FK to wards
	IsAlive         bool          `db:"is_alive"`
	CreatedAt       time.Time     `db:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"`
	DeletedAt       *time.Time    `db:"deleted_at"`
}

// Province represents a Vietnamese province/city
type Province struct {
	Code      string    `db:"code"`
	Name      string    `db:"name"`
	NameEn    string    `db:"name_en"`
	CreatedAt time.Time `db:"created_at"`
}

// District represents a district inside a province
type District struct {
	Code         string    `db:"code"`
	Name         string    `db:"name"`
	ProvinceCode string    `db:"province_code"`
	CreatedAt    time.Time `db:"created_at"`
}

// Ward represents a ward inside a district
type Ward struct {
	Code         string    `db:"code"`
	Name         string    `db:"name"`
	DistrictCode string    `db:"district_code"`
	CreatedAt    time.Time `db:"created_at"`
}

// PopulationStat is aggregated population data per province
type PopulationStat struct {
	ProvinceCode  string  `db:"province_code"`
	ProvinceName  string  `db:"province_name"`
	Total         int64   `db:"total"`
	Male          int64   `db:"male"`
	Female        int64   `db:"female"`
	Other         int64   `db:"other"`
	Alive         int64   `db:"alive"`
	Deceased      int64   `db:"deceased"`
	AverageAge    float64 `db:"average_age"`
}
