-- ============================================================
-- 006_seed_users_full.sql
-- Seed đầy đủ tài khoản cho CẢ 9 ROLE
-- Password mặc định tất cả: Test@1234
-- Hash bcrypt cost 10: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
-- ĐỔI MẬT KHẨU NGAY SAU KHI DEPLOY LÊN PRODUCTION!
-- ============================================================

-- ── 1. super_admin ────────────────────────────────────────
-- Quyền cao nhất: quản lý user, CRUD công dân, xóa, thống kê
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES
    (uuid_generate_v4(), 'super_admin',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'super_admin', true)
ON CONFLICT (username) DO NOTHING;

-- ── 2. national_manager ───────────────────────────────────
-- Không giới hạn địa bàn: CRUD công dân (kể cả xóa), thống kê toàn quốc
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES
    (uuid_generate_v4(), 'national_mgr',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'national_manager', true)
ON CONFLICT (username) DO NOTHING;

-- ── 3. province_manager ───────────────────────────────────
-- Mỗi nick gắn với 1 tỉnh — xem/tạo/sửa công dân trong tỉnh mình
-- province: Hà Nội (01), TP.HCM (79), Đà Nẵng (48), Cần Thơ (92)
INSERT INTO users (id, username, password_hash, role, province_code, is_active)
VALUES
    (uuid_generate_v4(), 'pm_hanoi',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'province_manager', '01', true),

    (uuid_generate_v4(), 'pm_hcm',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'province_manager', '79', true),

    (uuid_generate_v4(), 'pm_danang',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'province_manager', '48', true),

    (uuid_generate_v4(), 'pm_cantho',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'province_manager', '92', true)
ON CONFLICT (username) DO NOTHING;

-- ── 4. district_manager ───────────────────────────────────
-- Mỗi nick gắn với 1 quận/huyện — xem/tạo/sửa công dân trong huyện mình
-- districts: Ba Đình (001), Hoàn Kiếm (002), Q.1 HCM (760), Hải Châu (490), Ninh Kiều (900)
INSERT INTO users (id, username, password_hash, role, province_code, district_code, is_active)
VALUES
    (uuid_generate_v4(), 'dm_badinh',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'district_manager', '01', '001', true),

    (uuid_generate_v4(), 'dm_hoankiem',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'district_manager', '01', '002', true),

    (uuid_generate_v4(), 'dm_quan1',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'district_manager', '79', '760', true),

    (uuid_generate_v4(), 'dm_haichau',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'district_manager', '48', '490', true),

    (uuid_generate_v4(), 'dm_ninhkieu',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'district_manager', '92', '900', true)
ON CONFLICT (username) DO NOTHING;

-- ── 5. ward_officer ───────────────────────────────────────
-- Mỗi nick gắn với 1 phường/xã — xem/tạo/sửa công dân trong xã mình
-- wards: Phúc Xá (00001), Trúc Bạch (00002), Phan Chu Trinh (00010),
--        Bến Nghé (26734), Bến Thành (26736), Hải Châu 1 (20194), Tân An (31150)
INSERT INTO users (id, username, password_hash, role, province_code, district_code, ward_code, is_active)
VALUES
    (uuid_generate_v4(), 'wo_phucxa',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '01', '001', '00001', true),

    (uuid_generate_v4(), 'wo_trucbach',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '01', '001', '00002', true),

    (uuid_generate_v4(), 'wo_phanchutrinh',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '01', '002', '00010', true),

    (uuid_generate_v4(), 'wo_bennghe',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '79', '760', '26734', true),

    (uuid_generate_v4(), 'wo_benthanh',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '79', '760', '26736', true),

    (uuid_generate_v4(), 'wo_haichau1',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '48', '490', '20194', true),

    (uuid_generate_v4(), 'wo_tanan',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'ward_officer', '92', '900', '31150', true)
ON CONFLICT (username) DO NOTHING;

-- ── 6. data_entry ─────────────────────────────────────────
-- Chỉ có quyền TẠO MỚI công dân, không xem danh sách, không sửa, không xóa
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES
    (uuid_generate_v4(), 'dataentry1',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'data_entry', true),

    (uuid_generate_v4(), 'dataentry2',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'data_entry', true)
ON CONFLICT (username) DO NOTHING;

-- ── 7. auditor ────────────────────────────────────────────
-- Chỉ đọc: xem danh sách công dân + xem thống kê toàn quốc và từng tỉnh
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES
    (uuid_generate_v4(), 'auditor1',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'auditor', true),

    (uuid_generate_v4(), 'auditor2',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'auditor', true)
ON CONFLICT (username) DO NOTHING;

-- ── 8. analytics_viewer ───────────────────────────────────
-- Chỉ xem thống kê dân số, không được xem/chạm dữ liệu công dân
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES
    (uuid_generate_v4(), 'analytics1',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'analytics_viewer', true),

    (uuid_generate_v4(), 'analytics2',
     '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
     'analytics_viewer', true)
ON CONFLICT (username) DO NOTHING;

-- ── 9. citizen_self ───────────────────────────────────────
-- Gắn với 1 citizen_id cụ thể — chỉ xem được hồ sơ của chính mình
-- Lấy ID thật từ bảng citizens sau khi đã seed 003_seed_citizens.sql
INSERT INTO users (id, username, password_hash, role, citizen_id, is_active)
SELECT
    uuid_generate_v4(),
    'citizen_' || SUBSTRING(c.id::text, 1, 8),
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'citizen_self',
    c.id,
    true
FROM citizens c
WHERE c.deleted_at IS NULL
ORDER BY c.created_at ASC
LIMIT 5
ON CONFLICT (username) DO NOTHING;

-- ============================================================
-- TỔNG KẾT tài khoản đã seed:
-- ============================================================
-- Role                | Username(s)
-- --------------------|--------------------------------------------------
-- super_admin         | super_admin
-- national_manager    | national_mgr
-- province_manager    | pm_hanoi, pm_hcm, pm_danang, pm_cantho
-- district_manager    | dm_badinh, dm_hoankiem, dm_quan1, dm_haichau, dm_ninhkieu
-- ward_officer        | wo_phucxa, wo_trucbach, wo_phanchutrinh, wo_bennghe, wo_benthanh, wo_haichau1, wo_tanan
-- data_entry          | dataentry1, dataentry2
-- auditor             | auditor1, auditor2
-- analytics_viewer    | analytics1, analytics2
-- citizen_self        | citizen_<id> x5 (tự động từ bảng citizens)
-- ============================================================
-- Tất cả password: Test@1234
-- ============================================================