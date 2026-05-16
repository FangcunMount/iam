// Package tenant 定义 IAM 授权域 / Casbin domain（多租户 SaaS 隔离）的 string 约定。
// 业务组织 ID 不属于 IAM 核心模型，由业务系统定义并经 token org_id claim 透传。
package tenant

// DefaultID 未在请求上下文显式指定授权域时使用的 tenant domain（与 Casbin domain、authz.tenant_id 对齐）。
const DefaultID = "fangcun"

// PlatformID 是平台控制面的固定授权域。
const PlatformID = "platform"
