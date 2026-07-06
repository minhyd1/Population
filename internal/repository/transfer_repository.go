package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"population-service/internal/model"
)

type TransferRepository interface {
	// TransferRequest
	CreateRequest(ctx context.Context, req *model.TransferRequest) error
	GetRequestByID(ctx context.Context, id string) (*model.TransferRequest, error)
	// Vấn đề 6: GetRequestByIDForUpdate — pessimistic lock (SELECT FOR UPDATE)
	GetRequestByIDForUpdate(ctx context.Context, id string) (*model.TransferRequest, error)
	ListRequests(ctx context.Context, filter model.ListTransferFilter) ([]*model.TransferRequest, int64, error)
	UpdateRequestStatus(ctx context.Context, id string, status model.TransferStatus) error
	CompleteRequest(ctx context.Context, id string) error
	// Vấn đề 4: Escalation
	EscalateRequest(ctx context.Context, id string, escalateTo string) error
	ListExpiredPending(ctx context.Context) ([]*model.TransferRequest, error)

	// TransferApproval
	CreateApprovals(ctx context.Context, approvals []*model.TransferApproval) error
	GetApprovalsByRequestID(ctx context.Context, requestID string) ([]*model.TransferApproval, error)
	// Vấn đề 3: GetPendingApprovalForUnit chỉ được xem approval của unit mình
	GetPendingApprovalForUnit(ctx context.Context, requestID, unitCode string) (*model.TransferApproval, error)
	// Vấn đề 3: HasApprovalForUnit kiểm tra unit có phiếu duyệt trong request này không
	HasApprovalForUnit(ctx context.Context, requestID, unitCode string) (bool, error)
	UpdateApproval(ctx context.Context, approval *model.TransferApproval) error

	// ResidenceHistory
	CreateResidenceHistory(ctx context.Context, h *model.ResidenceHistory) error
	GetResidenceHistory(ctx context.Context, citizenID string) ([]*model.ResidenceHistory, error)

	// Transaction support
	WithTx(ctx context.Context, fn func(txRepo TransferRepository) error) error
}

type transferRepo struct{ db *sqlx.DB }

func NewTransferRepository(db *sqlx.DB) TransferRepository {
	return &transferRepo{db: db}
}

// ─── TransferRequest ────────────────────────────────────────

func (r *transferRepo) CreateRequest(ctx context.Context, req *model.TransferRequest) error {
	// Vấn đề 4: tự set deadline_at = NOW() + 7 ngày
	q := `
		INSERT INTO transfer_requests
		  (id, citizen_id, from_household_id, to_household_id, from_unit_code, to_unit_code,
		   approval_level, status, reason, created_by, deadline_at)
		VALUES (uuid_generate_v4(), :citizen_id, :from_household_id, :to_household_id,
		        :from_unit_code, :to_unit_code, :approval_level, :status, :reason, :created_by, NOW() + INTERVAL '7 days')
		RETURNING id, created_at, updated_at, deadline_at`
	rows, err := r.db.NamedQueryContext(ctx, q, req)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt, &req.DeadlineAt)
	}
	return nil
}

