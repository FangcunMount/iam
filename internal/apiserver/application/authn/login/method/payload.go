package method

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// CommonPayload 是所有登录方式共享的请求上下文。
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
