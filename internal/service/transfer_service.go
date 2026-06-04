package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	jwtpkg "population-service/pkg/jwt"
	"population-service/internal/model"
	"population-service/internal/repository"
	"population-service/pkg/middleware"
)

// TransferService xử lý toàn bộ workflow chuyển hộ khẩu
type TransferService interface {
	// Household
	CreateHousehold(ctx context.Context, req model.CreateHouseholdRequest) (*model.HouseholdResponse, error)
	GetHousehold(ctx context.Context, id string) (*model.HouseholdResponse, error)
	ListHouseholds(ctx context.Context, filter model.ListHouseholdFilter) (*model.HouseholdListResponse, error)
	AddHouseholdMember(ctx context.Context, householdID string, req model.AddHouseholdMemberRequest) error

	// Transfer Workflow
	CreateTransferRequest(ctx context.Context, req model.CreateTransferRequest) (*model.TransferRequestResponse, error)
	GetTransferRequest(ctx context.Context, id string) (*model.TransferRequestResponse, error)
	ListTransferRequests(ctx context.Context, filter model.ListTransferFilter) (*model.TransferListResponse, error)
	ApproveTransfer(ctx context.Context, requestID string, req model.ApproveTransferRequest) error
	ForceApproveTransfer(ctx context.Context, requestID string, req model.ForceApproveRequest) error

	// Residence History
	GetResidenceHistory(ctx context.Context, citizenID string) ([]*model.ResidenceHistoryResponse, error)

	// Vấn đề 4: Escalation
	EscalateExpiredRequests(ctx context.Context) (int, error)
}

type transferSvc struct {
	transferRepo  repository.TransferRepository
	householdRepo repository.HouseholdRepository
	citizenRepo   repository.CitizenRepository
	auditRepo     repository.AuditRepository
}

func NewTransferService(
	db *sqlx.DB,
	transferRepo repository.TransferRepository,
	householdRepo repository.HouseholdRepository,
	citizenRepo repository.CitizenRepository,
	auditRepo repository.AuditRepository,
) TransferService {
	return &transferSvc{
		transferRepo:  transferRepo,
		householdRepo: householdRepo,
		citizenRepo:   citizenRepo,
		auditRepo:     auditRepo,
	}
}

// ─── Household ──────────────────────────────────────────────

func (s *transferSvc) CreateHousehold(ctx context.Context, req model.CreateHouseholdRequest) (*model.HouseholdResponse, error) {
	claims := claimsFromCtx(ctx)
	if claims != nil {
		if err := canManageUnit(claims, req.WardCode, req.DistrictCode, req.ProvinceCode); err != nil {
			return nil, err
		}
	}

	h := &model.Household{
		HouseholdNo:   req.HouseholdNo,
		ProvinceCode:  req.ProvinceCode,
		DistrictCode:  req.DistrictCode,
		WardCode:      req.WardCode,
		Address:       req.Address,
		HeadCitizenID: req.HeadCitizenID,
	}
	if err := s.householdRepo.Create(ctx, h); err != nil {
		return nil, fmt.Errorf("create household: %w", err)
	}
	return toHouseholdResponse(h, nil), nil
}

func (s *transferSvc) GetHousehold(ctx context.Context, id string) (*model.HouseholdResponse, error) {
	h, err := s.householdRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	members, _ := s.householdRepo.GetMembers(ctx, id)
	return toHouseholdResponse(h, members), nil
}

