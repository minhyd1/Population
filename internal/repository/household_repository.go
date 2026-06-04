package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"population-service/internal/model"
)

type HouseholdRepository interface {
	Create(ctx context.Context, h *model.Household) error
	GetByID(ctx context.Context, id string) (*model.Household, error)
	GetByNo(ctx context.Context, no string) (*model.Household, error)
	List(ctx context.Context, filter model.ListHouseholdFilter) ([]*model.Household, int64, error)

	AddMember(ctx context.Context, m *model.HouseholdMember) error
	RemoveMember(ctx context.Context, householdID, citizenID string) error
	GetMembers(ctx context.Context, householdID string) ([]*model.HouseholdMember, error)
	// GetMemberHistory trả về toàn bộ lịch sử thành viên (kể cả đã rời)
	GetMemberHistory(ctx context.Context, householdID string) ([]*model.HouseholdMember, error)
	GetMemberHousehold(ctx context.Context, citizenID string) (*model.Household, error)
}

type householdRepo struct{ db *sqlx.DB }

func NewHouseholdRepository(db *sqlx.DB) HouseholdRepository {
	return &householdRepo{db: db}
}

func (r *householdRepo) Create(ctx context.Context, h *model.Household) error {
	q := `
		INSERT INTO households (id, household_no, province_code, district_code, ward_code, address, head_citizen_id)
		VALUES (uuid_generate_v4(), :household_no, :province_code, :district_code, :ward_code, :address, :head_citizen_id)
		RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, h)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)
	}
	return nil
}

func (r *householdRepo) GetByID(ctx context.Context, id string) (*model.Household, error) {
	h := &model.Household{}
	err := r.db.GetContext(ctx, h,
		`SELECT * FROM households WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (r *householdRepo) GetByNo(ctx context.Context, no string) (*model.Household, error) {
	h := &model.Household{}
	err := r.db.GetContext(ctx, h,
		`SELECT * FROM households WHERE household_no=$1 AND deleted_at IS NULL`, no)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (r *householdRepo) List(ctx context.Context, filter model.ListHouseholdFilter) ([]*model.Household, int64, error) {
	where := "deleted_at IS NULL"
	args := []interface{}{}
	idx := 1
	if filter.WardCode != "" {
		where += fmt.Sprintf(" AND ward_code=$%d", idx)
		args = append(args, filter.WardCode)
		idx++
	} else if filter.DistrictCode != "" {
		where += fmt.Sprintf(" AND district_code=$%d", idx)
		args = append(args, filter.DistrictCode)
		idx++
	} else if filter.ProvinceCode != "" {
		where += fmt.Sprintf(" AND province_code=$%d", idx)
		args = append(args, filter.ProvinceCode)
		idx++
	}

	var total int64
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM households WHERE "+where, args...); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(
		"SELECT * FROM households WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, idx, idx+1,
	)
	args = append(args, pageSize, offset)

	var rows []*model.Household
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *householdRepo) AddMember(ctx context.Context, m *model.HouseholdMember) error {
	// Thêm thành viên mới — INSERT hoàn toàn mới (không UPDATE ON CONFLICT)
	// vì mỗi lần thêm là một bản ghi lịch sử riêng.
	// Constraint UNIQUE (household_id, citizen_id) WHERE is_active=TRUE đảm bảo
	// không có hai bản ghi active cùng lúc cho cùng một cặp.
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO household_members (id, household_id, citizen_id, relationship, is_active)
		 VALUES (uuid_generate_v4(), $1, $2, $3, TRUE)`,
		m.HouseholdID, m.CitizenID, m.Relationship)
	return err
}

// RemoveMember đánh dấu thành viên đã rời hộ (soft-leave).
// KHÔNG DELETE — giữ lại lịch sử là ai từng ở hộ nào.
func (r *householdRepo) RemoveMember(ctx context.Context, householdID, citizenID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE household_members
		 SET is_active=FALSE, left_at=NOW()
		 WHERE household_id=$1 AND citizen_id=$2 AND is_active=TRUE`,
		householdID, citizenID)
	return err
}

// GetMembers trả về thành viên HIỆN TẠI (is_active=TRUE)
func (r *householdRepo) GetMembers(ctx context.Context, householdID string) ([]*model.HouseholdMember, error) {
	var members []*model.HouseholdMember
	err := r.db.SelectContext(ctx, &members,
		`SELECT id, household_id, citizen_id, relationship, joined_at, left_at, is_active
		 FROM household_members
		 WHERE household_id=$1 AND is_active=TRUE
		 ORDER BY joined_at`, householdID)
	return members, err
}

// GetMemberHistory trả về TOÀN BỘ lịch sử thành viên của hộ (kể cả đã rời)
func (r *householdRepo) GetMemberHistory(ctx context.Context, householdID string) ([]*model.HouseholdMember, error) {
	var members []*model.HouseholdMember
	err := r.db.SelectContext(ctx, &members,
		`SELECT id, household_id, citizen_id, relationship, joined_at, left_at, is_active
		 FROM household_members
		 WHERE household_id=$1
		 ORDER BY joined_at DESC`, householdID)
	return members, err
}

// GetMemberHousehold trả về hộ hiện tại của công dân (chỉ thành viên active)
func (r *householdRepo) GetMemberHousehold(ctx context.Context, citizenID string) (*model.Household, error) {
	h := &model.Household{}
	err := r.db.GetContext(ctx, h, `
		SELECT h.* FROM households h
		JOIN household_members hm ON hm.household_id = h.id
		WHERE hm.citizen_id = $1 AND hm.is_active = TRUE AND h.deleted_at IS NULL`, citizenID)
	if err != nil {
		return nil, err
	}
	return h, nil
}