func (r *transferRepo) GetRequestByID(ctx context.Context, id string) (*model.TransferRequest, error) {
	req := &model.TransferRequest{}
	err := r.db.GetContext(ctx, req, `SELECT * FROM transfer_requests WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// GetRequestByIDForUpdate — Vấn đề 6: SELECT FOR UPDATE tránh double-approve
// Phải được gọi trong một transaction (WithTx)
func (r *transferRepo) GetRequestByIDForUpdate(ctx context.Context, id string) (*model.TransferRequest, error) {
	// Không hỗ trợ ngoài transaction
	return nil, fmt.Errorf("GetRequestByIDForUpdate chỉ dùng trong transaction — dùng txTransferRepo")
}

func (r *transferRepo) ListRequests(ctx context.Context, filter model.ListTransferFilter) ([]*model.TransferRequest, int64, error) {
	where := "1=1"
	args := []interface{}{}
	idx := 1

	if filter.CitizenID != "" {
		where += fmt.Sprintf(" AND citizen_id=$%d", idx)
		args = append(args, filter.CitizenID)
		idx++
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND status=$%d", idx)
		args = append(args, string(filter.Status))
		idx++
	}
	// Vấn đề 3: Scope filter chặt tại Repository — chỉ xem request có liên quan đến unit
	// (WardCode → kiểm tra from hoặc to household thuộc ward đó)
	if filter.WardCode != "" {
		where += fmt.Sprintf(`
			AND (from_household_id IN (SELECT id FROM households WHERE ward_code=$%d)
			  OR to_household_id   IN (SELECT id FROM households WHERE ward_code=$%d))`, idx, idx)
		args = append(args, filter.WardCode)
		idx++
	} else if filter.DistrictCode != "" {
		where += fmt.Sprintf(`
			AND (from_household_id IN (SELECT id FROM households WHERE district_code=$%d)
			  OR to_household_id   IN (SELECT id FROM households WHERE district_code=$%d))`, idx, idx)
		args = append(args, filter.DistrictCode)
		idx++
	} else if filter.ProvinceCode != "" {
		where += fmt.Sprintf(`
			AND (from_household_id IN (SELECT id FROM households WHERE province_code=$%d)
			  OR to_household_id   IN (SELECT id FROM households WHERE province_code=$%d))`, idx, idx)
		args = append(args, filter.ProvinceCode)
		idx++
	}

	var total int64
	if err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM transfer_requests WHERE "+where, args...); err != nil {
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
		"SELECT * FROM transfer_requests WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, idx, idx+1,
	)
	args = append(args, pageSize, offset)

	var rows []*model.TransferRequest
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *transferRepo) UpdateRequestStatus(ctx context.Context, id string, status model.TransferStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transfer_requests SET status=$1, updated_at=NOW() WHERE id=$2`,
		string(status), id)
	return err
}

func (r *transferRepo) CompleteRequest(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transfer_requests SET status='completed', completed_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

// EscalateRequest — Vấn đề 4: đánh dấu escalated_to và ghi timestamp
func (r *transferRepo) EscalateRequest(ctx context.Context, id string, escalateTo string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transfer_requests
		 SET escalated_to=$1, updated_at=NOW()
		 WHERE id=$2 AND status='pending'`,
		escalateTo, id)
	return err
}

// ListExpiredPending — Vấn đề 4: tìm các request đã quá hạn chưa escalate
func (r *transferRepo) ListExpiredPending(ctx context.Context) ([]*model.TransferRequest, error) {
	var rows []*model.TransferRequest
	err := r.db.SelectContext(ctx, &rows, `
		SELECT * FROM transfer_requests
		WHERE status = 'pending'
		  AND deadline_at IS NOT NULL
		  AND deadline_at < NOW()
		  AND escalated_to IS NULL
		ORDER BY deadline_at ASC
	`)
	return rows, err
}

// ─── TransferApproval ───────────────────────────────────────

func (r *transferRepo) CreateApprovals(ctx context.Context, approvals []*model.TransferApproval) error {
	for _, a := range approvals {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO transfer_approvals (id, request_id, unit_code, unit_role, decision)
			VALUES (uuid_generate_v4(), $1, $2, $3, 'pending')`,
			a.RequestID, a.UnitCode, a.UnitRole)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *transferRepo) GetApprovalsByRequestID(ctx context.Context, requestID string) ([]*model.TransferApproval, error) {
	var approvals []*model.TransferApproval
	err := r.db.SelectContext(ctx, &approvals,
		`SELECT * FROM transfer_approvals WHERE request_id=$1 ORDER BY unit_role`, requestID)
	return approvals, err
}