func (s *transferSvc) ListHouseholds(ctx context.Context, filter model.ListHouseholdFilter) (*model.HouseholdListResponse, error) {
	claims := claimsFromCtx(ctx)
	if claims != nil {
		switch claims.Role {
		case jwtpkg.RoleWardOfficer:
			if filter.WardCode == "" {
				filter.WardCode = claims.WardCode
			}
		case jwtpkg.RoleDistrictManager:
			if filter.DistrictCode == "" && filter.WardCode == "" {
				filter.DistrictCode = claims.DistrictCode
			}
		case jwtpkg.RoleProvinceManager:
			if filter.ProvinceCode == "" && filter.DistrictCode == "" && filter.WardCode == "" {
				filter.ProvinceCode = claims.ProvinceCode
			}
		}
	}

	rows, total, err := s.householdRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]model.HouseholdResponse, 0, len(rows))
	for _, h := range rows {
		resp = append(resp, *toHouseholdResponse(h, nil))
	}
	return &model.HouseholdListResponse{
		Data:     resp,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func (s *transferSvc) AddHouseholdMember(ctx context.Context, householdID string, req model.AddHouseholdMemberRequest) error {
	h, err := s.householdRepo.GetByID(ctx, householdID)
	if err != nil {
		return fmt.Errorf("household not found: %w", err)
	}
	claims := claimsFromCtx(ctx)
	if claims != nil {
		if err := canManageUnit(claims, h.WardCode, h.DistrictCode, h.ProvinceCode); err != nil {
			return err
		}
	}
	return s.householdRepo.AddMember(ctx, &model.HouseholdMember{
		HouseholdID:  householdID,
		CitizenID:    req.CitizenID,
		Relationship: req.Relationship,
	})
}

// ─── Transfer Workflow ──────────────────────────────────────

// CreateTransferRequest tạo yêu cầu chuyển hộ khẩu và tự động sinh phiếu phê duyệt
func (s *transferSvc) CreateTransferRequest(ctx context.Context, req model.CreateTransferRequest) (*model.TransferRequestResponse, error) {
	fromHH, err := s.householdRepo.GetByID(ctx, req.FromHouseholdID)
	if err != nil {
		return nil, fmt.Errorf("from_household not found: %w", err)
	}
	toHH, err := s.householdRepo.GetByID(ctx, req.ToHouseholdID)
	if err != nil {
		return nil, fmt.Errorf("to_household not found: %w", err)
	}

	// ─── Vấn đề 5: Household Ownership Validation ───────────
	// Kiểm tra citizen có thực sự thuộc from_household không
	actualHH, err := s.householdRepo.GetMemberHousehold(ctx, req.CitizenID)
	if err != nil {
		// Nếu không tìm thấy household → citizen chưa thuộc hộ nào
		return nil, fmt.Errorf("citizen_id=%s không thuộc hộ khẩu nào: %w", req.CitizenID, err)
	}
	if actualHH.ID != req.FromHouseholdID {
		return nil, fmt.Errorf(
			"công dân hiện thuộc hộ %s (household_no=%s), không phải hộ %s — 400 Bad Request",
			actualHH.ID, actualHH.HouseholdNo, req.FromHouseholdID,
		)
	}
	// ─────────────────────────────────────────────────────────

	level := determineApprovalLevel(fromHH, toHH)

	claims := claimsFromCtx(ctx)
	createdBy := ""
	if claims != nil {
		createdBy = claims.UserID
	}

	// Snapshot unit_code tại thời điểm tạo — bất biến (điểm 3: transfer snapshot)
	// Nếu household sau này đổi địa chỉ / ward_code, lịch sử request vẫn chính xác
	fromUnitCode := fromHH.WardCode
	toUnitCode := toHH.WardCode

	treq := &model.TransferRequest{
		CitizenID:       req.CitizenID,
		FromHouseholdID: req.FromHouseholdID,
		ToHouseholdID:   req.ToHouseholdID,
		FromUnitCode:    &fromUnitCode,
		ToUnitCode:      &toUnitCode,
		ApprovalLevel:   level,
		Status:          model.TransferStatusPending,
		Reason:          req.Reason,
		CreatedBy:       createdBy,
	}

	if err := s.transferRepo.CreateRequest(ctx, treq); err != nil {
		return nil, fmt.Errorf("create transfer request: %w", err)
	}

	// Sinh phiếu phê duyệt tự động theo cấp
	approvals := buildApprovals(treq.ID, fromHH, toHH, level)
	if len(approvals) > 0 {
		if err := s.transferRepo.CreateApprovals(ctx, approvals); err != nil {
			return nil, fmt.Errorf("create approvals: %w", err)
		}
	} else {
		// Cùng phường — không cần phê duyệt, tự động hoàn thành
		if err := s.executeTransfer(ctx, treq, fromHH); err != nil {
			return nil, err
		}
	}

	return s.buildTransferResponse(ctx, treq)
}

func (s *transferSvc) GetTransferRequest(ctx context.Context, id string) (*model.TransferRequestResponse, error) {
	req, err := s.transferRepo.GetRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("transfer request not found")
		}
		return nil, err
	}

	claims := claimsFromCtx(ctx)
	if claims != nil {
		if err := s.canViewTransfer(ctx, claims, req); err != nil {
			return nil, err
		}
	}

	return s.buildTransferResponse(ctx, req)
}

