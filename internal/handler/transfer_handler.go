package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"population-service/internal/model"
	"population-service/internal/service"
	"population-service/pkg/response"
)

type TransferHandler struct {
	svc service.TransferService
}

func NewTransferHandler(svc service.TransferService) *TransferHandler {
	return &TransferHandler{svc: svc}
}

// ─── Household endpoints ─────────────────────────────────────

// CreateHousehold POST /households
func (h *TransferHandler) CreateHousehold(c *gin.Context) {
	var req model.CreateHouseholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", err.Error())
		return
	}
	res, err := h.svc.CreateHousehold(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, res)
}

// GetHousehold GET /households/:id
func (h *TransferHandler) GetHousehold(c *gin.Context) {
	id := c.Param("id")
	res, err := h.svc.GetHousehold(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "household not found")
		return
	}
	response.OK(c, res)
}

// ListHouseholds GET /households
func (h *TransferHandler) ListHouseholds(c *gin.Context) {
	var filter model.ListHouseholdFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, "INVALID_FILTER", err.Error())
		return
	}
	res, err := h.svc.ListHouseholds(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}

// AddHouseholdMember POST /households/:id/members
func (h *TransferHandler) AddHouseholdMember(c *gin.Context) {
	householdID := c.Param("id")
	var req model.AddHouseholdMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.svc.AddHouseholdMember(c.Request.Context(), householdID, req); err != nil {
		response.BadRequest(c, "ADD_MEMBER_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "thêm thành viên thành công"})
}

// ─── Transfer endpoints ──────────────────────────────────────

// CreateTransfer POST /transfers
func (h *TransferHandler) CreateTransfer(c *gin.Context) {
	var req model.CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", err.Error())
		return
	}
	res, err := h.svc.CreateTransferRequest(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, res)
}

// GetTransfer GET /transfers/:id
func (h *TransferHandler) GetTransfer(c *gin.Context) {
	id := c.Param("id")
	res, err := h.svc.GetTransferRequest(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "transfer request not found" {
			response.NotFound(c, err.Error())
			return
		}
		if isForbidenErr(err) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}

// ListTransfers GET /transfers
func (h *TransferHandler) ListTransfers(c *gin.Context) {
	var filter model.ListTransferFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, "INVALID_FILTER", err.Error())
		return
	}
	res, err := h.svc.ListTransferRequests(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}

// ApproveTransfer POST /transfers/:id/approve
func (h *TransferHandler) ApproveTransfer(c *gin.Context) {
	id := c.Param("id")
	var req model.ApproveTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.svc.ApproveTransfer(c.Request.Context(), id, req); err != nil {
		if isForbidenErr(err) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
			return
		}
		response.BadRequest(c, "APPROVE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "phê duyệt thành công"})
}

// ForceApproveTransfer POST /transfers/:id/force-approve (super_admin only)
func (h *TransferHandler) ForceApproveTransfer(c *gin.Context) {
	id := c.Param("id")
	var req model.ForceApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.svc.ForceApproveTransfer(c.Request.Context(), id, req); err != nil {
		response.BadRequest(c, "FORCE_APPROVE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "force approve thành công — đã ghi audit log"})
}

// GetResidenceHistory GET /citizens/:id/residence-history
func (h *TransferHandler) GetResidenceHistory(c *gin.Context) {
	citizenID := c.Param("id")
	res, err := h.svc.GetResidenceHistory(c.Request.Context(), citizenID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}

func isForbidenErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) >= 9 && msg[:9] == "forbidden"
}