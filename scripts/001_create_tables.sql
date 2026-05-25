-- ============================================================
-- Migration: 001_create_tables.sql
-- Population Service - PostgreSQL Schema
-- ============================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- PROVINCES (Tỉnh/Thành phố)
-- ============================================================
CREATE TABLE IF NOT EXISTS provinces (
    code        VARCHAR(10)  PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ============================================================
-- DISTRICTS (Quận/Huyện)
-- ============================================================
CREATE TABLE IF NOT EXISTS districts (
    code          VARCHAR(10)  PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    province_code VARCHAR(10)  NOT NULL REFERENCES provinces(code),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ============================================================
-- WARDS (Phường/Xã)
-- ============================================================
CREATE TABLE IF NOT EXISTS wards (
    code          VARCHAR(10)  PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    district_code VARCHAR(10)  NOT NULL REFERENCES districts(code),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ============================================================
-- CITIZENS (Công dân)
-- Lưu ý bảo mật:
--   - national_id, phone_number, email, permanent_address
--     được mã hóa AES-256-GCM (base64) TRƯỚC KHI lưu vào DB
--   - Không bao giờ lưu plaintext CCCD/CMND vào DB
-- ============================================================
CREATE TABLE IF NOT EXISTS citizens (
    id                UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    full_name         VARCHAR(200) NOT NULL,
    date_of_birth     DATE         NOT NULL,
    gender            VARCHAR(10)  NOT NULL CHECK (gender IN ('male', 'female', 'other')),

    -- ⚠️  ENCRYPTED FIELDS (AES-256-GCM, stored as base64)
    national_id       TEXT         NOT NULL UNIQUE,       -- CCCD 12 số / CMND 9 số (encrypted)
    phone_number      TEXT,                         -- SĐT (encrypted)
    email             TEXT,                         -- Email (encrypted)
    permanent_address TEXT         NOT NULL,        -- Địa chỉ thường trú (encrypted)

    -- Plain fields (không nhạy cảm)
    religion          VARCHAR(50),
    ethnicity         VARCHAR(50),
    marital_status    VARCHAR(20)  NOT NULL CHECK (marital_status IN ('single', 'married', 'divorced', 'widowed')),
    province_code     VARCHAR(10)  NOT NULL REFERENCES provinces(code),
    district_code     VARCHAR(10)  NOT NULL REFERENCES districts(code),
    ward_code         VARCHAR(10)  NOT NULL REFERENCES wards(code),
    is_alive          BOOLEAN      NOT NULL DEFAULT TRUE,

    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ                      -- soft delete
);

-- Indexes
CREATE INDEX idx_citizens_province_code  ON citizens(province_code) WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_district_code  ON citizens(district_code) WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_ward_code      ON citizens(ward_code)     WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_gender         ON citizens(gender)         WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_marital_status ON citizens(marital_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_is_alive       ON citizens(is_alive)       WHERE deleted_at IS NULL;
CREATE INDEX idx_citizens_full_name      ON citizens USING gin(to_tsvector('simple', full_name));
CREATE INDEX idx_citizens_deleted_at     ON citizens(deleted_at);

-- ============================================================
-- TRIGGER: auto update updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_citizens_updated_at
    BEFORE UPDATE ON citizens
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN citizens.national_id      IS 'CCCD/CMND - ENCRYPTED with AES-256-GCM, stored as base64';
COMMENT ON COLUMN citizens.phone_number     IS 'Số điện thoại - ENCRYPTED with AES-256-GCM, stored as base64';
COMMENT ON COLUMN citizens.email            IS 'Email - ENCRYPTED with AES-256-GCM, stored as base64';
COMMENT ON COLUMN citizens.permanent_address IS 'Địa chỉ thường trú - ENCRYPTED with AES-256-GCM, stored as base64';
