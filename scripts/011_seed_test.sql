-- ============================================================
-- Migration: 011_seed_test_data.sql
-- Seed dữ liệu test đầy đủ cho tất cả bảng còn trống:
--   households, household_members, transfer_requests,
--   transfer_approvals, residence_histories,
--   audit_logs, audit_visibility, user_assignments
--
-- Tất cả field ENCRYPTED dùng placeholder base64 hợp lệ
-- (trong production phải mã hoá AES-256-GCM thật sự)
-- ============================================================

-- ============================================================
-- BƯỚC 1: HOUSEHOLDS — 10 hộ gia đình trải đều các tỉnh
-- address là placeholder base64 (thay bằng AES encrypted thật)
-- ============================================================
INSERT INTO households (id, household_no, province_code, district_code, ward_code, address, head_citizen_id)
VALUES
    -- Hà Nội — Ba Đình
    ('a1000000-0000-0000-0000-000000000001', 'HN-001-2024', '01', '001', '00001',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),
    ('a1000000-0000-0000-0000-000000000002', 'HN-002-2024', '01', '001', '00001',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),
    ('a1000000-0000-0000-0000-000000000003', 'HN-003-2024', '01', '001', '00002',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),

    -- Hà Nội — Hoàn Kiếm
    ('a1000000-0000-0000-0000-000000000004', 'HN-004-2024', '01', '002', '00010',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),

    -- TP.HCM — Quận 1
    ('a1000000-0000-0000-0000-000000000005', 'HCM-001-2024', '79', '760', '26734',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),
    ('a1000000-0000-0000-0000-000000000006', 'HCM-002-2024', '79', '760', '26734',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),
    ('a1000000-0000-0000-0000-000000000007', 'HCM-003-2024', '79', '760', '26736',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),

    -- Đà Nẵng — Hải Châu
    ('a1000000-0000-0000-0000-000000000008', 'DN-001-2024', '48', '490', '20194',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),

    -- Cần Thơ — Ninh Kiều
    ('a1000000-0000-0000-0000-000000000009', 'CT-001-2024', '92', '900', '31150',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL),
    ('a1000000-0000-0000-0000-000000000010', 'CT-002-2024', '92', '900', '31151',
     'dGVzdC1lbmNyeXB0ZWQtYWRkcmVzcw==', NULL)
ON CONFLICT (household_no) DO NOTHING;

-- ============================================================
-- BƯỚC 2: GẮN HEAD_CITIZEN vào households từ citizens có sẵn
-- Dùng subquery lấy citizen thuộc đúng ward
-- ============================================================
UPDATE households h
SET head_citizen_id = (
    SELECT c.id FROM citizens c
    WHERE c.ward_code = h.ward_code
      AND c.deleted_at IS NULL
    ORDER BY c.created_at ASC
    LIMIT 1
)
WHERE h.head_citizen_id IS NULL;

-- ============================================================
-- BƯỚC 3: HOUSEHOLD_MEMBERS — mỗi hộ 2-3 thành viên
-- Dùng citizens có sẵn trong đúng ward, tránh trùng
-- ============================================================
INSERT INTO household_members (id, household_id, citizen_id, relationship, joined_at, is_active)
SELECT
    uuid_generate_v4(),
    h.id,
    c.id,
    CASE ROW_NUMBER() OVER (PARTITION BY h.id ORDER BY c.created_at)
        WHEN 1 THEN 'chủ hộ'
        WHEN 2 THEN 'vợ/chồng'
        ELSE        'con'
    END,
    NOW() - INTERVAL '1 year',
    TRUE
FROM households h
JOIN citizens c ON c.ward_code = h.ward_code
               AND c.deleted_at IS NULL
WHERE (
    SELECT COUNT(*) FROM household_members hm2
    WHERE hm2.household_id = h.id AND hm2.is_active = TRUE
) < 3
ON CONFLICT DO NOTHING;

