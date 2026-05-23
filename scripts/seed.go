// scripts/seed.go - Chạy: go run scripts/seed.go
// Tạo dữ liệu mẫu với mã hóa đúng cách
//
// Usage:
//   ENCRYPTION_KEY=<your-key> DB_HOST=localhost DB_USER=postgres DB_PASSWORD=xxx DB_NAME=population_db go run scripts/seed.go

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"population-service/pkg/crypto"
)

func main() {
	_ = godotenv.Load("configs/.env")

	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatal("ENCRYPTION_KEY is required")
	}

	enc, err := crypto.New(encKey)
	if err != nil {
		log.Fatalf("Init encryptor: %v", err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_NAME", "population_db"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Open DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Ping DB: %v", err)
	}

	log.Println("✅ Connected to DB, seeding citizens...")

	// Sample citizens data
	type SeedCitizen struct {
		FullName         string
		DateOfBirth      string
		Gender           string
		NationalID       string // will be encrypted
		PhoneNumber      string // will be encrypted
		Email            string // will be encrypted
		PermanentAddress string // will be encrypted
		Religion         string
		Ethnicity        string
		MaritalStatus    string
		ProvinceCode     string
		DistrictCode     string
		WardCode         string
		IsAlive          bool
	}

	citizens := []SeedCitizen{
		{
			FullName: "Nguyễn Văn An", DateOfBirth: "1990-05-15",
			Gender: "male", NationalID: "079123456789",
			PhoneNumber: "0912345678", Email: "nguyen.van.an@email.com",
			PermanentAddress: "45 Phúc Xá, Ba Đình, Hà Nội",
			Religion: "Phật giáo", Ethnicity: "Kinh", MaritalStatus: "married",
			ProvinceCode: "01", DistrictCode: "001", WardCode: "00001", IsAlive: true,
		},
		{
			FullName: "Trần Thị Bình", DateOfBirth: "1985-08-20",
			Gender: "female", NationalID: "079987654321",
			PhoneNumber: "0987654321", Email: "tran.thi.binh@email.com",
			PermanentAddress: "12 Trúc Bạch, Ba Đình, Hà Nội",
			Religion: "Không", Ethnicity: "Kinh", MaritalStatus: "single",
			ProvinceCode: "01", DistrictCode: "001", WardCode: "00002", IsAlive: true,
		},
		{
			FullName: "Lê Hoàng Cường", DateOfBirth: "1975-12-01",
			Gender: "male", NationalID: "079111222333",
			PhoneNumber: "0903111222", Email: "le.hoang.cuong@email.com",
			PermanentAddress: "78 Bến Nghé, Quận 1, TP.HCM",
			Religion: "Thiên Chúa giáo", Ethnicity: "Kinh", MaritalStatus: "married",
			ProvinceCode: "79", DistrictCode: "760", WardCode: "26734", IsAlive: true,
		},
		{
			FullName: "Phạm Thị Dung", DateOfBirth: "2000-03-08",
			Gender: "female", NationalID: "079444555666",
			PhoneNumber: "0771444555", Email: "pham.thi.dung@email.com",
			PermanentAddress: "25 Bến Thành, Quận 1, TP.HCM",
			Religion: "Không", Ethnicity: "Hoa", MaritalStatus: "single",
			ProvinceCode: "79", DistrictCode: "760", WardCode: "26736", IsAlive: true,
		},
		{
			FullName: "Võ Minh Đức", DateOfBirth: "1960-07-22",
			Gender: "male", NationalID: "048777888999",
			PhoneNumber: "0236777888", Email: "vo.minh.duc@email.com",
			PermanentAddress: "15 Hải Châu 1, Đà Nẵng",
			Religion: "Không", Ethnicity: "Kinh", MaritalStatus: "divorced",
			ProvinceCode: "48", DistrictCode: "490", WardCode: "20194", IsAlive: true,
		},
		{
			FullName: "Nguyễn Thị Em", DateOfBirth: "1945-01-10",
			Gender: "female", NationalID: "079000111222",
			PhoneNumber: "0292000111", Email: "nguyen.thi.em@email.com",
			PermanentAddress: "33 Tân An, Ninh Kiều, Cần Thơ",
			Religion: "Phật giáo", Ethnicity: "Kinh", MaritalStatus: "widowed",
			ProvinceCode: "92", DistrictCode: "900", WardCode: "31150", IsAlive: false,
		},
	}

	insertSQL := `
		INSERT INTO citizens (
			id, full_name, date_of_birth, gender,
			national_id, phone_number, email, permanent_address,
			religion, ethnicity, marital_status,
			province_code, district_code, ward_code,
			is_alive, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT DO NOTHING
	`

	for _, s := range citizens {
		dob, _ := time.Parse("2006-01-02", s.DateOfBirth)

		encNID, _ := enc.Encrypt(s.NationalID)
		encPhone, _ := enc.Encrypt(s.PhoneNumber)
		encEmail, _ := enc.Encrypt(s.Email)
		encAddr, _ := enc.Encrypt(s.PermanentAddress)

		_, err := db.Exec(insertSQL,
			uuid.New().String(),
			s.FullName, dob, s.Gender,
			encNID, encPhone, encEmail, encAddr,
			s.Religion, s.Ethnicity, s.MaritalStatus,
			s.ProvinceCode, s.DistrictCode, s.WardCode,
			s.IsAlive, time.Now(), time.Now(),
		)
		if err != nil {
			log.Printf("⚠️  Failed to insert %s: %v", s.FullName, err)
		} else {
			log.Printf("✅ Seeded: %s", s.FullName)
		}
	}

	log.Println("✅ Seeding complete!")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
