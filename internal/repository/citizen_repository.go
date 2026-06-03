package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"population-service/internal/model"

	"github.com/jmoiron/sqlx"
)

// CitizenRepository định nghĩa interface cho citizen repository
type CitizenRepository interface {
	Create(ctx context.Context, c *model.Citizen) error
	GetByID(ctx context.Context, id string) (*model.Citizen, error)
	Update(ctx context.Context, c *model.Citizen) error
	SoftDelete(ctx context.Context, id string) error
	List(ctx context.Context, filter model.ListCitizenFilter) ([]*model.Citizen, int64, error)
	GetPopulationStatsByProvince(ctx context.Context) ([]*model.PopulationStat, error)
	GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStat, error)
	ExistsByNationalID(ctx context.Context, nationalID string, excludeID string) (bool, error)
	// UpdateResidence cập nhật địa bàn cư trú sau khi chuyển hộ khẩu
	UpdateResidence(ctx context.Context, citizenID, provinceCode, districtCode, wardCode string) error
}

type citizenRepo struct {
	db *sqlx.DB
}

// NewCitizenRepository tạo mới citizen repository
func NewCitizenRepository(db *sqlx.DB) CitizenRepository {
	return &citizenRepo{db: db}
}

func (r *citizenRepo) Create(ctx context.Context, c *model.Citizen) error {
	query := `
		INSERT INTO citizens (
			id, full_name, date_of_birth, gender,
			national_id, phone_number, email, permanent_address,
			religion, ethnicity, marital_status,
			province_code, district_code, ward_code,
			is_alive, created_at, updated_at
		) VALUES (
			:id, :full_name, :date_of_birth, :gender,
			:national_id, :phone_number, :email, :permanent_address,
			:religion, :ethnicity, :marital_status,
			:province_code, :district_code, :ward_code,
			:is_alive, :created_at, :updated_at
		)
	`
	_, err := r.db.NamedExecContext(ctx, query, c)
	return err
}

