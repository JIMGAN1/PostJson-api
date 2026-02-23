package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// 用户角色
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User 用户模型
type User struct {
	ID              int       `json:"id"`
	Username        string    `json:"username"`
	Password        string    `json:"-"` // 不输出到JSON
	Email           string    `json:"email"`
	Role            string    `json:"role"`
	CreatedAt       time.Time `json:"created_at"`
	LastTokenResetAt time.Time `json:"-"` // 最近一次登录或修改密码时间，用于判定旧token是否失效
	TokenVersion    int       `json:"-"` // 用户token版本号：登录/改密时递增，用于让旧token失效
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Claims JWT声明
type Claims struct {
	UserID        int    `json:"user_id"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	TokenVersion  int    `json:"token_version"`
	jwt.RegisteredClaims
}

// HashPassword 密码哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// 模拟用户数据库（实际应该用真实数据库）
var MockUsers = []User{
	{
		ID:              1,
		Username:        "admin",
		Password:        "$2a$10$RWxx/wUjnWokXMfHzZupRusrhhnlgXZ8rsoEz1RzixQcRhhee/1mi", // password: admin123
		Email:           "admin@example.com",
		Role:            RoleAdmin,
		CreatedAt:       time.Now(),
		LastTokenResetAt: time.Now(),
		TokenVersion:    0,
	},
	{
		ID:              2,
		Username:        "user",
		Password:        "$2a$10$mpypAw/I9LVuwZgW76VNz.dmdaSOesRM4yj.yn9K6bJXj2TP0AyPm", // password: user123
		Email:           "user@example.com",
		Role:            RoleUser,
		CreatedAt:       time.Now(),
		LastTokenResetAt: time.Now(),
		TokenVersion:    0,
	},
}

// CheckPassword 验证密码
// func CheckPassword(password, hash string) bool {
// 	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

// 	// 添加详细日志
// 	if err != nil {
// 		log.Printf("[密码验证失败] 密码长度=%d, 哈希长度=%d, 错误: %v",
// 			len(password), len(hash), err)

// 		// 检查哈希格式
// 		if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") {
// 			log.Printf("哈希格式看起来正确")
// 		} else {
// 			log.Printf("哈希格式错误！应该以$2a$或$2b$开头，实际是: %s", hash[:10])
// 		}
// 	} else {
// 		log.Printf("[密码验证成功] 用户验证通过")
// 	}

// 	return err == nil
// }
