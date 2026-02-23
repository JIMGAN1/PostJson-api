package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"PostJson/config"
	"PostJson/model"
)

type JSONHandler struct {
	config *config.RequestConfig
}

func NewJSONHandler(cfg *config.RequestConfig) *JSONHandler {
	return &JSONHandler{
		config: cfg,
	}
}

// ProcessJSON 处理 JSON 请求
func (h *JSONHandler) ProcessJSON(w http.ResponseWriter, r *http.Request) {
	// 1. 方法检查
	if r.Method != http.MethodPost {
		h.sendJSONError(w, "METHOD_NOT_ALLOWED", "只接受 POST 请求", nil, http.StatusMethodNotAllowed)
		return
	}

	// 2. Content-Type 检查
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		h.sendJSONError(w, "INVALID_CONTENT_TYPE", "Content-Type 必须是 application/json", nil, http.StatusUnsupportedMediaType)
		return
	}

	// 3. 设置超时
	ctx, cancel := context.WithTimeout(r.Context(), h.config.TimeoutSeconds)
	defer cancel()
	r = r.WithContext(ctx)

	// 4. 限制请求体大小
	r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxBodySize)

	// 5. 读取并处理 BOM
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			h.sendJSONError(w, "REQUEST_TOO_LARGE",
				fmt.Sprintf("请求体过大，最大 %d MB", h.config.MaxBodySize/1024/1024),
				nil, http.StatusRequestEntityTooLarge)
		} else {
			h.sendJSONError(w, "READ_ERROR", "读取请求失败", err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer r.Body.Close()

	// 6. 移除可能的 BOM
	body = h.trimBOM(body)

	// 7. 验证 JSON 格式
	if !json.Valid(body) {
		h.sendJSONError(w, "INVALID_JSON", "无效的 JSON 格式", nil, http.StatusBadRequest)
		return
	}

	// 8. 解析 JSON
	var reqData model.RequestData
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&reqData); err != nil {
		h.handleJSONError(w, err)
		return
	}

	// 9. 检查是否有多余数据
	if decoder.More() {
		h.sendJSONError(w, "EXTRA_DATA", "请求体包含多余的数据", nil, http.StatusBadRequest)
		return
	}

	// 10. 业务验证
	if err := reqData.Validate(); err != nil {
		if validationErr, ok := err.(*model.ValidationError); ok {
			h.sendJSONError(w, "VALIDATION_ERROR", validationErr.Message,
				map[string]string{"field": validationErr.Field}, http.StatusBadRequest)
		} else {
			h.sendJSONError(w, "VALIDATION_ERROR", err.Error(), nil, http.StatusBadRequest)
		}
		return
	}

	// 11. 模拟业务处理
	response := map[string]interface{}{
		"received":  reqData,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("你好，%s！", reqData.Name),
	}

	// 12. 成功响应
	h.sendJSONResponse(w, model.NewSuccessResponse("处理成功", response), http.StatusOK)
}

// trimBOM 移除 UTF-8 BOM
func (h *JSONHandler) trimBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

// handleJSONError 处理 JSON 解析错误
func (h *JSONHandler) handleJSONError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *json.SyntaxError:
		h.sendJSONError(w, "JSON_SYNTAX_ERROR",
			fmt.Sprintf("JSON 语法错误 (位置 %d)", e.Offset),
			map[string]interface{}{"offset": e.Offset},
			http.StatusBadRequest)
	case *json.UnmarshalTypeError:
		h.sendJSONError(w, "JSON_TYPE_ERROR",
			fmt.Sprintf("字段 '%s' 类型错误，期望 %s", e.Field, e.Type.String()),
			map[string]interface{}{"field": e.Field, "expected_type": e.Type.String()},
			http.StatusBadRequest)
	default:
		h.sendJSONError(w, "JSON_PARSE_ERROR", "JSON 解析失败", err.Error(), http.StatusBadRequest)
	}
}

// sendJSONResponse 发送 JSON 响应
func (h *JSONHandler) sendJSONResponse(w http.ResponseWriter, response *model.Response, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "响应编码失败", http.StatusInternalServerError)
	}
}

// sendJSONError 发送 JSON 错误响应
func (h *JSONHandler) sendJSONError(w http.ResponseWriter, code, message string, details interface{}, status int) {
	apiError := &model.APIError{
		Code:    code,
		Message: message,
	}

	if details != nil {
		if str, ok := details.(string); ok {
			apiError.Details = str
		} else {
			// 如果是结构体，尝试 JSON 序列化
			if detailsBytes, err := json.Marshal(details); err == nil {
				apiError.Details = string(detailsBytes)
			}
		}
	}

	response := &model.Response{
		Success: false,
		Error:   apiError,
	}

	h.sendJSONResponse(w, response, status)
}
