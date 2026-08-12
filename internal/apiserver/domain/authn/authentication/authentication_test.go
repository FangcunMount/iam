package authentication

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestProofConstructorsValidateRequiredFieldsAndMapCredentialKind(t *testing.T) {
	t.Parallel()

	password, err := NewPasswordCredential(PasswordProofSpec{TenantID: meta.FromUint64(1), Username: "alice", Password: "secret"})
	require.NoError(t, err)
	require.Equal(t, CredentialKindPassword, password.CredentialKind())
	_, err = NewPasswordCredential(PasswordProofSpec{})
	require.Error(t, err)

	phone, err := NewPhoneOTPCredential(PhoneOTPProofSpec{TenantID: meta.FromUint64(1), PhoneE164: "+8613800138000", OTP: "123456"})
	require.NoError(t, err)
	require.Equal(t, CredentialKindPhoneOTP, phone.CredentialKind())
	_, err = NewPhoneOTPCredential(PhoneOTPProofSpec{})
	require.Error(t, err)

	wechat, err := NewWechatMiniCredential(WechatMiniProofSpec{
		TenantID:  meta.FromUint64(1),
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWechatMinip, wechat.CredentialKind())
	_, err = NewWechatMiniCredential(WechatMiniProofSpec{})
	require.Error(t, err)

	wechatOpen, err := NewWechatOpenCredential(WechatOpenProofSpec{
		AppID:     "wx-app",
		AppSecret: "secret",
		Code:      "code",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWechatOpen, wechatOpen.CredentialKind())
	_, err = NewWechatOpenCredential(WechatOpenProofSpec{})
	require.Error(t, err)

	wecom, err := NewWecomCredential(WecomProofSpec{
		TenantID:   meta.FromUint64(1),
		CorpID:     "corp",
		AgentID:    "agent",
		CorpSecret: "secret",
		Code:       "code",
	})
	require.NoError(t, err)
	require.Equal(t, CredentialKindWecom, wecom.CredentialKind())
	_, err = NewWecomCredential(WecomProofSpec{})
	require.Error(t, err)
}

type authenticatorStrategyStub struct {
	kind        CredentialKind
	called      bool
	hasDecision bool
	decision    AuthDecision
	err         error
}

func (s *authenticatorStrategyStub) Kind() CredentialKind {
	return s.kind
}

func (s *authenticatorStrategyStub) Authenticate(context.Context, AuthCredential) (AuthDecision, error) {
	s.called = true
	if s.err != nil {
		return AuthDecision{}, s.err
	}
	if s.hasDecision {
		return s.decision, nil
	}
	return AuthDecision{
		OK: true,
		Principal: &Principal{
			UserID:          meta.FromUint64(1001),
			LoginIdentityID: meta.FromUint64(2002),
			TenantID:        meta.FromUint64(1),
		},
	}, nil
}

type authenticatorAuditLoggerStub struct {
	events []AuthAuditEvent
}

func (s *authenticatorAuditLoggerStub) LogAuthAttempt(_ context.Context, event AuthAuditEvent) {
	s.events = append(s.events, event)
}

func TestAuthenticatorUsesInjectedStrategyMapping(t *testing.T) {
	t.Parallel()

	strategy := &authenticatorStrategyStub{kind: CredentialKindPassword}
	a := NewAuthenticator(strategy)
	proof, err := NewPasswordCredential(PasswordProofSpec{
		TenantID: meta.FromUint64(1),
		Username: "alice",
		Password: "secret",
	})
	require.NoError(t, err)

	decision, err := a.Authenticate(context.Background(), proof)

	require.NoError(t, err)
	require.True(t, decision.OK)
	require.True(t, strategy.called)
	require.Nil(t, a.strategyFor(CredentialKind("unknown")))
}

func TestAuthenticatorLogsSuccessfulAuthAttempt(t *testing.T) {
	t.Parallel()

	strategy := &authenticatorStrategyStub{kind: CredentialKindPassword}
	auditLogger := &authenticatorAuditLoggerStub{}
	a := NewAuthenticator(strategy).WithAuditLogger(auditLogger)
	proof, err := NewPasswordCredential(PasswordProofSpec{
		TenantID:  meta.FromUint64(1),
		RemoteIP:  "10.0.0.1",
		UserAgent: "iam-test",
		Username:  "alice",
		Password:  "secret",
	})
	require.NoError(t, err)

	decision, err := a.Authenticate(context.Background(), proof)

	require.NoError(t, err)
	require.True(t, decision.OK)
	require.Len(t, auditLogger.events, 1)
	event := auditLogger.events[0]
	require.True(t, event.Success)
	require.Equal(t, 0, event.Code)
	require.Equal(t, CredentialKindPassword, event.CredentialKind)
	require.Equal(t, meta.FromUint64(2002), event.LoginIdentityID)
	require.Equal(t, "10.0.0.1", event.RemoteIP)
	require.Equal(t, "iam-test", event.UserAgent)
	require.False(t, event.Timestamp.IsZero())
}

func TestAuthenticatorLogsFailedAuthAttempt(t *testing.T) {
	t.Parallel()

	strategy := &authenticatorStrategyStub{
		kind:        CredentialKindPassword,
		hasDecision: true,
		decision: AuthDecision{
			OK:              false,
			Code:            code.ErrCredentialLocked,
			LoginIdentityID: meta.FromUint64(2002),
			CredentialID:    meta.FromUint64(3003),
		},
	}
	auditLogger := &authenticatorAuditLoggerStub{}
	a := NewAuthenticator(strategy).WithAuditLogger(auditLogger)
	proof, err := NewPasswordCredential(PasswordProofSpec{
		TenantID:  meta.FromUint64(1),
		RemoteIP:  "10.0.0.2",
		UserAgent: "iam-test",
		Username:  "alice",
		Password:  "secret",
	})
	require.NoError(t, err)

	decision, err := a.Authenticate(context.Background(), proof)

	require.NoError(t, err)
	require.False(t, decision.OK)
	require.Len(t, auditLogger.events, 1)
	event := auditLogger.events[0]
	require.False(t, event.Success)
	require.Equal(t, code.ErrCredentialLocked, event.Code)
	require.Equal(t, CredentialKindPassword, event.CredentialKind)
	require.Equal(t, meta.FromUint64(2002), event.LoginIdentityID)
	require.Equal(t, meta.FromUint64(3003), event.CredentialID)
	require.Equal(t, "10.0.0.2", event.RemoteIP)
	require.Equal(t, "iam-test", event.UserAgent)
	require.False(t, event.Timestamp.IsZero())
}
