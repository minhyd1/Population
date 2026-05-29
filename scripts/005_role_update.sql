-- ============================================================
-- Migration: 005_roles_upgrade.sql
-- Nâng cấp bảng users từ 2 role cũ (admin/citizen) lên 9 role mới
-- ============================================================

-- Bước 1: Đổi kiểu cột role sang TEXT tạm thời để thay giá trị
ALTER TABLE users ALTER COLUMN role TYPE TEXT;

-- Bước 2: XÓA constraint cũ TRƯỚC KHI update dữ liệu
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

-- Bước 3: Map role cũ → role mới (Lúc này không còn constraint cũ cản trở)
UPDATE users SET role = 'super_admin'  WHERE role = 'admin';
UPDATE users SET role = 'citizen_self' WHERE role = 'citizen';

-- Bước 4: Thêm cột địa bàn
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS province_code VARCHAR(10) REFERENCES provinces(code),
  ADD COLUMN IF NOT EXISTS district_code VARCHAR(10) REFERENCES districts(code),
  ADD COLUMN IF NOT EXISTS ward_code     VARCHAR(10) REFERENCES wards(code);

-- Bước 5: THÊM LẠI constraint với 9 role mới (Dữ liệu đã chuẩn xác để check)
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN (
    'super_admin',
    'national_manager',
    'province_manager',
    'district_manager',
    'ward_officer',
    'data_entry',
    'auditor',
    'analytics_viewer',
    'citizen_self'
  ));

-- Bước 6: Index cho địa bàn (dùng trong RequireScopeMatch)
CREATE INDEX IF NOT EXISTS idx_users_province_code ON users(province_code) WHERE province_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_district_code ON users(district_code) WHERE district_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_ward_code     ON users(ward_code)     WHERE ward_code IS NOT NULL;

-- Bước 7: Cập nhật comment
COMMENT ON COLUMN users.role IS
  '9 roles: super_admin | national_manager | province_manager | district_manager | ward_officer | data_entry | auditor | analytics_viewer | citizen_self';
COMMENT ON COLUMN users.province_code IS 'Tỉnh phụ trách — bắt buộc với province_manager';
COMMENT ON COLUMN users.district_code IS 'Huyện phụ trách — bắt buộc với district_manager';
COMMENT ON COLUMN users.ward_code     IS 'Xã phụ trách — bắt buộc với ward_officer';

-- Bước 8: Seed thêm tài khoản mẫu cho từng role
-- Tất cả dùng password: Test@1234 (bcrypt cost 10)
-- ĐỔI PASSWORD NGAY SAU KHI DÙNG!
INSERT INTO users (id, username, password_hash, role, is_active) VALUES
  (uuid_generate_v4(), 'national_mgr',
   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
   'national_manager', true),
  (uuid_generate_v4(), 'auditor1',
   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
   'auditor', true),
  (uuid_generate_v4(), 'analytics1',
   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
   'analytics_viewer', true),
  (uuid_generate_v4(), 'dataentry1',
   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
   'data_entry', true)
ON CONFLICT (username) DO NOTHING;

-- province_manager, district_manager, ward_officer cần code địa bàn thật
-- → Tạo sau khi đã có dữ liệu provinces/districts/wards
-- Ví dụ:
-- INSERT INTO users (id, username, password_hash, role, province_code, is_active)
-- VALUES (uuid_generate_v4(), 'pm_hanoi', '$2a$10$...', 'province_manager', '01', true);