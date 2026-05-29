package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"population-service/internal/model"
	"population-service/internal/service"
	"population-service/pkg/response"
)

// AuditHandler xử lý các request liên quan đến audit log
type AuditHandler struct {
	svc service.AuditService
}

// NewAuditHandler tạo mới audit handler
func NewAuditHandler(svc service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List trả về danh sách audit log với filter và phân trang
//
//	GET /api/v1/audit-logs
//	Query params:
//	  citizen_id  - lọc theo công dân
//	  action      - create | update | delete
//	  changed_by  - lọc theo user_id
//	  from        - "2006-01-02" hoặc RFC3339
//	  to          - "2006-01-02" hoặc RFC3339
//	  page        - trang (default 1)
//	  page_size   - kích thước trang (default 20, max 100)
func (h *AuditHandler) List(c *gin.Context) {
	var filter model.ListAuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, "INVALID_QUERY", err.Error())
		return
	}

	result, err := h.svc.GetLogs(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result.Data,
		"meta": gin.H{
			"total":     result.Total,
			"page":      result.Page,
			"page_size": result.PageSize,
		},
	})
}