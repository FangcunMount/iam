package authentication

import "context"

// AuthCredential 认证凭据接口。
type AuthCredential interface {
	CredentialKind() CredentialKind
}

// AuthStrategy 认证策略（领域服务接口）。
type AuthStrategy interface {
	Kind() CredentialKind
	Authenticate(ctx context.Context, proof AuthCredential) (AuthDecision, error)
}
