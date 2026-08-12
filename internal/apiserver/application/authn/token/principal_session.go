package token

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// validatePrincipalSessionAlignment 校验认证主体与既有 session 的身份上下文一致。
func validatePrincipalSessionAlignment(principal *authentication.Principal, sess *sessiondomain.Session) error {
	if principal == nil || sess == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "principal and session are required")
	}
	if principal.UserID != sess.UserID {
		return perrors.WithCode(code.ErrInvalidArgument, "principal user does not match session")
	}
	if principal.LoginIdentityID != sess.LoginIdentityID {
		return perrors.WithCode(code.ErrInvalidArgument, "principal login identity does not match session")
	}
	if sessionID := strings.TrimSpace(principal.SessionID); sessionID != "" && sessionID != sess.SessionID {
		return perrors.WithCode(code.ErrInvalidArgument, "principal session id does not match session")
	}
	if !principal.TenantID.IsZero() && !sess.TenantID.IsZero() && principal.TenantID != sess.TenantID {
		return perrors.WithCode(code.ErrInvalidArgument, "principal tenant does not match session")
	}
	if principal.AuthMethod != "" && sess.AuthMethod != "" && principal.AuthMethod != sess.AuthMethod {
		return perrors.WithCode(code.ErrInvalidArgument, "principal auth method does not match session")
	}
	if principal.Realm != "" && sess.Realm != "" && principal.Realm != sess.Realm {
		return perrors.WithCode(code.ErrInvalidArgument, "principal realm does not match session")
	}
	return nil
}

// resolveMintTenantID 在 mint 阶段统一租户 ID：优先 session，其次 principal。
func resolveMintTenantID(principal *authentication.Principal, sess *sessiondomain.Session) meta.ID {
	if sess != nil && !sess.TenantID.IsZero() {
		return sess.TenantID
	}
	if principal != nil && !principal.TenantID.IsZero() {
		return principal.TenantID
	}
	return meta.ZeroID
}

// accessTokenSubjectFromAuth 将领域主体与会话合并为 access token 编码快照。
func accessTokenSubjectFromAuth(principal *authentication.Principal, sess *sessiondomain.Session) *AccessTokenSubject {
	tenantID := resolveMintTenantID(principal, sess)
	return &AccessTokenSubject{
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
		SessionID:       sess.SessionID,
		TenantID:        tenantID,
		AuthMethod:      coalesceNonEmpty(principal.AuthMethod, sess.AuthMethod),
		Realm:           coalesceNonEmpty(principal.Realm, sess.Realm),
		AMR:             append([]string(nil), principal.AMR...),
		Claims:          cloneAnyMap(principal.Claims),
	}
}

func coalesceNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
