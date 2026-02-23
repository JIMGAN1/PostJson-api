package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
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
	// log.Printf("[配置调试] 最终 ReadTimeout: %v", cfg.Server.ReadTimeout)
	// log.Printf("[配置调试] 最终 WriteTimeout: %v", cfg.Server.WriteTimeout)
	// log.Printf("[配置调试] 最终 IdleTimeout: %v", cfg.Server.IdleTimeout)
	// log.Printf("[配置调试] 最终 TimeoutSeconds: %v", cfg.Request.TimeoutSeconds)
	// log.Printf("[配置调试] 最终 TokenExpiry: %v", cfg.Auth.TokenExpiry)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 设置日志
	if err := setupLogger(cfg.Log); err != nil {
		log.Fatalf("设置日志失败: %v", err)
	}

	// 创建处理器
	jsonHandler := handler.NewJSONHandler(&cfg.Request)
	authHandler := handler.NewAuthHandler(&cfg.Auth)

	// 创建路由器
	mux := http.NewServeMux()

	// 公开接口（不需要认证）
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/auth/register", authHandler.Register) //用户注册接口

	// 需要认证的用户相关接口
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/profile", authHandler.Profile)   // /api/profile 用户数据获取
	apiMux.HandleFunc("/settings", authHandler.Settings) // /api/settings 用户数据修改

	// 业务接口
	apiMux.HandleFunc("/json", jsonHandler.ProcessJSON) // /api/json
	stripAndServeAPIMux := http.StripPrefix("/api", apiMux)
	mux.Handle("/api/", middleware.AuthMiddleware(&cfg.Auth)(stripAndServeAPIMux))

	// 构建中间件链
	var handler http.Handler = mux
	handler = middleware.Security(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logger(handler)
	handler = middleware.Cors(handler)
	if cfg.RateLimit.Enabled {
		limiter := rate.NewLimiter(
			rate.Limit(cfg.RateLimit.RequestsPerSecond),
			cfg.RateLimit.Burst,
		)
		handler = middleware.RateLimiter(limiter)(handler)
	}

	// 根据配置启动HTTP或HTTPS服务
	if cfg.HTTPS.Enabled {
		startHTTPSServer(cfg, handler)
	} else {
		startHTTPServer(cfg, handler)
	}
}

// startHTTPServer 启动HTTP服务器
func startHTTPServer(cfg *config.AppConfig, handler http.Handler) {
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
		log.Printf("HTTP服务器启动在 :%d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	waitForShutdown(srv)
}

// startHTTPSServer 启动HTTPS服务器
func startHTTPSServer(cfg *config.AppConfig, handler http.Handler) {
	// 如果启用了自动证书且配置了域名
	if cfg.HTTPS.AutoCert && cfg.HTTPS.Domain != "" {
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.HTTPS.Domain),
			Cache:      autocert.DirCache("./certs"),
			Email:      cfg.HTTPS.Email,
		}

		srv := &http.Server{
			Addr:    ":https",
			Handler: handler,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			},
			ReadTimeout:    cfg.Server.ReadTimeout,
			WriteTimeout:   cfg.Server.WriteTimeout,
			IdleTimeout:    cfg.Server.IdleTimeout,
			MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		}

		// 启动HTTP重定向服务
		go func() {
			httpServer := &http.Server{
				Addr:    ":http",
				Handler: certManager.HTTPHandler(nil),
			}
			log.Println("HTTP重定向服务启动在 :80")
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP服务错误: %v", err)
			}
		}()

		// 启动HTTPS服务
		go func() {
			log.Printf("HTTPS服务器启动在 :443，域名：%s", cfg.HTTPS.Domain)
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS服务器启动失败: %v", err)
			}
		}()

		waitForShutdown(srv)
	} else {
		// 使用证书文件
		srv := &http.Server{
			Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
			Handler:        handler,
			ReadTimeout:    cfg.Server.ReadTimeout,
			WriteTimeout:   cfg.Server.WriteTimeout,
			IdleTimeout:    cfg.Server.IdleTimeout,
			MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		}

		go func() {
			log.Printf("HTTPS服务器启动在 :%d", cfg.Server.Port)
			if err := srv.ListenAndServeTLS(cfg.HTTPS.CertFile, cfg.HTTPS.KeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("服务器启动失败: %v", err)
			}
		}()

		waitForShutdown(srv)
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown(srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

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

// setupLogger 设置日志（按天分割）
func setupLogger(cfg config.LogConfig) error {
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	if cfg.Output == "stdout" {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		return nil
	}

	go func() {
		for {
			today := time.Now().Format("2006-01-02")
			logFile := fmt.Sprintf("./logs/app-%s.log", today)

			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				log.SetOutput(io.MultiWriter(os.Stdout, file))
				log.Printf("日志文件切换到: %s", logFile)
			}

			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))

			if file != nil {
				file.Close()
			}
		}
	}()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return nil
}