func (s *transferSvc) ListTransferRequests(ctx context.Context, filter model.ListTransferFilter) (*model.TransferListResponse, error) {
	claims := claimsFromCtx(ctx)
	if claims != nil {
		// Vấn đề 3: enforce scope tại Service → Repository cũng enforce
		switch claims.Role {
		case jwtpkg.RoleWardOfficer:
			filter.WardCode = claims.WardCode
		case jwtpkg.RoleDistrictManager:
			if filter.WardCode == "" {
				filter.DistrictCode = claims.DistrictCode
			}
		case jwtpkg.RoleProvinceManager:
			if filter.WardCode == "" && filter.DistrictCode == "" {
				filter.ProvinceCode = claims.ProvinceCode
			}
		}
	}

	rows, total, err := s.transferRepo.ListRequests(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]model.TransferRequestResponse, 0, len(rows))
	for _, req := range rows {
		r, _ := s.buildTransferResponse(ctx, req)
		if r != nil {
			resp = append(resp, *r)
		}
	}
	return &model.TransferListResponse{
		Data:     resp,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// ApproveTransfer cho phép cán bộ phụ trách đơn vị duyệt/từ chối
// Vấn đề 3: Enforce ở cả Repository (HasApprovalForUnit) + Vấn đề 6: SELECT FOR UPDATE
func (s *transferSvc) ApproveTransfer(ctx context.Context, requestID string, req model.ApproveTransferRequest) error {
	if req.Decision == model.ApprovalDecisionRejected && req.RejectReason == "" {
		return fmt.Errorf("reject_reason bắt buộc khi từ chối")
	}

	claims := claimsFromCtx(ctx)
	if claims == nil {
		return fmt.Errorf("unauthorized")
	}

	// Xác định unit_code của người duyệt
	unitCode := getUnitCode(claims)
	if unitCode == "" {
		return fmt.Errorf("user không có địa bàn phụ trách")
	}

	// Vấn đề 3: Kiểm tra sớm tại Repository — unit có phiếu duyệt trong request này không?
	// (Tránh truy vấn thông tin request cho đơn vị không liên quan)
	hasApproval, err := s.transferRepo.HasApprovalForUnit(ctx, requestID, unitCode)
	if err != nil {
		return fmt.Errorf("kiểm tra quyền duyệt: %w", err)
	}
	if !hasApproval {
		return fmt.Errorf("forbidden: đơn vị %s không có phiếu duyệt cho request này", unitCode)
	}

	// Vấn đề 6: Toàn bộ logic approve chạy trong transaction với SELECT FOR UPDATE
	return s.transferRepo.WithTx(ctx, func(txRepo repository.TransferRepository) error {
		// SELECT FOR UPDATE — chặn Ward A và Ward B cùng trigger executeTransfer
		treq, err := txRepo.GetRequestByIDForUpdate(ctx, requestID)
		if err != nil {
			return fmt.Errorf("transfer request not found: %w", err)
		}
		if treq.Status != model.TransferStatusPending {
			return fmt.Errorf("yêu cầu không ở trạng thái pending (hiện tại: %s)", treq.Status)
		}

		approval, err := txRepo.GetPendingApprovalForUnit(ctx, requestID, unitCode)
		if err != nil {
			return fmt.Errorf("không tìm thấy phiếu phê duyệt cho đơn vị %s: %w", unitCode, err)
		}

		approval.Decision = req.Decision
		approval.ApprovedBy = &claims.UserID
		if req.RejectReason != "" {
			approval.RejectReason = &req.RejectReason
		}

		if err := txRepo.UpdateApproval(ctx, approval); err != nil {
			return fmt.Errorf("update approval: %w", err)
		}

		if req.Decision == model.ApprovalDecisionApproved {
			allApprovals, err := txRepo.GetApprovalsByRequestID(ctx, requestID)
			if err != nil {
				return err
			}
			if checkAllApproved(allApprovals) {
				fromHH, err := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
				if err != nil {
					return err
				}
				// executeTransfer cần repo riêng vì đang trong transaction
				return s.executeTransferInTx(ctx, txRepo, treq, fromHH)
			}
		} else if req.Decision == model.ApprovalDecisionRejected {
			return txRepo.UpdateRequestStatus(ctx, requestID, model.TransferStatusRejected)
		}

		return nil
	})
}

// ForceApproveTransfer dành cho super_admin override trong tình huống khẩn cấp
func (s *transferSvc) ForceApproveTransfer(ctx context.Context, requestID string, req model.ForceApproveRequest) error {
	claims := claimsFromCtx(ctx)
	if claims == nil || claims.Role != jwtpkg.RoleSuperAdmin {
		return fmt.Errorf("chỉ super_admin mới có quyền force approve")
	}

	treq, err := s.transferRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("transfer request not found: %w", err)
	}
	if treq.Status == model.TransferStatusCompleted || treq.Status == model.TransferStatusCancelled {
		return fmt.Errorf("yêu cầu đã hoàn thành hoặc bị hủy")
	}

	fromHH, err := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
	if err != nil {
		return err
	}

	logID := uuid.New().String()
	now := time.Now()
	_ = s.auditRepo.Insert(ctx, &model.AuditLog{
		ID:            logID,
		CitizenID:     treq.CitizenID,
		Action:        model.AuditAction("force_approve_transfer"),
		ChangedBy:     claims.UserID,
		ChangedByName: claims.Username,
		ChangedByRole: string(claims.Role),
		ChangedAt:     now,
	})

	return s.executeTransfer(ctx, treq, fromHH)
}

