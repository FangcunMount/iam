package signin

import (
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Result 是登录成功后的应用层结果。
type Result struct {
	Principal       *authentication.Principal
	TokenPair       *tokenapp.TokenPair
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
}

// ResultFromPrincipal 由 Principal 与 TokenPair 构造登录结果。
// 参数：principal 认证主体, tokenPair 令牌对
// 返回：登录结果
// 职责：由认证主体与令牌对构造登录结果
func ResultFromPrincipal(principal *authentication.Principal, tokenPair *tokenapp.TokenPair) *Result {
	// 如果认证主体为空，返回仅包含令牌对的登录结果
	if principal == nil {
		return &Result{TokenPair: tokenPair}
	}
	return &Result{
		Principal:       principal,
		TokenPair:       tokenPair,
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
		TenantID:        principal.TenantID,
	}
}
