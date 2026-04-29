// Package dto 定义 REST API 的数据传输对象
package dto

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListResponse 分页列表响应
type ListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Total   int64       `json:"total,omitempty"`
	Offset  int         `json:"offset,omitempty"`
	Limit   int         `json:"limit,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// NewResponse 创建成功响应
func NewResponse(data interface{}) Response {
	return Response{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

// NewListResponse 创建分页列表响应
func NewListResponse(data interface{}, total int64, offset, limit int) ListResponse {
	return ListResponse{
		Code:    200,
		Message: "success",
		Data:    data,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code int, message string, err error) ErrorResponse {
	resp := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}
