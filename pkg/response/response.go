package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response là cấu trúc response chuẩn của API
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorInfo chứa chi tiết lỗi
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta chứa thông tin pagination
type Meta struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// OK trả về 200 với data
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// OKWithMeta trả về 200 với data + pagination meta
func OKWithMeta(c *gin.Context, data interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// Created trả về 201
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "Created successfully",
		Data:    data,
	})
}

// NoContent trả về 204
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// BadRequest trả về 400
func BadRequest(c *gin.Context, code, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// NotFound trả về 404
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    "NOT_FOUND",
			Message: message,
		},
	})
}

// InternalError trả về 500
func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    "INTERNAL_ERROR",
			Message: message,
		},
	})
}

// Conflict trả về 409
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    "CONFLICT",
			Message: message,
		},
	})
}