// ─── Residence History ──────────────────────────────────────

func (s *transferSvc) GetResidenceHistory(ctx context.Context, citizenID string) ([]*model.ResidenceHistoryResponse, error) {
	rows, err := s.transferRepo.GetResidenceHistory(ctx, citizenID)
	if err != nil {
		return nil, err
	}
	resp := make([]*model.ResidenceHistoryResponse, 0, len(rows))
	for _, h := range rows {
		resp = append(resp, &model.ResidenceHistoryResponse{
			ID:                h.ID,
			CitizenID:         h.CitizenID,
			FromHouseholdID:   h.FromHouseholdID,
			ToHouseholdID:     h.ToHouseholdID,
			TransferRequestID: h.TransferRequestID,
			Reason:            h.Reason,
			EffectiveDate:     h.EffectiveDate,
			CreatedAt:         h.CreatedAt,
		})
	}
	return resp, nil
}

// ─── Vấn đề 4: Escalation Cron ─────────────────────────────

// EscalateExpiredRequests tìm tất cả request quá hạn 7 ngày và escalate lên cấp trên.
// Được gọi bởi cron job (ví dụ: mỗi giờ).
// Trả về số lượng request đã được escalate.
func (s *transferSvc) EscalateExpiredRequests(ctx context.Context) (int, error) {
	expired, err := s.transferRepo.ListExpiredPending(ctx)
	if err != nil {
		return 0, fmt.Errorf("list expired: %w", err)
	}

	count := 0
	for _, treq := range expired {
		escalateTo, err := s.determineEscalationTarget(ctx, treq)
		if err != nil || escalateTo == "" {
			continue
		}

		if err := s.transferRepo.EscalateRequest(ctx, treq.ID, escalateTo); err != nil {
			continue
		}

		// Tạo approval mới cho cấp escalated
		_ = s.transferRepo.CreateApprovals(ctx, []*model.TransferApproval{
			{RequestID: treq.ID, UnitCode: escalateTo, UnitRole: "escalated"},
		})

		// Ghi audit log cho escalation
		logID := uuid.New().String()
		now := time.Now()
		_ = s.auditRepo.Insert(ctx, &model.AuditLog{
			ID:            logID,
			CitizenID:     treq.CitizenID,
			Action:        model.AuditAction("transfer_escalated"),
			ChangedBy:     "system",
			ChangedByName: "cron",
			ChangedByRole: "system",
			ChangedAt:     now,
		})

		count++
	}
	return count, nil
}

