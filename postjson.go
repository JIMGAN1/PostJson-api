package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"PostJson/config"
	"PostJson/handler"
	"PostJson/middleware"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "./config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 设置日志
	if err := setupLogger(cfg.Log); err != nil {
		log.Fatalf("设置日志失败: %v", err)
	}

	// 创建处理器
	jsonHandler := handler.NewJSONHandler(&cfg.Request)

	// 创建路由器
	mux := http.NewServeMux()
	mux.HandleFunc("/api-json", jsonHandler.ProcessJSON)
	mux.HandleFunc("/health", healthCheck)
	//测试panic
	// mux.HandleFunc("/test-panic", jsonHandler.TestPanic)
	// 构建中间件链
	var handler http.Handler = mux

	// 安全中间件
	handler = middleware.Security(handler)

	// 恢复中间件（防止 panic）
	handler = middleware.Recovery(handler)

	// 日志中间件
	handler = middleware.Logger(handler)

	// 跨域中间件
	handler = middleware.Cors(handler)

	// 限流中间件（如果启用）
	if cfg.RateLimit.Enabled {
		limiter := rate.NewLimiter(
			rate.Limit(cfg.RateLimit.RequestsPerSecond),
			cfg.RateLimit.Burst,
		)
		handler = middleware.RateLimiter(limiter)(handler)
	}

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        handler,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// 优雅关闭
	go func() {
		log.Printf("服务器启动在 :%d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	// 设置关闭超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务器强制关闭: %v", err)
	}

	log.Println("服务器已关闭")
}

// healthCheck 健康检查
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// setupLogger 设置日志（纯手工按天分割）
func setupLogger(cfg config.LogConfig) error {
	// 创建日志目录
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 如果配置输出到 stdout
	if cfg.Output == "stdout" {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		return nil
	}

	// 启动一个 goroutine 每天切换日志文件
	go func() {
		for {
			// 生成当天的日志文件名
			today := time.Now().Format("2006-01-02")
			logFile := fmt.Sprintf("./logs/app-%s.log", today)

			// 打开今天的日志文件
			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				// 设置日志输出到文件和控制台
				log.SetOutput(io.MultiWriter(os.Stdout, file))
				log.Printf("日志文件切换到: %s", logFile)
			}

			// 计算到明天零点的时间
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))

			// 关闭今天的文件
			if file != nil {
				file.Close()
			}
		}
	}()

	// 初始设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 让主程序继续执行，不阻塞
	return nil
}
