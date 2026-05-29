-- ============================================================
-- Migration: 004_auth_tables.sql
-- Thêm bảng users và refresh_tokens cho JWT authentication
-- ============================================================

-- ============================================================
-- USERS (Tài khoản đăng nhập)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    username      VARCHAR(50)  NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,                  -- bcrypt hash
    role          VARCHAR(20)  NOT NULL DEFAULT 'citizen'
                               CHECK (role IN ('admin', 'citizen')),
    citizen_id    UUID         REFERENCES citizens(id),   -- NULL nếu là admin
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username   ON users(username);
CREATE INDEX idx_users_citizen_id ON users(citizen_id) WHERE citizen_id IS NOT NULL;

-- ============================================================
-- REFRESH_TOKENS (Lưu hash của refresh token để revoke được)
-- ============================================================
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE, -- SHA-256 hash, không lưu plaintext
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ             -- NULL = còn hiệu lực
);

CREATE INDEX idx_refresh_tokens_user_id    ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_revoked    ON refresh_tokens(revoked_at) WHERE revoked_at IS NULL;

-- Auto-update updated_at cho users
CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- Seed: Tạo tài khoản admin mặc định
-- Password: Admin@123 (bcrypt cost 10)
-- ĐỔI PASSWORD NGAY SAU KHI DEPLOY!
-- ============================================================
INSERT INTO users (id, username, password_hash, role, is_active)
VALUES (
    uuid_generate_v4(),
    'admin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- Admin@123
    'admin',
    true
) ON CONFLICT (username) DO NOTHING;

COMMENT ON TABLE users IS 'Tài khoản đăng nhập hệ thống';
COMMENT ON COLUMN users.citizen_id IS 'Liên kết với bảng citizens nếu role = citizen';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'SHA-256 hash của refresh token, không lưu plaintext';