// determineEscalationTarget xác định unit_code cấp trên để escalate
// Dựa trên approval_level của request
func (s *transferSvc) determineEscalationTarget(ctx context.Context, treq *model.TransferRequest) (string, error) {
	fromHH, err := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
	if err != nil {
		return "", err
	}
	switch treq.ApprovalLevel {
	case model.ApprovalLevelWard:
		// Ward không duyệt → escalate lên District
		return fromHH.DistrictCode, nil
	case model.ApprovalLevelDistrict:
		// District không duyệt → escalate lên Province
		return fromHH.ProvinceCode, nil
	case model.ApprovalLevelProvince:
		// Province không duyệt → không có cấp trên, đây là max
		return "", nil
	}
	return "", nil
}

// ─── ExecuteTransfer (Transaction) ─────────────────────────

// executeTransfer — gọi từ bên ngoài transaction (ForceApprove, ApprovalLevelNone)
func (s *transferSvc) executeTransfer(ctx context.Context, treq *model.TransferRequest, fromHH *model.Household) error {
	return s.transferRepo.WithTx(ctx, func(txRepo repository.TransferRepository) error {
		return s.executeTransferInTx(ctx, txRepo, treq, fromHH)
	})
}

// executeTransferInTx — gọi từ bên trong transaction đang có (ApproveTransfer)
// Tách ra để tránh nested transaction
func (s *transferSvc) executeTransferInTx(
	ctx context.Context,
	txRepo repository.TransferRepository,
	treq *model.TransferRequest,
	fromHH *model.Household,
) error {
	toHH, err := s.householdRepo.GetByID(ctx, treq.ToHouseholdID)
	if err != nil {
		return err
	}

	// 1. Residence History
	requestID := treq.ID
	history := &model.ResidenceHistory{
		CitizenID:         treq.CitizenID,
		FromHouseholdID:   &treq.FromHouseholdID,
		ToHouseholdID:     treq.ToHouseholdID,
		TransferRequestID: &requestID,
		Reason:            treq.Reason,
	}
	if err := txRepo.CreateResidenceHistory(ctx, history); err != nil {
		return fmt.Errorf("residence history: %w", err)
	}

	// 2. Xóa khỏi hộ cũ
	if err := s.householdRepo.RemoveMember(ctx, treq.FromHouseholdID, treq.CitizenID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}

	// 3. Thêm vào hộ mới
	if err := s.householdRepo.AddMember(ctx, &model.HouseholdMember{
		HouseholdID:  treq.ToHouseholdID,
		CitizenID:    treq.CitizenID,
		Relationship: "thành viên",
	}); err != nil {
		return fmt.Errorf("add member: %w", err)
	}

	// 4. Cập nhật địa chỉ công dân theo hộ mới
	if err := s.citizenRepo.UpdateResidence(ctx, treq.CitizenID,
		toHH.ProvinceCode, toHH.DistrictCode, toHH.WardCode); err != nil {
		return fmt.Errorf("update citizen residence: %w", err)
	}

	// 5. Cập nhật request status = completed
	if err := txRepo.CompleteRequest(ctx, treq.ID); err != nil {
		return fmt.Errorf("complete request: %w", err)
	}

	// 6. Audit log với visibility
	// Vấn đề 2: Ward A, Ward B, District A, District B, Province A đều thấy
	visibilityUnits := buildAuditVisibility(fromHH, toHH)
	logID := uuid.New().String()
	now := time.Now()
	auditLog := &model.AuditLog{
		ID:            logID,
		CitizenID:     treq.CitizenID,
		Action:        model.AuditAction("transfer_completed"),
		ChangedBy:     treq.CreatedBy,
		ChangedByName: "system",
		ChangedByRole: "transfer_workflow",
		ChangedAt:     now,
	}
	_ = s.auditRepo.InsertWithVisibility(ctx, auditLog, visibilityUnits)

	_ = fromHH // used for reference
	return nil
}

