-- ============================================================
-- Migration: 002_seed_data.sql
-- Dữ liệu mẫu - Provinces + Districts + Wards + Citizens
-- ============================================================

-- Provinces (một số tỉnh thành Việt Nam)
INSERT INTO provinces (code, name, name_en) VALUES
    ('01', 'Hà Nội',         'Hanoi'),
    ('79', 'TP. Hồ Chí Minh','Ho Chi Minh City'),
    ('48', 'Đà Nẵng',        'Da Nang'),
    ('46', 'Thừa Thiên Huế', 'Thua Thien Hue'),
    ('56', 'Khánh Hòa',      'Khanh Hoa'),
    ('92', 'Cần Thơ',        'Can Tho')
ON CONFLICT (code) DO NOTHING;

-- Districts Hà Nội
INSERT INTO districts (code, name, province_code) VALUES
    ('001', 'Quận Ba Đình',       '01'),
    ('002', 'Quận Hoàn Kiếm',     '01'),
    ('003', 'Quận Tây Hồ',        '01'),
    ('004', 'Quận Long Biên',     '01')
ON CONFLICT (code) DO NOTHING;

-- Districts HCM
INSERT INTO districts (code, name, province_code) VALUES
    ('760', 'Quận 1',             '79'),
    ('761', 'Quận 3',             '79'),
    ('762', 'Quận Bình Thạnh',    '79'),
    ('763', 'Quận Tân Bình',      '79')
ON CONFLICT (code) DO NOTHING;

-- Districts Đà Nẵng
INSERT INTO districts (code, name, province_code) VALUES
    ('490', 'Quận Hải Châu',      '48'),
    ('491', 'Quận Thanh Khê',     '48')
ON CONFLICT (code) DO NOTHING;

-- Districts Cần Thơ
INSERT INTO districts (code, name, province_code) VALUES
    ('900', 'Quận Ninh Kiều',     '92'),
    ('901', 'Quận Bình Thuỷ',     '92')
ON CONFLICT (code) DO NOTHING;

-- Wards Ba Đình, HN
INSERT INTO wards (code, name, district_code) VALUES
    ('00001', 'Phường Phúc Xá',    '001'),
    ('00002', 'Phường Trúc Bạch',  '001'),
    ('00003', 'Phường Vĩnh Phúc',  '001')
ON CONFLICT (code) DO NOTHING;

-- Wards Hoàn Kiếm, HN
INSERT INTO wards (code, name, district_code) VALUES
    ('00010', 'Phường Phan Chu Trinh', '002'),
    ('00011', 'Phường Hàng Bài',       '002')
ON CONFLICT (code) DO NOTHING;

-- Wards Q1, HCM
INSERT INTO wards (code, name, district_code) VALUES
    ('26734', 'Phường Bến Nghé',   '760'),
    ('26736', 'Phường Bến Thành',  '760'),
    ('26737', 'Phường Nguyễn Thái Bình', '760')
ON CONFLICT (code) DO NOTHING;

-- Wards Hải Châu, Đà Nẵng
INSERT INTO wards (code, name, district_code) VALUES
    ('20194', 'Phường Hải Châu 1', '490'),
    ('20195', 'Phường Hải Châu 2', '490')
ON CONFLICT (code) DO NOTHING;

-- Wards Ninh Kiều, Cần Thơ
INSERT INTO wards (code, name, district_code) VALUES
    ('31150', 'Phường Tân An',     '900'),
    ('31151', 'Phường Xuân Khánh', '900')
ON CONFLICT (code) DO NOTHING;

-- ============================================================
-- Citizens mẫu
-- LƯU Ý: Trong thực tế, national_id/phone/email/permanent_address
-- phải được mã hóa AES-256-GCM trước khi INSERT.
-- Dưới đây là placeholder cho seed - trong production script
-- sẽ chạy qua encryptor trước khi INSERT.
-- ============================================================
-- Dùng script Go để seed với mã hóa thực, xem scripts/seed.go