func (r *citizenRepo) GetByID(ctx context.Context, id string) (*model.Citizen, error) {
	var c model.Citizen
	query := `
		SELECT * FROM citizens
		WHERE id = $1 AND deleted_at IS NULL
	`
	err := r.db.GetContext(ctx, &c, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (r *citizenRepo) Update(ctx context.Context, c *model.Citizen) error {
	c.UpdatedAt = time.Now()
	query := `
		UPDATE citizens SET
			full_name = :full_name,
			date_of_birth = :date_of_birth,
			gender = :gender,
			national_id = :national_id,
			phone_number = :phone_number,
			email = :email,
			permanent_address = :permanent_address,
			religion = :religion,
			ethnicity = :ethnicity,
			marital_status = :marital_status,
			province_code = :province_code,
			district_code = :district_code,
			ward_code = :ward_code,
			is_alive = :is_alive,
			updated_at = :updated_at
		WHERE id = :id AND deleted_at IS NULL
	`
	result, err := r.db.NamedExecContext(ctx, query, c)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *citizenRepo) SoftDelete(ctx context.Context, id string) error {
	query := `
		UPDATE citizens SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *citizenRepo) List(ctx context.Context, filter model.ListCitizenFilter) ([]*model.Citizen, int64, error) {
	conditions := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter.ProvinceCode != "" {
		conditions = append(conditions, fmt.Sprintf("province_code = $%d", argIdx))
		args = append(args, filter.ProvinceCode)
		argIdx++
	}
	if filter.DistrictCode != "" {
		conditions = append(conditions, fmt.Sprintf("district_code = $%d", argIdx))
		args = append(args, filter.DistrictCode)
		argIdx++
	}
	if filter.WardCode != "" {
		conditions = append(conditions, fmt.Sprintf("ward_code = $%d", argIdx))
		args = append(args, filter.WardCode)
		argIdx++
	}
	if filter.Gender != "" {
		conditions = append(conditions, fmt.Sprintf("gender = $%d", argIdx))
		args = append(args, filter.Gender)
		argIdx++
	}
	if filter.MaritalStatus != "" {
		conditions = append(conditions, fmt.Sprintf("marital_status = $%d", argIdx))
		args = append(args, filter.MaritalStatus)
		argIdx++
	}
	if filter.IsAlive != nil {
		conditions = append(conditions, fmt.Sprintf("is_alive = $%d", argIdx))
		args = append(args, *filter.IsAlive)
		argIdx++
	}
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("full_name ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count total
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM citizens WHERE %s", where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	query := fmt.Sprintf(`
		SELECT * FROM citizens
		WHERE %s
		ORDER BY created_at ASC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	var citizens []*model.Citizen
	if err := r.db.SelectContext(ctx, &citizens, query, args...); err != nil {
		return nil, 0, err
	}

	return citizens, total, nil
}

func (r *citizenRepo) GetPopulationStatsByProvince(ctx context.Context) ([]*model.PopulationStat, error) {
	query := `
		SELECT
			c.province_code,
			p.name AS province_name,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE c.gender = 'male') AS male,
			COUNT(*) FILTER (WHERE c.gender = 'female') AS female,
			COUNT(*) FILTER (WHERE c.gender = 'other') AS other,
			COUNT(*) FILTER (WHERE c.is_alive = true) AS alive,
			COUNT(*) FILTER (WHERE c.is_alive = false) AS deceased,
			COALESCE(
				AVG(EXTRACT(YEAR FROM AGE(NOW(), c.date_of_birth))),
				0
			) AS average_age
		FROM citizens c
		LEFT JOIN provinces p ON p.code = c.province_code
		WHERE c.deleted_at IS NULL
		GROUP BY c.province_code, p.name
		ORDER BY total ASC
	`
	var stats []*model.PopulationStat
	err := r.db.SelectContext(ctx, &stats, query)
	return stats, err
}

func (r *citizenRepo) GetPopulationStatByProvince(ctx context.Context, provinceCode string) (*model.PopulationStat, error) {
	query := `
		SELECT
			c.province_code,
			p.name AS province_name,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE c.gender = 'male') AS male,
			COUNT(*) FILTER (WHERE c.gender = 'female') AS female,
			COUNT(*) FILTER (WHERE c.gender = 'other') AS other,
			COUNT(*) FILTER (WHERE c.is_alive = true) AS alive,
			COUNT(*) FILTER (WHERE c.is_alive = false) AS deceased,
			COALESCE(
				AVG(EXTRACT(YEAR FROM AGE(NOW(), c.date_of_birth))),
				0
			) AS average_age
		FROM citizens c
		LEFT JOIN provinces p ON p.code = c.province_code
		WHERE c.deleted_at IS NULL AND c.province_code = $1
		GROUP BY c.province_code, p.name
	`
	var stat model.PopulationStat
	err := r.db.GetContext(ctx, &stat, query, provinceCode)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &stat, err
}

func (r *citizenRepo) ExistsByNationalID(ctx context.Context, nationalID string, excludeID string) (bool, error) {
	var query string
	var exists bool
	var err error

	if excludeID == "" {
		// Tạo mới: không cần loại trừ bản ghi nào
		query = `
			SELECT EXISTS(
				SELECT 1 FROM citizens
				WHERE national_id = $1 AND deleted_at IS NULL
			)
		`
		err = r.db.QueryRowContext(ctx, query, nationalID).Scan(&exists)
	} else {
		// Cập nhật: loại trừ bản ghi đang sửa
		query = `
			SELECT EXISTS(
				SELECT 1 FROM citizens
				WHERE national_id = $1 AND id != $2 AND deleted_at IS NULL
			)
		`
		err = r.db.QueryRowContext(ctx, query, nationalID, excludeID).Scan(&exists)
	}

	return exists, err
}

// UpdateResidence cập nhật địa bàn cư trú của công dân sau khi chuyển hộ khẩu
func (r *citizenRepo) UpdateResidence(ctx context.Context, citizenID, provinceCode, districtCode, wardCode string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE citizens
		SET province_code=$1, district_code=$2, ward_code=$3, updated_at=NOW()
		WHERE id=$4 AND deleted_at IS NULL`,
		provinceCode, districtCode, wardCode, citizenID)
	return err
}