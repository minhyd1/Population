package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	// Tự động giới hạn scope theo role
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

	level := determineApprovalLevel(fromHH, toHH)

	claims := claimsFromCtx(ctx)
	createdBy := ""
	if claims != nil {
		createdBy = claims.UserID
	}

	treq := &model.TransferRequest{
		CitizenID:       req.CitizenID,
		FromHouseholdID: req.FromHouseholdID,
		ToHouseholdID:   req.ToHouseholdID,
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
		// Tự động giới hạn scope
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
func (s *transferSvc) ApproveTransfer(ctx context.Context, requestID string, req model.ApproveTransferRequest) error {
	if req.Decision == model.ApprovalDecisionRejected && req.RejectReason == "" {
		return fmt.Errorf("reject_reason bắt buộc khi từ chối")
	}

	treq, err := s.transferRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("transfer request not found: %w", err)
	}
	if treq.Status != model.TransferStatusPending {
		return fmt.Errorf("yêu cầu không ở trạng thái pending")
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

	approval, err := s.transferRepo.GetPendingApprovalForUnit(ctx, requestID, unitCode)
	if err != nil {
		return fmt.Errorf("không tìm thấy phiếu phê duyệt cho đơn vị %s: %w", unitCode, err)
	}

	approval.Decision = req.Decision
	approval.ApprovedBy = &claims.UserID
	if req.RejectReason != "" {
		approval.RejectReason = &req.RejectReason
	}

	if err := s.transferRepo.UpdateApproval(ctx, approval); err != nil {
		return fmt.Errorf("update approval: %w", err)
	}

	// Kiểm tra tất cả đã duyệt chưa
	if req.Decision == model.ApprovalDecisionApproved {
		allApprovals, err := s.transferRepo.GetApprovalsByRequestID(ctx, requestID)
		if err != nil {
			return err
		}
		if checkAllApproved(allApprovals) {
			fromHH, err := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
			if err != nil {
				return err
			}
			return s.executeTransfer(ctx, treq, fromHH)
		}
	} else if req.Decision == model.ApprovalDecisionRejected {
		// Một bên từ chối → toàn bộ yêu cầu bị từ chối
		return s.transferRepo.UpdateRequestStatus(ctx, requestID, model.TransferStatusRejected)
	}

	return nil
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

	// Ghi audit log đặc biệt với action FORCE_APPROVE
	_ = s.auditRepo.Insert(ctx, &model.AuditLog{
		CitizenID:     treq.CitizenID,
		Action:        model.AuditAction("force_approve_transfer"),
		ChangedBy:     claims.UserID,
		ChangedByName: claims.Username,
		ChangedByRole: string(claims.Role),
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

// ─── ExecuteTransfer (Transaction) ─────────────────────────

// executeTransfer thực hiện chuyển hộ khẩu trong một transaction duy nhất:
// 1. Ghi residence_histories
// 2. Xóa khỏi hộ cũ
// 3. Thêm vào hộ mới
// 4. Cập nhật citizen (ward/district/province)
// 5. Cập nhật request status = completed
// 6. Ghi audit log
func (s *transferSvc) executeTransfer(ctx context.Context, treq *model.TransferRequest, fromHH *model.Household) error {
	toHH, err := s.householdRepo.GetByID(ctx, treq.ToHouseholdID)
	if err != nil {
		return err
	}

	return s.transferRepo.WithTx(ctx, func(txRepo repository.TransferRepository) error {
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

		// 3. Thêm vào hộ mới (quan hệ mặc định là "thành viên")
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

		// 5. Cập nhật request
		if err := txRepo.CompleteRequest(ctx, treq.ID); err != nil {
			return fmt.Errorf("complete request: %w", err)
		}

		// 6. Audit log
		_ = s.auditRepo.Insert(ctx, &model.AuditLog{
			CitizenID:     treq.CitizenID,
			Action:        model.AuditAction("transfer_completed"),
			ChangedBy:     treq.CreatedBy,
			ChangedByName: "system",
			ChangedByRole: "transfer_workflow",
		})

		_ = fromHH // used for reference
		return nil
	})
}

// ─── Helpers ────────────────────────────────────────────────

// determineApprovalLevel tự động xác định cấp phê duyệt cần thiết
func determineApprovalLevel(from, to *model.Household) model.ApprovalLevel {
	if from.WardCode == to.WardCode {
		return model.ApprovalLevelNone // cùng phường → không cần workflow
	}
	if from.DistrictCode == to.DistrictCode {
		return model.ApprovalLevelWard // khác phường cùng quận → cần 2 phường duyệt
	}
	if from.ProvinceCode == to.ProvinceCode {
		return model.ApprovalLevelDistrict // khác quận cùng tỉnh → cần 2 quận duyệt
	}
	return model.ApprovalLevelProvince // khác tỉnh → cần 2 tỉnh duyệt
}

// buildApprovals sinh danh sách phiếu phê duyệt theo cấp
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

// checkAllApproved kiểm tra tất cả phiếu phê duyệt đã được duyệt chưa
func checkAllApproved(approvals []*model.TransferApproval) bool {
	for _, a := range approvals {
		if a.Decision != model.ApprovalDecisionApproved {
			return false
		}
	}
	return len(approvals) > 0
}

// getUnitCode lấy unit code phù hợp với role của user
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

// canManageUnit kiểm tra user có quyền quản lý đơn vị hành chính này không
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

// canViewTransfer kiểm tra user có quyền xem yêu cầu chuyển hộ không
func (s *transferSvc) canViewTransfer(ctx context.Context, claims *jwtpkg.Claims, treq *model.TransferRequest) error {
	if claims.Role == jwtpkg.RoleSuperAdmin || claims.Role == jwtpkg.RoleNationalManager ||
		claims.Role == jwtpkg.RoleAuditor {
		return nil
	}
	// Kiểm tra hộ nơi đi hoặc nơi đến có trong địa bàn của user không
	fromHH, _ := s.householdRepo.GetByID(ctx, treq.FromHouseholdID)
	toHH, _ := s.householdRepo.GetByID(ctx, treq.ToHouseholdID)
	switch claims.Role {
	case jwtpkg.RoleWardOfficer:
		if (fromHH != nil && fromHH.WardCode == claims.WardCode) ||
			(toHH != nil && toHH.WardCode == claims.WardCode) {
			return nil
		}
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
	case jwtpkg.RoleCitizenSelf:
		if treq.CitizenID == claims.UserID { // so sánh citizen_id
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

// claimsFromCtx lấy JWT claims từ context (được inject bởi middleware)
func claimsFromCtx(ctx context.Context) *jwtpkg.Claims {
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID == "" {
		return nil
	}
	role, _ := ctx.Value(middleware.ContextKeyUserRole).(string)
	username, _ := ctx.Value(middleware.ContextKeyUsername).(string)
	return &jwtpkg.Claims{
		UserID:   userID,
		Username: username,
		Role:     jwtpkg.Role(role),
	}
}