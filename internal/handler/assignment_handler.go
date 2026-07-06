package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"population-service/internal/model"
	"population-service/internal/service"
	"population-service/pkg/middleware"
	"population-service/pkg/response"
)

// AssignmentHandler xử lý các API liên quan đến phân công cán bộ
type AssignmentHandler struct {
	svc service.AssignmentService
}

func NewAssignmentHandler(svc service.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{svc: svc}
}

// AssignUser POST /admin/assignments
// Phân công user vào đơn vị hành chính
func (h *AssignmentHandler) AssignUser(c *gin.Context) {
	var req model.AssignUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	claims := middleware.GetClaims(c)
	if claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	assignment, err := h.svc.AssignUser(c.Request.Context(), req, claims.UserID)
	if err != nil {
		response.BadRequest(c, "ASSIGN_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": assignment})
}

// EndAssignment POST /admin/assignments/:id/end
// Kết thúc phân công (điều chuyển, nghỉ việc)
func (h *AssignmentHandler) EndAssignment(c *gin.Context) {
	id := c.Param("id")

	var req model.EndAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		response.BadRequest(c, "INVALID_DATE", "end_date không hợp lệ, định dạng: 2006-01-02")
		return
	}

	if err := h.svc.EndAssignment(c.Request.Context(), id, endDate, req.Note); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "phân công đã kết thúc"})
}

// GetUserAssignments GET /admin/users/:id/assignments?history=true
// Lịch sử phân công của user
func (h *AssignmentHandler) GetUserAssignments(c *gin.Context) {
	userID := c.Param("id")
	historyOnly := c.Query("history") == "true"

	var result []model.UserAssignmentResponse
	var err error

	if historyOnly {
		result, err = h.svc.GetHistory(c.Request.Context(), userID)
	} else {
		result, err = h.svc.GetActiveAssignments(c.Request.Context(), userID)
	}

	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// GetUnitOfficers GET /admin/units/:code/officers?at=2024-01-01
// Cán bộ đang phụ trách đơn vị. Truyền ?at=YYYY-MM-DD để truy vấn lịch sử.
func (h *AssignmentHandler) GetUnitOfficers(c *gin.Context) {
	unitCode := c.Param("code")

	atStr := c.Query("at")
	if atStr != "" {
		at, err := time.Parse("2006-01-02", atStr)
		if err != nil {
			response.BadRequest(c, "INVALID_DATE", "at không hợp lệ, định dạng: 2006-01-02")
			return
		}
		result, err := h.svc.GetOfficerAtTime(c.Request.Context(), unitCode, at)
		if err != nil {
			response.InternalError(c, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": result, "at": atStr})
		return
	}

	result, err := h.svc.GetActiveOfficersByUnit(c.Request.Context(), unitCode)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