// GetPendingApprovalForUnit — Vấn đề 3: chỉ lấy approval thuộc đúng unit_code
// Query tự enforce: không có approval của unit này → ErrNoRows → forbidden
func (r *transferRepo) GetPendingApprovalForUnit(ctx context.Context, requestID, unitCode string) (*model.TransferApproval, error) {
	a := &model.TransferApproval{}
	err := r.db.GetContext(ctx, a, `
		SELECT * FROM transfer_approvals
		WHERE request_id=$1 AND unit_code=$2 AND decision='pending'`, requestID, unitCode)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// HasApprovalForUnit — Vấn đề 3: kiểm tra nhanh unit có liên quan đến request không
func (r *transferRepo) HasApprovalForUnit(ctx context.Context, requestID, unitCode string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM transfer_approvals WHERE request_id=$1 AND unit_code=$2`,
		requestID, unitCode)
	return count > 0, err
}

func (r *transferRepo) UpdateApproval(ctx context.Context, a *model.TransferApproval) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE transfer_approvals
		SET decision=$1, approved_by=$2, reject_reason=$3, approved_at=NOW()
		WHERE id=$4`,
		string(a.Decision), a.ApprovedBy, a.RejectReason, a.ID)
	return err
}

// ─── ResidenceHistory ───────────────────────────────────────

func (r *transferRepo) CreateResidenceHistory(ctx context.Context, h *model.ResidenceHistory) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO residence_histories
		  (id, citizen_id, from_household_id, to_household_id, transfer_request_id, reason, effective_date)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NOW())`,
		h.CitizenID, h.FromHouseholdID, h.ToHouseholdID, h.TransferRequestID, h.Reason)
	return err
}

func (r *transferRepo) GetResidenceHistory(ctx context.Context, citizenID string) ([]*model.ResidenceHistory, error) {
	var rows []*model.ResidenceHistory
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM residence_histories WHERE citizen_id=$1 ORDER BY effective_date DESC`, citizenID)
	return rows, err
}

// ─── Transaction ────────────────────────────────────────────

// WithTx thực hiện function trong một database transaction
func (r *transferRepo) WithTx(ctx context.Context, fn func(txRepo TransferRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	txDB := &txTransferRepo{tx: tx}
	err = fn(txDB)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ─── txTransferRepo (Transaction wrapper) ───────────────────

type txTransferRepo struct{ tx *sqlx.Tx }

func (r *txTransferRepo) CreateRequest(ctx context.Context, req *model.TransferRequest) error {
	q := `
		INSERT INTO transfer_requests
		  (id, citizen_id, from_household_id, to_household_id, from_unit_code, to_unit_code,
		   approval_level, status, reason, created_by, deadline_at)
		VALUES (uuid_generate_v4(), :citizen_id, :from_household_id, :to_household_id,
		        :from_unit_code, :to_unit_code, :approval_level, :status, :reason, :created_by, NOW() + INTERVAL '7 days')
		RETURNING id, created_at, updated_at, deadline_at`
	rows, err := r.tx.NamedQuery(q, req)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt, &req.DeadlineAt)
	}
	return nil
}

func (r *txTransferRepo) GetRequestByID(ctx context.Context, id string) (*model.TransferRequest, error) {
	req := &model.TransferRequest{}
	err := r.tx.GetContext(ctx, req, `SELECT * FROM transfer_requests WHERE id=$1`, id)
	return req, err
}

