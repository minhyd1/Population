# Ghi chú sửa đổi (2026-07-06)

## Đã sửa thật trong code

1. **README.md**
   - Xóa conflict marker `<<<<<<< HEAD / ======= / >>>>>>>`.
   - Sửa mục bảo mật: README cũ mô tả sai là "frontend decrypt bằng shared
     key" — code thật decrypt ở server (`internal/service/citizen/helpers.go`),
     client chỉ nhận plaintext qua HTTPS. Đã cập nhật lại đúng thực tế + JSON
     ví dụ.

2. **Tách `main.go` (316 dòng) thành 3 file** — `cmd/server/`:
   - `container.go`: toàn bộ DI (repo → service → handler), tương đương
     `InitRepository → InitService → InitHandler` bạn đề xuất.
   - `router.go`: toàn bộ route + middleware.
   - `main.go`: chỉ còn load config, connect DB, gọi container/router, chạy
     server (~25 dòng).
   - *Không* dùng wire/fx — DI thủ công nhưng đã tách trách nhiệm rõ ràng.
     Nếu sau này thực sự cần graph DI phức tạp hơn, cấu trúc này migrate sang
     wire dễ hơn nhiều so với 1 file 316 dòng.

3. **Bug atomicity trong transfer workflow** (nghiêm trọng hơn bug bạn nêu về
   `claimsFromCtx`, và có thật):
   - Trước: `NewTransferService` nhận `db *sqlx.DB` nhưng không dùng.
     `executeTransferInTx` gọi `householdRepo.RemoveMember/AddMember` và
     `citizenRepo.UpdateResidence` **ngoài transaction**, trong khi
     `CreateResidenceHistory`/`CompleteRequest` chạy trong transaction riêng.
     Nếu lỗi giữa chừng → dữ liệu household/citizen lệch vĩnh viễn với
     transfer_request.
   - Sau: thêm `TxHouseholdOps`/`TxCitizenOps` (repository package) chạy
     chung `*sqlx.Tx` với `TransferRepository`. `WithTx` giờ expose luôn
     `*sqlx.Tx` để build 2 ops này. Toàn bộ 5 bước của transfer giờ
     commit/rollback như một khối. Áp dụng cho cả `ApproveTransfer` lẫn
     `ForceApproveTransfer` → `executeTransfer`.
   - Đã bỏ tham số `db` không dùng khỏi `NewTransferService`.
   - Audit log insert (`InsertWithVisibility`) **cố ý** vẫn nằm ngoài
     transaction chính (best-effort) — có ghi rõ lý do trong code comment.

4. **Config validation** (`cmd/server/config.go`):
   - Thêm `Config.Validate()`: chặn khởi động nếu `APP_ENV=production` mà
     `JWT_ACCESS_SECRET`/`JWT_REFRESH_SECRET` vẫn là giá trị mặc định trong
     code, hoặc secret ngắn hơn 16 ký tự. Ở development chỉ cảnh báo.

5. **CI/CD**: `.github/workflows/ci.yml` — go vet, go build, go test
   (-race, coverage), golangci-lint, có service Postgres cho test cần DB
   thật sau này.

6. **Lint**: `.golangci.yml` — bật govet, staticcheck, errcheck, bodyclose,
   gosec, unused, ineffassign, gosimple, misspell, unconvert.

7. **Test mới**: `internal/service/assignment_service_test.go` (6 test case,
   mock tay không cần thư viện ngoài — minh chứng cho việc interface sẵn có
   giúp mock dễ) và `internal/service/transfer_service_test.go` (regression
   test cho scope fields trong `claimsFromCtx`).

## Đánh giá lại các điểm bạn nêu nhưng KHÔNG đúng với code hiện tại

- "Thiếu interface" → repo đã 100% là interface từ trước.
- "Thiếu context propagation" → mọi method đã nhận `ctx` từ trước.
- Bug `claimsFromCtx` thiếu scope field → đã được fix trước khi tôi đọc code
  (đã có test hồi quy để không bị regress lại).

## Cố tình KHÔNG làm (ngoài khả năng xác minh an toàn trong 1 lần sửa)

- **80-100 unit test**: chỉ thêm ví dụ minh họa (9 test case). Viết đủ
  80-100 test có ý nghĩa cho toàn bộ service/handler/repository là công sức
  nhiều ngày, không nên làm ẩu để đạt số lượng.
- **wire/fx, Unit of Work tổng quát cho mọi service, cache layer (Redis cho
  province/district/ward/statistics), structured logging (zap/slog) thay
  `log.Println`, golang-migrate/goose thay script SQL thủ công**: đây đều là
  đề xuất đúng hướng nhưng là các quyết định kiến trúc lớn, ảnh hưởng nhiều
  file — nên làm riêng từng cái, review kỹ, thay vì nhét chung vào 1 lần sửa
  không biên dịch được.
- ⚠️ **Không có Go toolchain trong môi trường này để chạy `go build`/`go
  test`/`go vet`/`golangci-lint` thật.** Code đã được soát tay kỹ (đọc từng
  interface, kiểm tra method signature khớp `*sqlx.DB`/`*sqlx.Tx`), nhưng bạn
  nên chạy các lệnh sau trước khi merge:
  ```bash
  go build ./...
  go vet ./...
  go test ./...
  golangci-lint run
  ```
