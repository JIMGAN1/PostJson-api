package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"golang.org/x/time/rate"
)

// Recovery 恢复中间件 - 防止 panic 导致服务崩溃
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				http.Error(w, "内部服务器错误", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseCapture 同时捕获状态码和响应体
type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	// 同时写入原始ResponseWriter和缓冲区
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// Logger 日志中间件（记录请求数据）
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// ----- 记录请求数据（但不影响后续处理）-----
		var requestBody []byte
		var requestBodyStr string

		// 读取并记录请求体（仅对POST/PUT/PATCH请求）
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			if r.Body != nil {
				// 读取请求体
				requestBody, _ = io.ReadAll(r.Body)
				// 重新构造Body，供后续处理器使用
				r.Body = io.NopCloser(bytes.NewBuffer(requestBody))

				// 限制日志中显示的body长度
				requestBodyStr = string(requestBody)
				if len(requestBodyStr) > 500 {
					requestBodyStr = requestBodyStr[:500] + "...(截断)"
				}
			}
		}

		// 创建响应捕获器
		capture := &responseCapture{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		// 处理请求
		next.ServeHTTP(capture, r)

		// ----- 记录响应数据 -----
		var responseBodyStr string
		if capture.body.Len() > 0 {
			responseBodyStr = capture.body.String()
			if len(responseBodyStr) > 500 {
				responseBodyStr = responseBodyStr[:500] + "...(截断)"
			}
		}

		// 计算耗时
		duration := time.Since(start)

		// ----- 记录请求数据 -----
		log.Printf("[%s] %s %s %d %v %s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			capture.statusCode,
			duration,
			r.UserAgent(),
		)

		// ----- 第二行：详细数据（如果有POST数据或错误）-----
		// 只在以下情况打印详细数据：
		// 1. 有POST/PUT/PATCH数据
		// 2. 响应状态码不是200
		// 3. 是登录接口（方便调试）
		if requestBodyStr != "" || capture.statusCode != http.StatusOK || r.URL.Path == "/auth/login" {
			log.Printf("  └─ 请求数据: %s", requestBodyStr)

			if responseBodyStr != "" {
				// 尝试格式化JSON响应
				var prettyJSON bytes.Buffer
				if json.Valid([]byte(responseBodyStr)) {
					json.Indent(&prettyJSON, []byte(responseBodyStr), "", "  ")
					// 把多行JSON压缩成一行显示
					oneLineJSON := bytes.ReplaceAll(prettyJSON.Bytes(), []byte("\n"), []byte(" "))
					oneLineJSON = bytes.ReplaceAll(oneLineJSON, []byte("  "), []byte(" "))
					log.Printf("  └─ 响应数据: %s", string(oneLineJSON))
				} else {
					log.Printf("  └─ 响应数据: %s", responseBodyStr)
				}
			}
		}
	})
}

// RateLimiter 限流中间件
func RateLimiter(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "请求过于频繁，请稍后再试", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Cors 跨域中间件
func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Security 安全中间件
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 添加安全头
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		next.ServeHTTP(w, r)
	})
}