-- ============================================================
-- BƯỚC 4: USER_ASSIGNMENTS — phân công cán bộ vào đơn vị
-- Seed cho tất cả users có ward/district/province_code (vẫn còn cột cũ)
-- ============================================================
INSERT INTO user_assignments (id, user_id, unit_code, role, start_date, note)
SELECT
    uuid_generate_v4(),
    u.id,
    au.code,
    u.role,
    CURRENT_DATE - INTERVAL '6 months',
    'Phân công ban đầu'
FROM users u
JOIN administrative_units au
    ON au.code = COALESCE(u.ward_code, u.district_code, u.province_code)
WHERE u.is_active = TRUE
  AND COALESCE(u.ward_code, u.district_code, u.province_code) IS NOT NULL
ON CONFLICT DO NOTHING;

-- ============================================================
-- BƯỚC 5: TRANSFER_REQUESTS — 6 yêu cầu chuyển hộ khẩu mẫu
-- Bao phủ các kịch bản: pending, approved, completed, rejected
-- ============================================================

-- Lấy IDs citizen từ household_members để dùng làm dữ liệu mẫu
WITH member_data AS (
    SELECT
        hm.citizen_id,
        hm.household_id AS from_hh,
        ROW_NUMBER() OVER (ORDER BY hm.joined_at) AS rn
    FROM household_members hm
    WHERE hm.is_active = TRUE
),
creator AS (
    SELECT id FROM users WHERE username = 'wo_phucxa' LIMIT 1
)
INSERT INTO transfer_requests
    (id, citizen_id, from_household_id, to_household_id,
     from_unit_code, to_unit_code,
     approval_level, status, reason,
     created_by, deadline_at, lock_version)
SELECT
    req_id,
    citizen_id,
    from_hh,
    to_hh,
    from_unit,
    to_unit,
    approval_level,
    status,
    reason,
    (SELECT id FROM creator),
    CASE WHEN status = 'pending' THEN NOW() + INTERVAL '7 days' ELSE NULL END,
    0
FROM (VALUES
    -- 1. Cùng phường Phúc Xá → pending
    ('b1000000-0000-0000-0000-000000000001'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=1),
     'a1000000-0000-0000-0000-000000000001'::uuid,
     'a1000000-0000-0000-0000-000000000002'::uuid,
     '00001', '00001', 'ward', 'pending',
     'Chuyển sang nhà mới trong cùng phường'),

    -- 2. Phúc Xá → Trúc Bạch (khác phường cùng quận) → pending
    ('b1000000-0000-0000-0000-000000000002'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=2),
     'a1000000-0000-0000-0000-000000000002'::uuid,
     'a1000000-0000-0000-0000-000000000003'::uuid,
     '00001', '00002', 'district', 'pending',
     'Chuyển sang phường Trúc Bạch do thay đổi nơi ở'),

    -- 3. Ba Đình → Hoàn Kiếm (cùng quận) → approved
    ('b1000000-0000-0000-0000-000000000003'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=3),
     'a1000000-0000-0000-0000-000000000003'::uuid,
     'a1000000-0000-0000-0000-000000000004'::uuid,
     '00002', '00010', 'district', 'approved',
     'Chuyển về Hoàn Kiếm gần nơi làm việc'),

    -- 4. Hà Nội → TP.HCM (khác tỉnh) → completed
    ('b1000000-0000-0000-0000-000000000004'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=4),
     'a1000000-0000-0000-0000-000000000004'::uuid,
     'a1000000-0000-0000-0000-000000000005'::uuid,
     '00010', '26734', 'province', 'completed',
     'Chuyển vào TP.HCM theo công việc'),

    -- 5. HCM Q1 → Hải Châu Đà Nẵng (khác tỉnh) → rejected
    ('b1000000-0000-0000-0000-000000000005'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=5),
     'a1000000-0000-0000-0000-000000000005'::uuid,
     'a1000000-0000-0000-0000-000000000008'::uuid,
     '26734', '20194', 'province', 'rejected',
     'Chuyển về Đà Nẵng theo gia đình'),

    -- 6. Cần Thơ nội bộ → pending (để test escalation)
    ('b1000000-0000-0000-0000-000000000006'::uuid,
     (SELECT citizen_id FROM member_data WHERE rn=6),
     'a1000000-0000-0000-0000-000000000009'::uuid,
     'a1000000-0000-0000-0000-000000000010'::uuid,
     '31150', '31151', 'ward', 'pending',
     'Chuyển sang phường Xuân Khánh')
) AS t(req_id, citizen_id, from_hh, to_hh, from_unit, to_unit,
       approval_level, status, reason)
