-- ============================================================
-- Migration: 009_transfer_fixes.sql
-- Fix 6 vấn đề nghiệp vụ chuyển hộ khẩu
-- ============================================================

-- ============================================================
-- VẤN ĐỀ 2: Audit Visibility
-- Bảng audit_visibility lưu các unit_code được phép xem audit log
-- của một operation (ví dụ: chuyển hộ từ Ward A sang Ward B
-- thì Ward A, Ward B, District A, District B, Province A đều thấy)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_visibility (
    audit_id   UUID         NOT NULL REFERENCES audit_logs(id) ON DELETE CASCADE,
    unit_code  VARCHAR(20)  NOT NULL,
    PRIMARY KEY (audit_id, unit_code)
);

CREATE INDEX IF NOT EXISTS idx_audit_visibility_unit_code
    ON audit_visibility(unit_code);

COMMENT ON TABLE audit_visibility IS
    'Danh sách đơn vị hành chính được phép xem từng audit log entry';

-- ============================================================
-- VẤN ĐỀ 4: Escalation
-- Thêm deadline_at và escalated_to vào transfer_requests
-- ============================================================
ALTER TABLE transfer_requests
    ADD COLUMN IF NOT EXISTS deadline_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS escalated_to  VARCHAR(20);   -- unit_code cấp trên sau khi escalate

CREATE INDEX IF NOT EXISTS idx_transfer_requests_deadline
    ON transfer_requests(deadline_at)
    WHERE status = 'pending' AND deadline_at IS NOT NULL;

COMMENT ON COLUMN transfer_requests.deadline_at  IS
    '7 ngày sau khi tạo — quá hạn sẽ escalate lên cấp trên';
COMMENT ON COLUMN transfer_requests.escalated_to IS
    'unit_code cấp trên đã nhận escalate (NULL = chưa escalate)';

-- ============================================================
-- VẤN ĐỀ 6: Pessimistic Lock support
-- Thêm version column để hỗ trợ optimistic locking nếu cần
-- (SELECT FOR UPDATE sẽ được thực hiện ở tầng Repository)
-- ============================================================
ALTER TABLE transfer_requests
    ADD COLUMN IF NOT EXISTS lock_version  INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN transfer_requests.lock_version IS
    'Optimistic lock version — tăng mỗi khi row được update';
