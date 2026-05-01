// @title           IAM API Documentation
// @version         1.0.0
// @description     IAM 系统 REST API 文档，包含认证(Authentication)、授权(Authorization)、身份管理(Identity)和身份提供商(IDP)模块
// @termsOfService  https://iam.fangcunmount.cn/terms

// @contact.name   API Support
// @contact.url    https://github.com/FangcunMount/iam/v2
// @contact.email  support@fangcunmount.cn

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      iam.fangcunmount.cn
// @BasePath  /api/v2

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT 认证令牌，格式: Bearer {access_token}

// @tag.name 认证
// @tag.description 认证 - 用户登录
// @tag.name 账户管理
// @tag.description 账号管理 - 创建、查询、绑定第三方账号
// @tag.name Authentication-JWKS
// @tag.description 密钥管理 - JWT 签名验证公钥集

// @tag.name Identity-Users
// @tag.description 用户管理 - 创建、查询、更新用户信息
// @tag.name Identity-Profiles
// @tag.description 儿童管理 - 注册、查询、更新儿童档案
// @tag.name Identity-Ref
// @tag.description 监护关系 - 授予、撤销、查询监护权

// @tag.name Authorization-Roles
// @tag.description 角色管理 - 创建、查询、更新、删除角色
// @tag.name Authorization-Assignments
// @tag.description 角色分配 - 授予、撤销用户或组的角色
// @tag.name Authorization-Policies
// @tag.description 策略管理 - 添加、移除 RBAC 策略规则
// @tag.name Authorization-Resources
// @tag.description 资源管理 - 创建、查询、更新受保护资源

// @tag.name IDP-Wechat
// @tag.description 微信集成 - 登录、应用管理、密钥轮换、令牌获取
// @tag.name Health
// @tag.description 健康检查 - 各模块健康状态

package main

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver"
	_ "github.com/FangcunMount/iam/v2/internal/apiserver/docs"
)

func main() {
	apiserver.NewApp("iam-apiserver").Run()
}
