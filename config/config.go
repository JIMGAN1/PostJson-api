package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
}

type RequestConfig struct {
	MaxBodySize    int64         `yaml:"max_body_size"`
	TimeoutSeconds time.Duration `yaml:"timeout_seconds"`
}

type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled"`
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type AppConfig struct {
	Server    ServerConfig    `yaml:"server"`
	Request   RequestConfig   `yaml:"request"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Log       LogConfig       `yaml:"log"`
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 转换为合适的单位
	config.Server.ReadTimeout *= time.Second
	config.Server.WriteTimeout *= time.Second
	config.Server.IdleTimeout *= time.Second
	config.Request.TimeoutSeconds *= time.Second

	return &config, nil
}

// GetAddress 返回服务器地址
func (c *ServerConfig) GetAddress() string {
	return ":" + string(rune(c.Port))
}
