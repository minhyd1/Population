-- ============================================================
-- Migration: 007_audit_log.sql
-- Bảng ghi lại toàn bộ thao tác chỉnh sửa thông tin công dân
-- ============================================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    citizen_id      UUID        NOT NULL,
    action          VARCHAR(20) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    changed_by      VARCHAR(255) NOT NULL,       -- user_id của người thực hiện
    changed_by_name VARCHAR(100) NOT NULL,       -- username để dễ đọc
    changed_by_role VARCHAR(50)  NOT NULL,       -- role tại thời điểm thực hiện
    old_values      JSONB,                        -- NULL với action=create
    new_values      JSONB,                        -- NULL với action=delete
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index để tra cứu nhanh theo công dân
CREATE INDEX IF NOT EXISTS idx_audit_logs_citizen_id
    ON audit_logs(citizen_id);

-- Index để tra cứu theo người thực hiện
CREATE INDEX IF NOT EXISTS idx_audit_logs_changed_by
    ON audit_logs(changed_by);

-- Index để tra cứu theo thời gian (DESC vì thường lấy log mới nhất)
CREATE INDEX IF NOT EXISTS idx_audit_logs_changed_at
    ON audit_logs(changed_at DESC);

-- Index composite để filter citizen + thời gian
CREATE INDEX IF NOT EXISTS idx_audit_logs_citizen_time
    ON audit_logs(citizen_id, changed_at DESC);

COMMENT ON TABLE audit_logs IS 'Ghi lại toàn bộ thao tác tạo/sửa/xóa thông tin công dân';
COMMENT ON COLUMN audit_logs.old_values IS 'Giá trị trước khi thay đổi (JSON). NULL với action=create';
COMMENT ON COLUMN audit_logs.new_values IS 'Giá trị sau khi thay đổi (JSON). NULL với action=delete';
COMMENT ON COLUMN audit_logs.changed_by IS 'user_id của người thực hiện thao tác';