-- ============================================================
-- Migration: 010_admin_units_and_assignments.sql
-- Thêm:
--   1. administrative_units — cây hành chính thống nhất
--   2. user_assignments      — gắn user vào đơn vị + lịch sử
--   3. Thêm left_at + is_active vào household_members
--   4. Snapshot unit_code vào transfer_requests
--   5. Nâng cấp audit_visibility (đã tạo ở 009, thêm FK + index mới)
--   6. permissions + role_permissions — RBAC thực sự
-- ============================================================

-- ============================================================
-- 1. ADMINISTRATIVE_UNITS — cây hành chính thống nhất
-- ============================================================
CREATE TABLE IF NOT EXISTS administrative_units (
    code        VARCHAR(10)  PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    level       VARCHAR(10)  NOT NULL CHECK (level IN ('province', 'district', 'ward')),
    parent_code VARCHAR(10)  REFERENCES administrative_units(code),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_units_level
    ON administrative_units(level);
CREATE INDEX IF NOT EXISTS idx_admin_units_parent_code
    ON administrative_units(parent_code) WHERE parent_code IS NOT NULL;

INSERT INTO administrative_units (code, name, level, parent_code)
    SELECT code, name, 'province', NULL FROM provinces
ON CONFLICT (code) DO NOTHING;

INSERT INTO administrative_units (code, name, level, parent_code)
    SELECT code, name, 'district', province_code FROM districts
ON CONFLICT (code) DO NOTHING;

INSERT INTO administrative_units (code, name, level, parent_code)
    SELECT code, name, 'ward', district_code FROM wards
ON CONFLICT (code) DO NOTHING;

COMMENT ON TABLE administrative_units IS
    'Cây hành chính thống nhất: province > district > ward.';
COMMENT ON COLUMN administrative_units.level IS 'province | district | ward';
COMMENT ON COLUMN administrative_units.parent_code IS 'NULL = cấp tỉnh (gốc cây)';

-- ============================================================
-- 2. USER_ASSIGNMENTS — phân công cán bộ vào đơn vị + lịch sử
-- ============================================================
CREATE TABLE IF NOT EXISTS user_assignments (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    unit_code   VARCHAR(10)  NOT NULL REFERENCES administrative_units(code),
    role        VARCHAR(20)  NOT NULL,
    start_date  DATE         NOT NULL DEFAULT CURRENT_DATE,
    end_date    DATE,
    note        TEXT,
    created_by  UUID         REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT user_assignments_role_check CHECK (role IN (
        'super_admin', 'national_manager', 'province_manager',
        'district_manager', 'ward_officer', 'data_entry',
        'auditor', 'analytics_viewer', 'citizen_self'
    ))
);

CREATE INDEX IF NOT EXISTS idx_user_assignments_user_id
    ON user_assignments(user_id);
CREATE INDEX IF NOT EXISTS idx_user_assignments_unit_code
    ON user_assignments(unit_code);
CREATE INDEX IF NOT EXISTS idx_user_assignments_active
    ON user_assignments(user_id, unit_code) WHERE end_date IS NULL;

-- Seed từ users hiện có — chỉ lấy user có unit_code tồn tại trong administrative_units
INSERT INTO user_assignments (user_id, unit_code, role, start_date, note)
    SELECT
        u.id,
        au.code,
        u.role,
        CURRENT_DATE,
        'Seed từ dữ liệu cũ'
    FROM users u
    JOIN administrative_units au
        ON au.code = COALESCE(u.ward_code, u.district_code, u.province_code)
    WHERE u.is_active = true
ON CONFLICT DO NOTHING;

COMMENT ON TABLE user_assignments IS
    'Lịch sử phân công cán bộ. end_date IS NULL = đang còn phụ trách.';
COMMENT ON COLUMN user_assignments.end_date IS 'NULL = đang còn phụ trách';
COMMENT ON COLUMN user_assignments.role IS
    'Snapshot role tại thời điểm phân công';

-- ============================================================
-- 3. HOUSEHOLD_MEMBERS — thêm lịch sử (left_at + is_active)
-- ============================================================
ALTER TABLE household_members DROP CONSTRAINT IF EXISTS household_members_pkey;

ALTER TABLE household_members
    ADD COLUMN IF NOT EXISTS id        UUID    DEFAULT uuid_generate_v4(),
    ADD COLUMN IF NOT EXISTS left_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'household_members_pkey'
    ) THEN
        ALTER TABLE household_members ADD PRIMARY KEY (id);
    END IF;
END$$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_household_members_active_unique
    ON household_members(household_id, citizen_id)
    WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_household_members_history
    ON household_members(citizen_id, joined_at DESC);

COMMENT ON COLUMN household_members.left_at IS
    'Thời điểm rời hộ — NULL = đang là thành viên';
COMMENT ON COLUMN household_members.is_active IS
    'true = đang là thành viên, false = đã rời (lịch sử)';

-- ============================================================
-- 4. TRANSFER_REQUESTS — snapshot đơn vị lúc tạo request
-- ============================================================
ALTER TABLE transfer_requests
    ADD COLUMN IF NOT EXISTS from_unit_code VARCHAR(10)
        REFERENCES administrative_units(code),
    ADD COLUMN IF NOT EXISTS to_unit_code VARCHAR(10)
        REFERENCES administrative_units(code);