// buildAuditVisibility sinh danh sách unit_code được phép xem audit log
// của một operation chuyển hộ khẩu.
// Vấn đề 2: Ward A, Ward B, District A, District B, Province A được xem.
// Ward C không được xem.
func buildAuditVisibility(from, to *model.Household) []string {
	seen := map[string]bool{}
	var units []string
	add := func(code string) {
		if code != "" && !seen[code] {
			seen[code] = true
			units = append(units, code)
		}
	}
	// Từ nơi đi: ward, district, province
	add(from.WardCode)
	add(from.DistrictCode)
	add(from.ProvinceCode)
	// Từ nơi đến: ward, district, province
	add(to.WardCode)
	add(to.DistrictCode)
	add(to.ProvinceCode)
	return units
}

// ─── Helpers ────────────────────────────────────────────────

func determineApprovalLevel(from, to *model.Household) model.ApprovalLevel {
	if from.WardCode == to.WardCode {
		return model.ApprovalLevelNone
	}
	if from.DistrictCode == to.DistrictCode {
		return model.ApprovalLevelWard
	}
	if from.ProvinceCode == to.ProvinceCode {
		return model.ApprovalLevelDistrict
	}
	return model.ApprovalLevelProvince
}

func buildApprovals(requestID string, from, to *model.Household, level model.ApprovalLevel) []*model.TransferApproval {
	var approvals []*model.TransferApproval
	switch level {
	case model.ApprovalLevelWard:
		approvals = []*model.TransferApproval{
			{RequestID: requestID, UnitCode: from.WardCode, UnitRole: "source"},
			{RequestID: requestID, UnitCode: to.WardCode, UnitRole: "destination"},
		}
	case model.ApprovalLevelDistrict:
		approvals = []*model.TransferApproval{
			{RequestID: requestID, UnitCode: from.DistrictCode, UnitRole: "source"},
			{RequestID: requestID, UnitCode: to.DistrictCode, UnitRole: "destination"},
		}
	case model.ApprovalLevelProvince:
		approvals = []*model.TransferApproval{
			{RequestID: requestID, UnitCode: from.ProvinceCode, UnitRole: "source"},
			{RequestID: requestID, UnitCode: to.ProvinceCode, UnitRole: "destination"},
		}
	}
	return approvals
}

func checkAllApproved(approvals []*model.TransferApproval) bool {
	for _, a := range approvals {
		if a.Decision != model.ApprovalDecisionApproved {
			return false
		}
	}
	return len(approvals) > 0
}

func getUnitCode(claims *jwtpkg.Claims) string {
	switch claims.Role {
	case jwtpkg.RoleWardOfficer:
		return claims.WardCode
	case jwtpkg.RoleDistrictManager:
		return claims.DistrictCode
	case jwtpkg.RoleProvinceManager:
		return claims.ProvinceCode
	}
	return ""
}

func canManageUnit(claims *jwtpkg.Claims, wardCode, districtCode, provinceCode string) error {
	if claims.Role == jwtpkg.RoleSuperAdmin || claims.Role == jwtpkg.RoleNationalManager {
		return nil
	}
	switch claims.Role {
	case jwtpkg.RoleWardOfficer:
		if claims.WardCode != wardCode {
			return fmt.Errorf("forbidden: ngoài phạm vi phường %s", claims.WardCode)
		}
	case jwtpkg.RoleDistrictManager:
		if claims.DistrictCode != districtCode {
			return fmt.Errorf("forbidden: ngoài phạm vi quận %s", claims.DistrictCode)
		}
	case jwtpkg.RoleProvinceManager:
		if claims.ProvinceCode != provinceCode {
			return fmt.Errorf("forbidden: ngoài phạm vi tỉnh %s", claims.ProvinceCode)
		}
	}
	return nil
}

