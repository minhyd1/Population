package handler

import (
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"population-service/internal/model"
	"population-service/internal/service"
	"population-service/pkg/crypto"
	"population-service/pkg/response"
)

type CitizenHandler struct {
	svc service.CitizenService
	enc *crypto.Encryptor
}

func NewCitizenHandler(svc service.CitizenService, enc *crypto.Encryptor) *CitizenHandler {
	return &CitizenHandler{svc: svc, enc: enc}
}

func (h *CitizenHandler) Create(c *gin.Context) {
	var req model.CreateCitizenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
    		response.Conflict(c, "Số CCCD/CMND đã tồn tại trong hệ thống")
   	 		return
		}
		if strings.Contains(err.Error(), "national_id already exists") {
			response.Conflict(c, err.Error()) // trả về lỗi chi tiết để debug
			return
		}
    response.InternalError(c, err.Error())
    return
}
	response.Created(c, result)
}

func (h *CitizenHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	result, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if result == nil {
		response.NotFound(c, "Citizen not found")
		return
	}
	response.OK(c, result)
}

func (h *CitizenHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req model.UpdateCitizenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			response.Conflict(c, "Số CCCD/CMND đã tồn tại trong hệ thống")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	if result == nil {
		response.NotFound(c, "Citizen not found")
		return
	}
	response.OK(c, result)
}

func (h *CitizenHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

func (h *CitizenHandler) List(c *gin.Context) {
	var filter model.ListCitizenFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, "INVALID_QUERY", err.Error())
		return
	}
	result, err := h.svc.List(c.Request.Context(), filter)
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

func (h *CitizenHandler) GetPopulationStats(c *gin.Context) {
	stats, err := h.svc.GetPopulationStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, stats)
}

func (h *CitizenHandler) GetPopulationStatByProvince(c *gin.Context) {
	provinceCode := c.Param("province_code")
	stat, err := h.svc.GetPopulationStatByProvince(c.Request.Context(), provinceCode)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if stat == nil {
		response.NotFound(c, "No population data found for this province")
		return
	}
	response.OK(c, stat)
}

func (h *CitizenHandler) GetEncryptionMeta(c *gin.Context) {
	response.OK(c, model.EncryptionMetaResponse{
		Algorithm:  "AES-256-GCM",
		KeyVersion: "v1",
		Fields:     crypto.SensitiveFields,
	})
}
