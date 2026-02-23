package handler

import (
	"PostJson/config"
	"PostJson/middleware"
	"PostJson/model"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	config *config.AuthConfig
}

func NewAuthHandler(cfg *config.AuthConfig) *AuthHandler {
	return &AuthHandler{
		config: cfg,
	}
}

// Login 用户登录
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受POST请求", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var loginReq model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "无效的请求数据", http.StatusBadRequest)
		return
	}

	// 验证用户（这里用模拟数据，实际应从数据库查询）
	var foundUser *model.User
	for i := range model.MockUsers {
		if model.MockUsers[i].Username == loginReq.Username {
			foundUser = &model.MockUsers[i]
			break
		}
	}

	if foundUser == nil {
		http.Error(w, "用户名不存在", http.StatusUnauthorized)
		return
	}

	// 验证密码
	if !model.CheckPassword(loginReq.Password, foundUser.Password) {
		http.Error(w, "密码错误", http.StatusUnauthorized)
		return
	}

	// 登录成功，刷新该用户的 token 基准时间
	now := time.Now()
	foundUser.LastTokenResetAt = now
	foundUser.TokenVersion++

	// 生成JWT token
	expirationTime := now.Add(h.config.TokenExpiry)
	claims := &model.Claims{
		UserID:   foundUser.ID,
		Username: foundUser.Username,
		Role:     foundUser.Role,
		TokenVersion: foundUser.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "PostJson",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.config.JWTSecret))
	if err != nil {
		http.Error(w, "生成令牌失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := model.LoginResponse{
		Token: tokenString,
		User:  *foundUser,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Register 用户注册（可选）
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只接受POST请求", http.StatusMethodNotAllowed)
		return
	}

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求数据", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" || req.Password == "" || req.Email == "" {
		http.Error(w, "用户名、密码和邮箱不能为空", http.StatusBadRequest)
		return
	}

	// 检查用户名是否已存在（基于模拟用户数据）
	for _, u := range model.MockUsers {
		if u.Username == req.Username {
			http.Error(w, "用户名已存在", http.StatusBadRequest)
			return
		}
	}

	// 哈希密码
	hashedPwd, err := model.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "密码处理失败", http.StatusInternalServerError)
		return
	}

	// 生成新用户ID
	newID := 1
	for _, u := range model.MockUsers {
		if u.ID >= newID {
			newID = u.ID + 1
		}
	}

	now := time.Now()
	newUser := model.User{
		ID:              newID,
		Username:        req.Username,
		Password:        hashedPwd,
		Email:           req.Email,
		Role:            model.RoleUser,
		CreatedAt:       now,
		LastTokenResetAt: now,
		TokenVersion:    1,
	}

	// 写入模拟“数据库”
	model.MockUsers = append(model.MockUsers, newUser)

	// 注册成功后直接生成 JWT，相当于自动登录
	expirationTime := now.Add(h.config.TokenExpiry)
	claims := &model.Claims{
		UserID:   newUser.ID,
		Username: newUser.Username,
		Role:     newUser.Role,
		TokenVersion: newUser.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "PostJson",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.config.JWTSecret))
	if err != nil {
		http.Error(w, "生成令牌失败", http.StatusInternalServerError)
		return
	}

	resp := model.LoginResponse{
		Token: tokenString,
		User:  newUser,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Profile 获取用户信息（需要认证）
func (h *AuthHandler) Profile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "未认证", http.StatusUnauthorized)
		return
	}

	response := map[string]interface{}{
		"user_id":  user.UserID,
		"username": user.Username,
		"role":     user.Role,
		"expires":  user.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type settingsUpdateRequest struct {
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

// Settings 修改用户信息（需要认证）
func (h *AuthHandler) Settings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "只接受PUT或PATCH请求", http.StatusMethodNotAllowed)
		return
	}

	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "未认证", http.StatusUnauthorized)
		return
	}

	var req settingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求数据", http.StatusBadRequest)
		return
	}

	if req.Email == nil && req.Password == nil {
		http.Error(w, "未提供任何可更新字段", http.StatusBadRequest)
		return
	}

	var hashed string
	if req.Password != nil {
		if strings.TrimSpace(*req.Password) == "" {
			http.Error(w, "密码不能为空", http.StatusBadRequest)
			return
		}
		h, err := model.HashPassword(*req.Password)
		if err != nil {
			http.Error(w, "密码处理失败", http.StatusInternalServerError)
			return
		}
		hashed = h
	}

	updated := false
	now := time.Now()
	var outUser model.User
	for i := range model.MockUsers {
		if model.MockUsers[i].ID != claims.UserID {
			continue
		}

		if req.Email != nil {
			email := strings.TrimSpace(*req.Email)
			if email == "" {
				http.Error(w, "邮箱不能为空", http.StatusBadRequest)
				return
			}
			model.MockUsers[i].Email = email
		}
		if req.Password != nil {
			model.MockUsers[i].Password = hashed
			// 修改密码时刷新 token 基准时间，并递增 token 版本号
			model.MockUsers[i].LastTokenResetAt = now
			model.MockUsers[i].TokenVersion++
		}

		outUser = model.MockUsers[i]
		updated = true
		break
	}

	if !updated {
		http.Error(w, "用户不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "更新成功",
		"user":    outUser,
	})
}
