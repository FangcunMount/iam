package authentication

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// logAuthAttempt 记录审计日志
func (a *Authenticator) logAuthAttempt(ctx context.Context, credential AuthCredential, decision AuthDecision) {
	if a == nil || a.auditLogger == nil || credential == nil {
		return
	}

	a.auditLogger.LogAuthAttempt(ctx, authAuditEvent(credential, decision))
}

// authAuditEvent 构建审计事件
func authAuditEvent(credential AuthCredential, decision AuthDecision) AuthAuditEvent {
	remoteIP, userAgent := authAuditClientInfo(credential)
	return AuthAuditEvent{
		LoginIdentityID: authAuditLoginIdentityID(decision),
		CredentialID:    decision.CredentialID,
		CredentialType:  credential.CredentialType(),
		Success:         decision.OK,
		Code:            authAuditCode(decision),
		RemoteIP:        remoteIP,
		UserAgent:       userAgent,
		Timestamp:       time.Now(),
	}
}

// authAuditLoginIdentityID 构建审计登录身份ID
func authAuditLoginIdentityID(decision AuthDecision) meta.ID {
	if !decision.LoginIdentityID.IsZero() {
		return decision.LoginIdentityID
	}
	if decision.Principal != nil {
		return decision.Principal.LoginIdentityID
	}
	return meta.ZeroID
}

// authAuditCode 构建审计代码
func authAuditCode(decision AuthDecision) int {
	if decision.OK {
		return 0
	}
	return decision.Code
}

// authAuditClientInfo 构建审计客户端信息
func authAuditClientInfo(credential AuthCredential) (remoteIP, userAgent string) {
	switch c := credential.(type) {
	case *PasswordCredential:
		return c.RemoteIP, c.UserAgent
	case *PhoneOTPCredential:
		return c.RemoteIP, c.UserAgent
	case *WechatMinipCredential:
		return c.RemoteIP, c.UserAgent
	case *WecomCredential:
		return c.RemoteIP, c.UserAgent
	default:
		return "", ""
	}
}
