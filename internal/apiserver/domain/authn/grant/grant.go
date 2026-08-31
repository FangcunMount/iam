package grant

import (
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
)

// AuthenticationGrant 表示一次认证成功后建立的完整在线认证结果。
// Session 表达服务端认证状态，TokenSet 表达交付给调用方的凭证集合。
type AuthenticationGrant struct {
	Session  *sessiondomain.Session
	TokenSet *tokendomain.UserTokenSet
}

// NewAuthenticationGrant 创建认证结果。
func NewAuthenticationGrant(session *sessiondomain.Session, tokenSet *tokendomain.UserTokenSet) *AuthenticationGrant {
	return &AuthenticationGrant{Session: session, TokenSet: tokenSet}
}