WHERE t.citizen_id IS NOT NULL  -- Bỏ qua nếu member_data rỗng
ON CONFLICT (id) DO NOTHING;

-- Cập nhật completed_at cho request đã hoàn thành
UPDATE transfer_requests
SET completed_at = NOW() - INTERVAL '10 days'
WHERE id = 'b1000000-0000-0000-0000-000000000004'
  AND status = 'completed';

-- ============================================================
-- BƯỚC 6: TRANSFER_APPROVALS — phiếu duyệt cho từng request
-- ============================================================
INSERT INTO transfer_approvals (id, request_id, unit_code, unit_role, decision, approved_by, approved_at, reject_reason)
VALUES
    -- Request 1 (ward, pending) → phường nguồn chưa duyệt
    ('c1000000-0000-0000-0000-000000000001', 'b1000000-0000-0000-0000-000000000001',
     '00001', 'source', 'pending', NULL, NULL, NULL),

    -- Request 2 (district, pending) → 2 phường + quận
    ('c1000000-0000-0000-0000-000000000002', 'b1000000-0000-0000-0000-000000000002',
     '00001', 'source', 'pending', NULL, NULL, NULL),
    ('c1000000-0000-0000-0000-000000000003', 'b1000000-0000-0000-0000-000000000002',
     '00002', 'destination', 'pending', NULL, NULL, NULL),

    -- Request 3 (district, approved) → cả 2 đã duyệt
    ('c1000000-0000-0000-0000-000000000004', 'b1000000-0000-0000-0000-000000000003',
     '00002', 'source',
     'approved',
     (SELECT id FROM users WHERE username = 'wo_trucbach' LIMIT 1),
     NOW() - INTERVAL '5 days', NULL),
    ('c1000000-0000-0000-0000-000000000005', 'b1000000-0000-0000-0000-000000000003',
     '00010', 'destination',
     'approved',
     (SELECT id FROM users WHERE username = 'wo_phanchutrinh' LIMIT 1),
     NOW() - INTERVAL '4 days', NULL),

    -- Request 4 (province, completed) → tất cả đã duyệt
    ('c1000000-0000-0000-0000-000000000006', 'b1000000-0000-0000-0000-000000000004',
     '00010', 'source',      'approved',
     (SELECT id FROM users WHERE username = 'wo_phanchutrinh' LIMIT 1),
     NOW() - INTERVAL '20 days', NULL),
    ('c1000000-0000-0000-0000-000000000007', 'b1000000-0000-0000-0000-000000000004',
     '26734', 'destination', 'approved',
     (SELECT id FROM users WHERE username = 'wo_bennghe' LIMIT 1),
     NOW() - INTERVAL '18 days', NULL),

    -- Request 5 (province, rejected) → phường nguồn từ chối
    ('c1000000-0000-0000-0000-000000000008', 'b1000000-0000-0000-0000-000000000005',
     '26734', 'source',      'rejected',
     (SELECT id FROM users WHERE username = 'wo_bennghe' LIMIT 1),
     NOW() - INTERVAL '3 days',
     'Hồ sơ chưa đủ điều kiện, thiếu giấy xác nhận nơi ở mới'),
    ('c1000000-0000-0000-0000-000000000009', 'b1000000-0000-0000-0000-000000000005',
     '20194', 'destination', 'pending', NULL, NULL, NULL),

    -- Request 6 (ward, pending) → chưa có phiếu duyệt
    ('c1000000-0000-0000-0000-000000000010', 'b1000000-0000-0000-0000-000000000006',
     '31150', 'source',      'pending', NULL, NULL, NULL),
    ('c1000000-0000-0000-0000-000000000011', 'b1000000-0000-0000-0000-000000000006',
     '31151', 'destination', 'pending', NULL, NULL, NULL)
