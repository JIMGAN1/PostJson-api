package model

import "time"

// RequestData 请求数据结构
type RequestData struct {
	Name    string   `json:"name" validate:"required,min=2,max=50"`
	Age     int      `json:"age" validate:"required,min=0,max=150"`
	Email   string   `json:"email" validate:"required,email"`
	Tags    []string `json:"tags" validate:"max=10"`
	Created string   `json:"created"` // ISO 8601 格式的时间字符串
}

// Validate 自定义验证
func (r *RequestData) Validate() error {
	if r.Name == "" {
		return NewValidationError("name", "姓名不能为空")
	}
	if len(r.Name) > 50 {
		return NewValidationError("name", "姓名不能超过50个字符")
	}
	if r.Age < 0 || r.Age > 150 {
		return NewValidationError("age", "年龄必须在0-150之间")
	}
	if r.Email == "" {
		return NewValidationError("email", "邮箱不能为空")
	}
	if len(r.Tags) > 10 {
		return NewValidationError("tags", "标签不能超过10个")
	}

	// 验证时间格式
	if r.Created != "" {
		if _, err := time.Parse(time.RFC3339, r.Created); err != nil {
			return NewValidationError("created", "时间格式必须是RFC3339")
		}
	}

	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// Response 标准响应结构
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError API错误
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(message string, data interface{}) *Response {
	return &Response{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code, message, details string) *Response {
	return &Response{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}
