-- ============================================================
-- Migration: 008_household_transfer.sql
-- Thêm quản lý hộ gia đình và workflow chuyển hộ khẩu
-- ============================================================

-- ============================================================
-- HOUSEHOLDS (Hộ gia đình)
-- address được mã hóa AES-256-GCM
-- ============================================================
CREATE TABLE IF NOT EXISTS households (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    household_no    VARCHAR(50)  NOT NULL UNIQUE, -- Số sổ hộ khẩu
    province_code   VARCHAR(10)  NOT NULL REFERENCES provinces(code),
    district_code   VARCHAR(10)  NOT NULL REFERENCES districts(code),
    ward_code       VARCHAR(10)  NOT NULL REFERENCES wards(code),
    address         TEXT         NOT NULL,         -- ⚠️ ENCRYPTED (AES-256-GCM)
    head_citizen_id UUID         REFERENCES citizens(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_households_ward_code     ON households(ward_code)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_households_district_code ON households(district_code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_households_province_code ON households(province_code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_households_head          ON households(head_citizen_id) WHERE head_citizen_id IS NOT NULL AND deleted_at IS NULL;

CREATE TRIGGER trigger_households_updated_at
    BEFORE UPDATE ON households
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- HOUSEHOLD_MEMBERS (Thành viên hộ gia đình)
-- ============================================================
CREATE TABLE IF NOT EXISTS household_members (
    household_id  UUID        NOT NULL REFERENCES households(id),
    citizen_id    UUID        NOT NULL REFERENCES citizens(id),
    relationship  VARCHAR(50) NOT NULL,  -- chủ hộ, vợ/chồng, con, cha/mẹ, thành viên, ...
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (household_id, citizen_id)
);

CREATE INDEX IF NOT EXISTS idx_household_members_citizen ON household_members(citizen_id);

-- ============================================================
-- TRANSFER_REQUESTS (Yêu cầu chuyển hộ khẩu)
-- Lưu theo đơn vị hành chính (unit_code), không theo cá nhân cán bộ
-- → Đổi người, nghỉ việc: workflow vẫn sống
-- ============================================================
CREATE TABLE IF NOT EXISTS transfer_requests (
    id                UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    citizen_id        UUID         NOT NULL REFERENCES citizens(id),
    from_household_id UUID         NOT NULL REFERENCES households(id),
    to_household_id   UUID         NOT NULL REFERENCES households(id),
    approval_level    VARCHAR(20)  NOT NULL CHECK (approval_level IN ('none', 'ward', 'district', 'province')),
    status            VARCHAR(20)  NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending', 'approved', 'rejected', 'completed', 'cancelled')),
    reason            TEXT         NOT NULL,
    created_by        UUID         REFERENCES users(id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_transfer_requests_citizen    ON transfer_requests(citizen_id);
CREATE INDEX IF NOT EXISTS idx_transfer_requests_status     ON transfer_requests(status);
CREATE INDEX IF NOT EXISTS idx_transfer_requests_from_hh    ON transfer_requests(from_household_id);
CREATE INDEX IF NOT EXISTS idx_transfer_requests_to_hh      ON transfer_requests(to_household_id);
CREATE INDEX IF NOT EXISTS idx_transfer_requests_created_at ON transfer_requests(created_at DESC);

CREATE TRIGGER trigger_transfer_requests_updated_at
    BEFORE UPDATE ON transfer_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- TRANSFER_APPROVALS (Phiếu phê duyệt theo đơn vị hành chính)
-- unit_code = mã phường/quận/tỉnh → bền vững với thay đổi nhân sự
-- ============================================================
CREATE TABLE IF NOT EXISTS transfer_approvals (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id    UUID         NOT NULL REFERENCES transfer_requests(id) ON DELETE CASCADE,
    unit_code     VARCHAR(20)  NOT NULL,   -- mã đơn vị hành chính
    unit_role     VARCHAR(20)  NOT NULL CHECK (unit_role IN ('source', 'destination')),
    decision      VARCHAR(20)  NOT NULL DEFAULT 'pending'
                               CHECK (decision IN ('pending', 'approved', 'rejected')),
    approved_by   UUID         REFERENCES users(id),
    reject_reason TEXT,
    approved_at   TIMESTAMPTZ,
    UNIQUE (request_id, unit_code)
);

CREATE INDEX IF NOT EXISTS idx_transfer_approvals_request_id ON transfer_approvals(request_id);
CREATE INDEX IF NOT EXISTS idx_transfer_approvals_unit_code  ON transfer_approvals(unit_code);
CREATE INDEX IF NOT EXISTS idx_transfer_approvals_pending    ON transfer_approvals(request_id, unit_code) WHERE decision = 'pending';

-- ============================================================
-- RESIDENCE_HISTORIES (Lịch sử cư trú)
-- Bất biến — chỉ INSERT, không UPDATE/DELETE
-- ============================================================
CREATE TABLE IF NOT EXISTS residence_histories (
    id                  UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    citizen_id          UUID         NOT NULL REFERENCES citizens(id),
    from_household_id   UUID         REFERENCES households(id), -- NULL = đăng ký lần đầu
    to_household_id     UUID         NOT NULL REFERENCES households(id),
    transfer_request_id UUID         REFERENCES transfer_requests(id), -- NULL = đăng ký mới
    reason              TEXT         NOT NULL,
    effective_date      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_residence_histories_citizen    ON residence_histories(citizen_id);
CREATE INDEX IF NOT EXISTS idx_residence_histories_effective  ON residence_histories(citizen_id, effective_date DESC);
CREATE INDEX IF NOT EXISTS idx_residence_histories_request_id ON residence_histories(transfer_request_id) WHERE transfer_request_id IS NOT NULL;

-- Comments
COMMENT ON TABLE households          IS 'Hộ gia đình — đơn vị quản lý hộ khẩu';
COMMENT ON TABLE household_members   IS 'Thành viên hộ gia đình';
COMMENT ON TABLE transfer_requests   IS 'Yêu cầu chuyển hộ khẩu — lưu theo đơn vị, không theo cán bộ';
COMMENT ON TABLE transfer_approvals  IS 'Phiếu phê duyệt theo unit_code — bền vững với thay đổi nhân sự';
COMMENT ON TABLE residence_histories IS 'Lịch sử cư trú — chỉ INSERT (immutable audit trail)';

COMMENT ON COLUMN transfer_approvals.unit_code  IS 'Mã đơn vị hành chính (ward/district/province code)';
COMMENT ON COLUMN transfer_approvals.unit_role  IS 'source = nơi đi, destination = nơi đến';
COMMENT ON COLUMN households.address            IS 'Địa chỉ cụ thể — ENCRYPTED with AES-256-GCM, stored as base64';