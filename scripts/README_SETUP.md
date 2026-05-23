# Hướng dẫn chạy dự án

## Thông tin kết nối
- Host: localhost
- Port: 5433
- User: postgres  
- Password: 1234
- Database: population_db
- ENCRYPTION_KEY: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=

## Bước 1 - Tạo database (chạy trong PowerShell Admin)
```
psql -U postgres -p 5433 -c "CREATE DATABASE population_db;"
```

## Bước 2 - Tạo bảng
```
psql -U postgres -p 5433 -d population_db -f scripts/001_create_tables.sql
```

## Bước 3 - Seed tỉnh/huyện/xã
```
psql -U postgres -p 5433 -d population_db -f scripts/002_seed_data.sql
```

## Bước 4 - Seed 30 công dân mẫu
```
psql -U postgres -p 5433 -d population_db -f scripts/003_seed_citizens.sql
```

## Bước 5 - Cài dependencies Go
```
go mod tidy
```

## Bước 6 - Tạo thư mục docs
```
mkdir api\docs
echo package docs > api\docs\docs.go
```

## Bước 7 - Chạy server
```
go run ./cmd/server/...
```

## Kiểm tra
- API: http://localhost:8080/health
- Thống kê dân số: http://localhost:8080/api/v1/population/stats
- Danh sách công dân: http://localhost:8080/api/v1/citizens
