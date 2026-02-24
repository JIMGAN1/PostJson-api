# PostJson API 🚀

一个轻量级、高性能的 Go 语言原生 HTTP API 服务，支持 JSON 格式的数据处理。

## ✨ 核心功能

- 🔐 **JWT 认证** - 登录返回Token，调api需Bearer Token验证，登录/改密后旧token自动失效
- 🌐 **HTTPS 支持** - 自签名证书 / Let's Encrypt 自动证书，支持http/https切换
- 🛡️ **限流保护** - 令牌桶算法，可配置 QPS
- 📝 **日志系统** - 按天自动分割，同时输出控制台和文件
- ✅ **安全中间件** - 安全响应头、跨域支持
- 🔄 **优雅关闭** - 等待现有请求处理完再退出
- ⚙️ **配置外置** - YAML 配置文件，支持多环境
- ✅ **健康检查** - `/health` 端点监控服务状态

## 🚀 快速开始

### 前置要求
- Go 1.21+
- Git

### 接口列表
- 📝 **端点	            方法    认证	    说明
- ⚙️ **/health	        GET	    ❌	    健康检查
- ⚙️ **/auth/login	    POST	  ❌	    用户登录
- ⚙️ **/auth/register	  POST	  ❌	    用户注册
- 🛡️ **/api/profile	  GET	    ✅	    获取用户信息
- 🛡️ **/api/settings	  PUT	    ✅	    修改用户设置
- 🛡️ **/api/json	      POST	  ✅	    JSON 数据处理

### 安装运行
```bash
git clone https://github.com/JIMGAN1/PostJson-api.git

go mod tidy
go run postjson.go