// canViewTransfer — Vấn đề 2 & 3: kiểm tra quyền xem request dựa trên approval_visibility
func (s *transferSvc) canViewTransfer(ctx context.Context, claims *jwtpkg.Claims, treq *model.TransferRequest) error {
	if claims.Role == jwtpkg.RoleSuperAdmin || claims.Role == jwtpkg.RoleNationalManager ||
		claims.Role == jwtpkg.RoleAuditor {
		return nil
	}

	unitCode := getUnitCode(claims)
	if unitCode == "" {
		if claims.Role == jwtpkg.RoleCitizenSelf && treq.CitizenID == claims.UserID {
			return nil
		}
		return fmt.Errorf("forbidden: không có quyền xem yêu cầu này")
	}

	// Vấn đề 3: Kiểm tra tại Repository — unit có approval trong request này không?
	hasApproval, err := s.transferRepo.HasApprovalForUnit(ctx, treq.ID, unitCode)
	if err != nil {
		return fmt.Errorf("kiểm tra quyền xem: %w", err)
	}
	if hasApproval {
		return nil
	}

	// District Manager / Province Manager: cũng có thể xem nếu household thuộc địa bàn
	fromHH, _ := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
	toHH, _ := s.householdRepo.GetByID(ctx, treq.ToHouseholdID)
	switch claims.Role {
	case jwtpkg.RoleDistrictManager:
		if (fromHH != nil && fromHH.DistrictCode == claims.DistrictCode) ||
			(toHH != nil && toHH.DistrictCode == claims.DistrictCode) {
			return nil
		}
	case jwtpkg.RoleProvinceManager:
		if (fromHH != nil && fromHH.ProvinceCode == claims.ProvinceCode) ||
			(toHH != nil && toHH.ProvinceCode == claims.ProvinceCode) {
			return nil
		}
	}

	return fmt.Errorf("forbidden: không có quyền xem yêu cầu này")
}

func (s *transferSvc) buildTransferResponse(ctx context.Context, req *model.TransferRequest) (*model.TransferRequestResponse, error) {
	approvals, _ := s.transferRepo.GetApprovalsByRequestID(ctx, req.ID)
	appResp := make([]model.TransferApprovalResponse, 0, len(approvals))
	for _, a := range approvals {
		appResp = append(appResp, model.TransferApprovalResponse{
			ID:           a.ID,
			UnitCode:     a.UnitCode,
			UnitRole:     a.UnitRole,
			Decision:     a.Decision,
			ApprovedBy:   a.ApprovedBy,
			RejectReason: a.RejectReason,
			ApprovedAt:   a.ApprovedAt,
		})
	}
	return &model.TransferRequestResponse{
		ID:              req.ID,
		CitizenID:       req.CitizenID,
		FromHouseholdID: req.FromHouseholdID,
		ToHouseholdID:   req.ToHouseholdID,
		ApprovalLevel:   req.ApprovalLevel,
		Status:          req.Status,
		Reason:          req.Reason,
		CreatedBy:       req.CreatedBy,
		CreatedAt:       req.CreatedAt,
		UpdatedAt:       req.UpdatedAt,
		CompletedAt:     req.CompletedAt,
		DeadlineAt:      req.DeadlineAt,
		EscalatedTo:     req.EscalatedTo,
		Approvals:       appResp,
	}, nil
}

func toHouseholdResponse(h *model.Household, members []*model.HouseholdMember) *model.HouseholdResponse {
	resp := &model.HouseholdResponse{
		ID:            h.ID,
		HouseholdNo:   h.HouseholdNo,
		ProvinceCode:  h.ProvinceCode,
		DistrictCode:  h.DistrictCode,
		WardCode:      h.WardCode,
		Address:       h.Address,
		HeadCitizenID: h.HeadCitizenID,
		CreatedAt:     h.CreatedAt,
	}
	for _, m := range members {
		resp.Members = append(resp.Members, model.HouseholdMemberResponse{
			CitizenID:    m.CitizenID,
			Relationship: m.Relationship,
			JoinedAt:     m.JoinedAt,
		})
	}
	return resp
}

func claimsFromCtx(ctx context.Context) *jwtpkg.Claims {
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID == "" {
		return nil
	}
	role, _ := ctx.Value(middleware.ContextKeyUserRole).(string)
	username, _ := ctx.Value(middleware.ContextKeyUsername).(string)
	wardCode, _ := ctx.Value(middleware.ContextKeyWardCode).(string)
	districtCode, _ := ctx.Value(middleware.ContextKeyDistrictCode).(string)
	provinceCode, _ := ctx.Value(middleware.ContextKeyProvinceCode).(string)
	return &jwtpkg.Claims{
		UserID:       userID,
		Username:     username,
		Role:         jwtpkg.Role(role),
		WardCode:     wardCode,
		DistrictCode: districtCode,
		ProvinceCode: provinceCode,
	}
}
