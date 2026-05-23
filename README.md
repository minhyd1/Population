# 🏛️ Population Service

API quản lý dân số Việt Nam viết bằng Golang, PostgreSQL, theo chuẩn microservice.

## Cấu trúc dự án

```
population-service/
├── api/
│   ├── docs/                          # Swagger auto-generated
│   └── population_service.postman_collection.json
├── cmd/
│   └── server/
│       ├── main.go                    # Entry point, router setup
│       └── config.go                  # Config, DB connection
├── configs/
│   └── .env.example                   # Template biến môi trường
├── internal/
│   ├── handler/
│   │   └── citizen_handler.go         # HTTP handlers (Gin)
│   ├── service/
│   │   └── citizen_service.go         # Business logic + mã hóa
│   ├── repository/
│   │   ├── citizen_repository.go      # DB queries (sqlx)
│   │   └── province_repository.go
│   └── model/
│       ├── citizen.go                 # Domain models
│       └── dto.go                     # Request/Response DTOs
├── pkg/
│   ├── crypto/
│   │   └── crypto.go                  # AES-256-GCM encryption
│   ├── response/
│   │   └── response.go                # Standard API response
│   └── middleware/
│       └── middleware.go              # Gin middlewares
├── deployments/
│   ├── Dockerfile
│   └── docker-compose.yml
└── scripts/
    ├── 001_create_tables.sql          # Schema migration
    ├── 002_seed_data.sql              # Seed provinces/districts/wards
    └── seed.go                        # Go seeder (mã hóa đúng cách)
```

## 🔐 Bảo mật dữ liệu nhạy cảm

### Các trường được mã hóa (AES-256-GCM):
| Field | Ý nghĩa |
|-------|---------|
| `national_id` | Số CCCD/CMND |
| `phone_number` | Số điện thoại |
| `email` | Địa chỉ email |
| `permanent_address` | Địa chỉ thường trú |

### Flow mã hóa:
```
Client gửi plaintext
    → Service encrypt → DB lưu ciphertext
    → Service trả về ciphertext (encrypted)
    → Frontend decrypt bằng shared key
```

### Frontend decrypt (JavaScript):
```javascript
async function decryptField(base64Ciphertext, base64Key) {
  const keyBytes = Uint8Array.from(atob(base64Key), c => c.charCodeAt(0));
  const cipherBytes = Uint8Array.from(atob(base64Ciphertext), c => c.charCodeAt(0));
  const iv = cipherBytes.slice(0, 12);          // 12 bytes nonce
  const data = cipherBytes.slice(12);            // ciphertext + auth tag
  const cryptoKey = await crypto.subtle.importKey(
    'raw', keyBytes, 'AES-GCM', false, ['decrypt']
  );
  const decrypted = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv }, cryptoKey, data
  );
  return new TextDecoder().decode(decrypted);
}

// Sử dụng:
const encryptedNationalID = citizen.national_id; // từ API response
const plainNationalID = await decryptField(encryptedNationalID, YOUR_BASE64_KEY);
```

### Response luôn có `encrypted_fields`:
```json
{
  "id": "uuid...",
  "full_name": "Nguyễn Văn An",
  "national_id": "base64-encrypted-ciphertext",
  "phone_number": "base64-encrypted-ciphertext",
  "email": "base64-encrypted-ciphertext",
  "permanent_address": "base64-encrypted-ciphertext",
  "encrypted_fields": ["national_id", "phone_number", "email", "permanent_address"]
}
```

## 🚀 Hướng dẫn chạy

### 1. Tạo file .env

```bash
cp configs/.env.example configs/.env
```

Tạo encryption key:
```bash
openssl rand -base64 32
# Output ví dụ: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
```

Điền vào `configs/.env`:
```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=population_db
ENCRYPTION_KEY=<output từ lệnh trên>
```

### 2. Chạy với Docker Compose (khuyến nghị)

```bash
# Set encryption key
export ENCRYPTION_KEY=$(openssl rand -base64 32)

cd deployments
docker-compose up -d
```

### 3. Chạy thủ công

```bash
# Tạo DB
createdb population_db

# Chạy migration
psql -d population_db -f scripts/001_create_tables.sql
psql -d population_db -f scripts/002_seed_data.sql

# Cài go dependencies
go mod tidy

# Generate Swagger docs
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o api/docs

# Chạy server
go run ./cmd/server/...

# Seed dữ liệu mẫu (với mã hóa đúng)
go run scripts/seed.go
```

## 📖 API Endpoints

| Method | URL | Mô tả |
|--------|-----|-------|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/citizens` | Tạo công dân mới |
| `GET` | `/api/v1/citizens` | Danh sách (có filter + pagination) |
| `GET` | `/api/v1/citizens/:id` | Chi tiết công dân |
| `PATCH` | `/api/v1/citizens/:id` | Cập nhật thông tin |
| `DELETE` | `/api/v1/citizens/:id` | Xóa (soft delete) |
| `GET` | `/api/v1/population/stats` | Thống kê dân số tất cả tỉnh |
| `GET` | `/api/v1/population/stats/:province_code` | Thống kê theo tỉnh |
| `GET` | `/api/v1/encryption/meta` | Metadata mã hóa |
| `GET` | `/swagger/index.html` | Swagger UI |

### Query Parameters cho `/citizens`:
- `province_code` - Lọc theo tỉnh
- `district_code` - Lọc theo huyện
- `ward_code` - Lọc theo xã
- `gender` - `male` / `female` / `other`
- `marital_status` - `single` / `married` / `divorced` / `widowed`
- `is_alive` - `true` / `false`
- `search` - Tìm kiếm theo tên (ILIKE)
- `page` - Trang (default: 1)
- `page_size` - Số bản ghi/trang (default: 20, max: 100)

## 🧪 Test

```bash
# Unit test crypto
go test ./pkg/crypto/... -v

# Import Postman collection
# File: api/population_service.postman_collection.json
```

## 📝 Notes

- **Soft delete**: Dữ liệu không bị xóa thật, chỉ set `deleted_at`
- **Pagination**: Mọi endpoint danh sách đều có pagination
- **Encryption**: AES-256-GCM, IV ngẫu nhiên mỗi lần → cùng plaintext cho ciphertext khác nhau
- **Key rotation**: Thêm `key_version` vào header `X-Encryption-Key-Version` để support rotate key
