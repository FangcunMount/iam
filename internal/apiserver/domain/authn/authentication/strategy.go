package authentication

import "context"

// AuthCredential 认证凭据接口。
type AuthCredential interface {
	// CredentialKind 返回认证凭据类型
	CredentialKind() CredentialKind
}

// AuthStrategy 认证策略（领域服务接口）。
type AuthStrategy interface {
	// Kind 返回认证策略类型
	Kind() CredentialKind
	// Authenticate 执行认证
	Authenticate(ctx context.Context, proof AuthCredential) (AuthDecision, error)
}
