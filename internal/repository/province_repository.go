package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"population-service/internal/model"
)

// ProvinceRepository interface
type ProvinceRepository interface {
	GetAll(ctx context.Context) ([]*model.Province, error)
	GetByCode(ctx context.Context, code string) (*model.Province, error)
}

type provinceRepo struct {
	db *sqlx.DB
}

func NewProvinceRepository(db *sqlx.DB) ProvinceRepository {
	return &provinceRepo{db: db}
}

func (r *provinceRepo) GetAll(ctx context.Context) ([]*model.Province, error) {
	var provinces []*model.Province
	err := r.db.SelectContext(ctx, &provinces, "SELECT * FROM provinces ORDER BY name")
	return provinces, err
}

func (r *provinceRepo) GetByCode(ctx context.Context, code string) (*model.Province, error) {
	var p model.Province
	err := r.db.GetContext(ctx, &p, "SELECT * FROM provinces WHERE code = $1", code)
	return &p, err
}