ON CONFLICT (request_id, unit_code) DO NOTHING;

-- ============================================================
-- BƯỚC 7: RESIDENCE_HISTORIES — lịch sử cư trú
-- ============================================================
INSERT INTO residence_histories
    (id, citizen_id, from_household_id, to_household_id, transfer_request_id, reason, effective_date)
VALUES
    -- Lịch sử đăng ký hộ khẩu ban đầu (không có from_hh)
    ('d1000000-0000-0000-0000-000000000001',
     (SELECT citizen_id FROM household_members WHERE household_id='a1000000-0000-0000-0000-000000000001' AND is_active=TRUE ORDER BY joined_at LIMIT 1),
     NULL, 'a1000000-0000-0000-0000-000000000001', NULL,
     'Đăng ký hộ khẩu lần đầu', NOW() - INTERVAL '2 years'),

    -- Lịch sử chuyển thành công (request 4 = completed)
    ('d1000000-0000-0000-0000-000000000002',
     (SELECT citizen_id FROM transfer_requests WHERE id='b1000000-0000-0000-0000-000000000004' LIMIT 1),
     'a1000000-0000-0000-0000-000000000004',
     'a1000000-0000-0000-0000-000000000005',
     'b1000000-0000-0000-0000-000000000004',
     'Chuyển vào TP.HCM theo công việc', NOW() - INTERVAL '10 days')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- BƯỚC 8: AUDIT_LOGS — lịch sử thao tác hệ thống
-- Schema thực tế (007_audit_log.sql):
--   id, citizen_id, action (create/update/delete),
--   changed_by (user_id text), changed_by_name, changed_by_role,
--   old_values, new_values, changed_at
-- ============================================================
INSERT INTO audit_logs
    (id, citizen_id, action,
     changed_by, changed_by_name, changed_by_role,
     old_values, new_values, changed_at)
