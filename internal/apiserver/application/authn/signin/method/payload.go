package method

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// CommonPayload 是所有登录方式共享的请求上下文。
//
// 它只能从 LoginRequest 顶层字段提取。具体 method Payload 不应包含
// TenantID、RemoteIP、UserAgent，避免同一上下文出现两套来源。
type CommonPayload struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
}

// Payload 是登录方式解析后的方法专属 payload。
type Payload interface {
	loginMethodPayload()
}

// CommonPayloadFromLoginRequest 从登录请求中提取公共上下文。
func CommonPayloadFromLoginRequest(cmd LoginRequest) CommonPayload {
	return CommonPayload{
		TenantID:  cmd.TenantID,
		RemoteIP:  cmd.RemoteIP,
		UserAgent: cmd.UserAgent,
	}
}
