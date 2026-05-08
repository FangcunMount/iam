package reauth

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// ReAuthenticator 负责已有 access token 的再验证，不是登录行为。
type ReAuthenticator interface {
	Reauthenticate(ctx context.Context, tokenValue string) (authentication.AuthDecision, error)
}
