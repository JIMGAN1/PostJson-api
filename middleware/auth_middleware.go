package middleware

import (
	"PostJson/config"
	"PostJson/model"
	"context"
	"log"

	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// AuthContextKey 用于上下文中存储用户信息的键
type AuthContextKey string

const UserContextKey AuthContextKey = "user"

// AuthMiddleware JWT认证中间件
func AuthMiddleware(cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果认证未启用，直接放行
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 从请求头获取token
			authHeader := r.Header.Get(cfg.TokenHeader)
			if authHeader == "" {
				log.Printf("[认证失败] %s %s - 无Token", r.Method, r.URL.Path)
				http.Error(w, "未提供认证令牌", http.StatusUnauthorized)
				return
			}

			// 去除Bearer前缀
			tokenString := strings.TrimPrefix(authHeader, cfg.TokenPrefix)

			// 解析验证token
			claims := &model.Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWTSecret), nil
			})

			// 检查token是否过期
			if err != nil {
				log.Printf("[认证失败] JWT解析错误: %v", err)

				http.Error(w, "验证令牌失败", http.StatusUnauthorized)
				return
			}
			// 检查token是否有效
			if !token.Valid {
				log.Printf("[认证失败] %s %s - Token无效: %v", r.Method, r.URL.Path, err)
				http.Error(w, "无效的认证令牌", http.StatusUnauthorized)
				return
			}

			// 基于用户当前状态检查 token 是否已被“废弃”
			var currentUser *model.User
			for i := range model.MockUsers {
				if model.MockUsers[i].ID == claims.UserID {
					currentUser = &model.MockUsers[i]
					break
				}
			}

			if currentUser == nil {
				log.Printf("[认证失败] %s %s - 用户不存在 (ID=%d)", r.Method, r.URL.Path, claims.UserID)
				http.Error(w, "用户不存在", http.StatusUnauthorized)
				return
			}

			// 基于 token 版本号判定是否失效：登录/改密时用户 TokenVersion 会递增，旧 token 的 version 会不匹配
			if claims.TokenVersion != currentUser.TokenVersion {
				log.Printf("[认证失败] %s %s - Token 已因重新登录或修改密码失效 (user=%s)", r.Method, r.URL.Path, currentUser.Username)
				http.Error(w, "令牌已失效，请重新登录", http.StatusUnauthorized)
				return
			}

			// 记录认证成功的用户
			log.Printf("[认证成功] 用户: %s (ID: %d, 角色: %s) - %s %s",
				claims.Username, claims.UserID, claims.Role, r.Method, r.URL.Path)

			// 将用户信息存入上下文
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext 从上下文中获取用户信息
func GetUserFromContext(ctx context.Context) *model.Claims {
	user, ok := ctx.Value(UserContextKey).(*model.Claims)
	if !ok {
		return nil
	}
	return user
}