// GetRequestByIDForUpdate — Vấn đề 6: SELECT FOR UPDATE trong transaction
// Chặn race condition: Ward A và Ward B cùng bấm Approve cùng lúc
func (r *txTransferRepo) GetRequestByIDForUpdate(ctx context.Context, id string) (*model.TransferRequest, error) {
	req := &model.TransferRequest{}
	err := r.tx.GetContext(ctx, req,
		`SELECT * FROM transfer_requests WHERE id=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (r *txTransferRepo) ListRequests(ctx context.Context, filter model.ListTransferFilter) ([]*model.TransferRequest, int64, error) {
	return nil, 0, fmt.Errorf("not supported in transaction")
}

func (r *txTransferRepo) UpdateRequestStatus(ctx context.Context, id string, status model.TransferStatus) error {
	_, err := r.tx.ExecContext(ctx,
		`UPDATE transfer_requests SET status=$1, updated_at=NOW() WHERE id=$2`,
		string(status), id)
	return err
}

func (r *txTransferRepo) CompleteRequest(ctx context.Context, id string) error {
	_, err := r.tx.ExecContext(ctx,
		`UPDATE transfer_requests SET status='completed', completed_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *txTransferRepo) EscalateRequest(ctx context.Context, id string, escalateTo string) error {
	_, err := r.tx.ExecContext(ctx,
		`UPDATE transfer_requests SET escalated_to=$1, updated_at=NOW() WHERE id=$2 AND status='pending'`,
		escalateTo, id)
	return err
}

func (r *txTransferRepo) ListExpiredPending(ctx context.Context) ([]*model.TransferRequest, error) {
	return nil, fmt.Errorf("not supported in transaction")
}

func (r *txTransferRepo) CreateApprovals(ctx context.Context, approvals []*model.TransferApproval) error {
	for _, a := range approvals {
		_, err := r.tx.ExecContext(ctx, `
			INSERT INTO transfer_approvals (id, request_id, unit_code, unit_role, decision)
			VALUES (uuid_generate_v4(), $1, $2, $3, 'pending')`,
			a.RequestID, a.UnitCode, a.UnitRole)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *txTransferRepo) GetApprovalsByRequestID(ctx context.Context, requestID string) ([]*model.TransferApproval, error) {
	var approvals []*model.TransferApproval
	err := r.tx.SelectContext(ctx, &approvals,
		`SELECT * FROM transfer_approvals WHERE request_id=$1 ORDER BY unit_role`, requestID)
	return approvals, err
}

func (r *txTransferRepo) GetPendingApprovalForUnit(ctx context.Context, requestID, unitCode string) (*model.TransferApproval, error) {
	a := &model.TransferApproval{}
	err := r.tx.GetContext(ctx, a, `
		SELECT * FROM transfer_approvals
		WHERE request_id=$1 AND unit_code=$2 AND decision='pending'`, requestID, unitCode)
	return a, err
}

func (r *txTransferRepo) HasApprovalForUnit(ctx context.Context, requestID, unitCode string) (bool, error) {
	var count int
	err := r.tx.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM transfer_approvals WHERE request_id=$1 AND unit_code=$2`,
		requestID, unitCode)
	return count > 0, err
}

func (r *txTransferRepo) UpdateApproval(ctx context.Context, a *model.TransferApproval) error {
	_, err := r.tx.ExecContext(ctx, `
		UPDATE transfer_approvals
		SET decision=$1, approved_by=$2, reject_reason=$3, approved_at=NOW()
		WHERE id=$4`,
		string(a.Decision), a.ApprovedBy, a.RejectReason, a.ID)
	return err
}

func (r *txTransferRepo) CreateResidenceHistory(ctx context.Context, h *model.ResidenceHistory) error {
	_, err := r.tx.ExecContext(ctx, `
		INSERT INTO residence_histories
		  (id, citizen_id, from_household_id, to_household_id, transfer_request_id, reason, effective_date)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NOW())`,
		h.CitizenID, h.FromHouseholdID, h.ToHouseholdID, h.TransferRequestID, h.Reason)
	return err
}

func (r *txTransferRepo) GetResidenceHistory(ctx context.Context, citizenID string) ([]*model.ResidenceHistory, error) {
	var rows []*model.ResidenceHistory
	err := r.tx.SelectContext(ctx, &rows,
		`SELECT * FROM residence_histories WHERE citizen_id=$1 ORDER BY effective_date DESC`, citizenID)
	return rows, err
}

func (r *txTransferRepo) WithTx(ctx context.Context, fn func(txRepo TransferRepository) error) error {
	return fmt.Errorf("nested transactions not supported")
}

// Ensure txTransferRepo satisfies interface at compile time
var _ TransferRepository = (*txTransferRepo)(nil)

// Helper: kiểm tra error là "no rows" không
func isNoRows(err error) bool {
	return err != nil && err.Error() == sql.ErrNoRows.Error()
}