-- Backfill: dùng FROM với 2 alias riêng biệt, không JOIN lồng nhau
-- PostgreSQL yêu cầu điều kiện join bảng phụ phải nằm trong WHERE, không trong JOIN
UPDATE transfer_requests
SET
    from_unit_code = h_from.ward_code,
    to_unit_code   = h_to.ward_code
FROM households h_from,
     households h_to
WHERE transfer_requests.from_household_id = h_from.id
  AND transfer_requests.to_household_id   = h_to.id
  AND transfer_requests.from_unit_code IS NULL;

COMMENT ON COLUMN transfer_requests.from_unit_code IS
    'Snapshot ward_code nơi đi — bất biến sau khi tạo';
COMMENT ON COLUMN transfer_requests.to_unit_code IS
    'Snapshot ward_code nơi đến — bất biến sau khi tạo';

-- ============================================================
-- 5. AUDIT_VISIBILITY — nâng cấp (bảng đã tạo ở 009)
--    Thêm: cột id, cột created_at, FK tới administrative_units,
--    index audit_id (009 chỉ có index unit_code)
-- ============================================================
ALTER TABLE audit_visibility
    ADD COLUMN IF NOT EXISTS id         UUID        DEFAULT uuid_generate_v4(),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'audit_visibility_unit_code_fkey'
    ) THEN
        ALTER TABLE audit_visibility
            ADD CONSTRAINT audit_visibility_unit_code_fkey
            FOREIGN KEY (unit_code) REFERENCES administrative_units(code);
    END IF;
END$$;

CREATE INDEX IF NOT EXISTS idx_audit_visibility_audit_id
    ON audit_visibility(audit_id);

COMMENT ON TABLE audit_visibility IS
    'Đơn vị được xem audit log. Dùng recursive CTE trên '
    'administrative_units để district thấy log của ward bên dưới.';

-- ============================================================
-- 6. PERMISSIONS + ROLE_PERMISSIONS — RBAC thực sự
-- ============================================================
CREATE TABLE IF NOT EXISTS permissions (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role          VARCHAR(20) NOT NULL,
    permission_id UUID        NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role, permission_id),
    CONSTRAINT role_permissions_role_check CHECK (role IN (
        'super_admin', 'national_manager', 'province_manager',
        'district_manager', 'ward_officer', 'data_entry',
        'auditor', 'analytics_viewer', 'citizen_self'
    ))
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions(role);

INSERT INTO permissions (code, description) VALUES
    ('citizens:read',           'Xem danh sách và chi tiết công dân'),
    ('citizens:write',          'Tạo mới công dân'),
    ('citizens:update',         'Sửa thông tin công dân'),
    ('citizens:delete',         'Xóa mềm công dân'),
    ('households:read',         'Xem thông tin hộ gia đình'),
    ('households:write',        'Tạo hộ gia đình và thêm thành viên'),
    ('transfers:read',          'Xem yêu cầu chuyển hộ khẩu'),
    ('transfers:write',         'Tạo yêu cầu chuyển hộ khẩu'),
    ('transfers:approve',       'Phê duyệt / từ chối yêu cầu chuyển hộ khẩu'),
    ('transfers:force_approve', 'Phê duyệt cưỡng bức (bypass workflow)'),
    ('audit_logs:read',         'Xem audit log'),
    ('users:manage',            'Quản lý tài khoản người dùng'),
    ('stats:read',              'Xem thống kê dân số'),
    ('assignments:manage',      'Phân công / điều chuyển cán bộ')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'super_admin', id FROM permissions
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'national_manager', id FROM permissions
    WHERE code NOT IN ('users:manage', 'assignments:manage')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'province_manager', id FROM permissions
    WHERE code IN (
        'citizens:read','citizens:write','citizens:update',
        'households:read','households:write',
        'transfers:read','transfers:write','transfers:approve',
        'stats:read'
    )
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'district_manager', id FROM permissions
    WHERE code IN (
        'citizens:read','citizens:write','citizens:update',
        'households:read','households:write',
        'transfers:read','transfers:write','transfers:approve',
        'stats:read'
    )
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'ward_officer', id FROM permissions
    WHERE code IN (
        'citizens:read','citizens:write','citizens:update',
        'households:read','households:write',
        'transfers:read','transfers:write','transfers:approve',
        'stats:read'
    )
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'data_entry', id FROM permissions
    WHERE code IN ('citizens:write', 'transfers:write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'auditor', id FROM permissions
    WHERE code IN (
        'citizens:read','households:read',
        'transfers:read','audit_logs:read','stats:read'
    )
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'analytics_viewer', id FROM permissions
    WHERE code IN ('stats:read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role, permission_id)
    SELECT 'citizen_self', id FROM permissions
    WHERE code IN ('citizens:read','transfers:read','transfers:write')
ON CONFLICT DO NOTHING;

COMMENT ON TABLE permissions      IS 'Danh sách quyền chi tiết của hệ thống';
COMMENT ON TABLE role_permissions IS 'Mapping role → permission (RBAC)';