VALUES
    -- Tạo citizen mới (CREATE)
    ('e1000000-0000-0000-0000-000000000001',
     (SELECT id FROM citizens WHERE ward_code='00001' AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
     'create',
     (SELECT id::text FROM users WHERE username='wo_phucxa' LIMIT 1),
     'wo_phucxa', 'ward_officer',
     NULL,
     '{"ward_code":"00001","province_code":"01"}',
     NOW() - INTERVAL '1 year'),

    -- Cập nhật thông tin citizen (UPDATE)
    ('e1000000-0000-0000-0000-000000000002',
     (SELECT id FROM citizens WHERE ward_code='00001' AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
     'update',
     (SELECT id::text FROM users WHERE username='wo_phucxa' LIMIT 1),
     'wo_phucxa', 'ward_officer',
     '{"phone":"0900000000"}',
     '{"phone":"0911111111"}',
     NOW() - INTERVAL '6 months'),

    -- Cập nhật citizen ở phường Trúc Bạch (UPDATE)
    ('e1000000-0000-0000-0000-000000000003',
     (SELECT id FROM citizens WHERE ward_code='00002' AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
     'update',
     (SELECT id::text FROM users WHERE username='wo_trucbach' LIMIT 1),
     'wo_trucbach', 'ward_officer',
     '{"address":"Số 1 Trúc Bạch"}',
     '{"address":"Số 5 Trúc Bạch"}',
     NOW() - INTERVAL '5 days'),

    -- Xóa mềm citizen (DELETE)
    ('e1000000-0000-0000-0000-000000000004',
     (SELECT id FROM citizens WHERE ward_code='00001' AND deleted_at IS NULL ORDER BY created_at OFFSET 1 LIMIT 1),
     'delete',
     (SELECT id::text FROM users WHERE username='super_admin' LIMIT 1),
     'super_admin', 'super_admin',
     '{"ward_code":"00001","status":"active"}',
     NULL,
     NOW() - INTERVAL '3 days'),

    -- Tạo citizen mới tại HCM (CREATE)
    ('e1000000-0000-0000-0000-000000000005',
     (SELECT id FROM citizens WHERE ward_code='26734' AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
     'create',
     (SELECT id::text FROM users WHERE username='wo_bennghe' LIMIT 1),
     'wo_bennghe', 'ward_officer',
     NULL,
     '{"ward_code":"26734","province_code":"79"}',
     NOW() - INTERVAL '2 days'),

    -- Kiểm tra và cập nhật bởi auditor (UPDATE)
    ('e1000000-0000-0000-0000-000000000006',
     (SELECT id FROM citizens WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1),
     'update',
     (SELECT id::text FROM users WHERE username='auditor1' LIMIT 1),
     'auditor1', 'auditor',
     '{"note":""}',
     '{"note":"Đã kiểm tra"}',
     NOW() - INTERVAL '1 day')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- BƯỚC 9: AUDIT_VISIBILITY — đơn vị được xem từng audit log
-- ============================================================
INSERT INTO audit_visibility (audit_id, unit_code)
VALUES
    -- Transfer tạo ở phường 00001 → ward + district + province đều thấy
    ('e1000000-0000-0000-0000-000000000003', '00001'),  -- phường Phúc Xá
    ('e1000000-0000-0000-0000-000000000003', '001'),    -- quận Ba Đình
    ('e1000000-0000-0000-0000-000000000003', '01'),     -- tỉnh Hà Nội

    -- Approve ở phường 00002 → ward + district + province đều thấy
    ('e1000000-0000-0000-0000-000000000004', '00002'),
    ('e1000000-0000-0000-0000-000000000004', '001'),
    ('e1000000-0000-0000-0000-000000000004', '01'),

    -- Reject ở phường 26734 HCM
    ('e1000000-0000-0000-0000-000000000005', '26734'),
    ('e1000000-0000-0000-0000-000000000005', '760'),
    ('e1000000-0000-0000-0000-000000000005', '79'),

    -- CREATE citizen ở phường 00001
    ('e1000000-0000-0000-0000-000000000006', '00001'),
    ('e1000000-0000-0000-0000-000000000006', '001'),
    ('e1000000-0000-0000-0000-000000000006', '01')
ON CONFLICT (audit_id, unit_code) DO NOTHING;

-- ============================================================
-- KIỂM TRA KẾT QUẢ
-- ============================================================
DO $$
DECLARE
    v_households        INT;
    v_hh_members        INT;
    v_assignments       INT;
    v_transfers         INT;
    v_approvals         INT;
    v_histories         INT;
    v_audit_logs        INT;
    v_audit_visibility  INT;
BEGIN
    SELECT COUNT(*) INTO v_households       FROM households;
    SELECT COUNT(*) INTO v_hh_members       FROM household_members WHERE is_active=TRUE;
    SELECT COUNT(*) INTO v_assignments      FROM user_assignments;
    SELECT COUNT(*) INTO v_transfers        FROM transfer_requests;
    SELECT COUNT(*) INTO v_approvals        FROM transfer_approvals;
    SELECT COUNT(*) INTO v_histories        FROM residence_histories;
    SELECT COUNT(*) INTO v_audit_logs       FROM audit_logs;
    SELECT COUNT(*) INTO v_audit_visibility FROM audit_visibility;

    RAISE NOTICE '=== Seed kết quả ===';
    RAISE NOTICE 'households:        %', v_households;
    RAISE NOTICE 'household_members: %', v_hh_members;
    RAISE NOTICE 'user_assignments:  %', v_assignments;
    RAISE NOTICE 'transfer_requests: %', v_transfers;
    RAISE NOTICE 'transfer_approvals:%', v_approvals;
    RAISE NOTICE 'residence_histories:%', v_histories;
    RAISE NOTICE 'audit_logs:        %', v_audit_logs;
    RAISE NOTICE 'audit_visibility:  %', v_audit_visibility;
END$$;