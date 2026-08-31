package authn

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	tokenjwt "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/jwt"
	authhandler "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/handler"
	resp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 集成测试：与登录一致的签发链（IssueToken → JWT）→ 本地解析 tenant_id →
// gRPC VerifyToken 与 REST POST /verify 返回的 user_id / login_identity_id / tenant_id 一致。

type noopTokenStore struct{}

func (noopTokenStore) SaveRefreshToken(context.Context, *tokendomain.RefreshToken) error { return nil }
func (noopTokenStore) RotateRefreshToken(context.Context, string, string, *tokendomain.RefreshToken) (bool, error) {
	return true, nil
}
func (noopTokenStore) GetRefreshToken(context.Context, string) (*tokendomain.RefreshToken, error) {
	return nil, nil
}
func (noopTokenStore) GetConsumedRefreshToken(context.Context, string) (*tokendomain.ConsumedRefreshToken, error) {
	return nil, nil
}
func (noopTokenStore) DeleteRefreshToken(context.Context, string) error { return nil }
func (noopTokenStore) MarkAccessTokenRevoked(context.Context, string, time.Duration) error {
	return nil
}
func (noopTokenStore) IsAccessTokenRevoked(context.Context, string) (bool, error) { return false, nil }

type memorySessionStore struct {
	sessions map[string]*sessiondomain.Session
}

func (s *memorySessionStore) Save(_ context.Context, session *sessiondomain.Session) error {
	if s.sessions == nil {
		s.sessions = make(map[string]*sessiondomain.Session)
	}
	s.sessions[session.SessionID] = session
	return nil
}

func (s *memorySessionStore) Get(_ context.Context, sessionID string) (*sessiondomain.Session, error) {
	if s.sessions == nil {
		return nil, nil
	}
	return s.sessions[sessionID], nil
}

func (s *memorySessionStore) Revoke(_ context.Context, sessionID string, reason string, revokedBy string) error {
	if s.sessions == nil {
		return nil
	}
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Revoke(reason, revokedBy)
	}
	return nil
}

func (s *memorySessionStore) Extend(_ context.Context, sessionID string, expiresAt time.Time) error {
	if s.sessions == nil {
		return nil
	}
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Extend(expiresAt)
	}
	return nil
}

func (s *memorySessionStore) RevokeByUser(_ context.Context, userID meta.ID, reason string, revokedBy string) error {
	for _, sess := range s.sessions {
		if sess.UserID == userID {
			sess.Revoke(reason, revokedBy)
		}
	}
	return nil
}

func (s *memorySessionStore) RevokeByLoginIdentity(_ context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error {
	for _, sess := range s.sessions {
		if sess.LoginIdentityID == loginIdentityID {
			sess.Revoke(reason, revokedBy)
		}
	}
	return nil
}

type allowAllAdmissionPolicy struct{}

func (allowAllAdmissionPolicy) Evaluate(_ context.Context, subject admissiondomain.Subject) (admissiondomain.Decision, error) {
	return admissiondomain.Admit(subject), nil
}

type fixedSigningKeySource struct {
	kid string
	key *rsa.PrivateKey
}

func (s fixedSigningKeySource) ActiveSigningKey(context.Context) (string, *rsa.PrivateKey, error) {
	return s.kid, s.key, nil
}

func (s fixedSigningKeySource) VerificationKey(context.Context, string) (*rsa.PublicKey, error) {
	return &s.key.PublicKey, nil
}

func newTestTokenStack(t *testing.T) (
	tokenapp.Capabilities,
	*tokenjwt.Generator,
) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kid := "integration-test-kid"
	gen := tokenjwt.NewGenerator("https://iam.integration.test", []string{"qs-api", "collection-api"}, fixedSigningKeySource{kid: kid, key: priv})
	store := noopTokenStore{}
	sessionStore := &memorySessionStore{}
	lifetime := sessiondomain.NewLifetimePolicy(24*time.Hour, 24*time.Hour)
	tokens := tokenapp.NewCapabilities(tokenapp.Dependencies{
		AccessTokenCodec:      gen,
		TokenStore:            store,
		SessionCreator:        sessiondomain.NewCreator(sessionStore, lifetime),
		SessionLoader:         sessiondomain.NewLoader(sessionStore, lifetime),
		SessionRevoker:        sessiondomain.NewRevoker(sessionStore),
		SessionExtender:       sessiondomain.NewExtender(sessionStore, lifetime),
		SessionRefreshExpirer: sessiondomain.NewRefreshExpirer(lifetime),
		AdmissionPolicy:       allowAllAdmissionPolicy{},
		RefreshClaimsCodec:    tokenapp.NewDefaultRefreshClaimsCodec(),
		AccessTTL:             time.Hour,
	})
	return tokens, gen
}

func TestIntegration_LoginIssueToken_VerifyToken_GRPC_REST_TenantConsistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	tokens, gen := newTestTokenStack(t)

	principal := &authentication.Principal{
		UserID:          meta.FromUint64(1001),
		LoginIdentityID: meta.FromUint64(2002),
		AMR:             []string{string(authentication.AMRPassword)},
		Claims: map[string]any{
			"phone_number":  "+8613800138000",
			"tenant_domain": "fangcun",
			"org_id":        "9001",
		},
	}

	// 与登录成功后的签发路径一致：IssueToken → access_token JWT
	pair, err := tokens.AuthenticationGrantIssuer.IssueAuthentication(ctx, principal)
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.NotNil(t, pair.AccessToken)
	access := pair.AccessToken.Value

	// 本地解析（与 apiserver 验签链相同的 Generator）
	parsed, err := gen.VerifyAccessToken(ctx, access)
	require.NoError(t, err)
	require.Equal(t, "fangcun", parsed.TenantDomain)
	require.Equal(t, meta.FromUint64(9001), parsed.OrgID)
	require.Equal(t, "1001", parsed.UserID.String())
	require.Equal(t, "2002", parsed.LoginIdentityID.String())
	require.Equal(t, []string{string(authentication.AMRPassword)}, parsed.AMR)
	require.Equal(t, "+8613800138000", parsed.Attributes["phone_number"])

	// gRPC VerifyToken
	grpcSrv := &authServiceServer{tokenVerifier: tokens.Verifier}
	gresp, err := grpcSrv.VerifyToken(ctx, &authnv2.VerifyTokenRequest{AccessToken: access})
	require.NoError(t, err)
	require.True(t, gresp.Valid)
	require.NotNil(t, gresp.Claims)
	require.Equal(t, "1001", gresp.Claims.UserId)
	require.Equal(t, "2002", gresp.Claims.LoginIdentityId)
	require.Equal(t, "fangcun", gresp.Claims.TenantId)
	require.Equal(t, "9001", gresp.Claims.OrgId)
	require.Equal(t, []string{string(authentication.AMRPassword)}, gresp.Claims.Amr)
	require.Equal(t, "+8613800138000", gresp.Claims.Attributes["phone_number"])

	gresp, err = grpcSrv.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:      access,
		ExpectedIssuer:   "https://iam.integration.test",
		ExpectedAudience: []string{"qs-api"},
	})
	require.NoError(t, err)
	require.True(t, gresp.Valid)

	// REST POST verify（与 gRPC 使用同一 Verifier 能力）
	h := authhandler.NewAuthHandler(nil, tokens, nil)
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"access_token":"` + access + `"}`)
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v2/authn/verify", body)
	c.Request.Header.Set("Content-Type", "application/json")
	h.VerifyToken(c)
	require.Equal(t, http.StatusOK, w.Code)

	var envelope core.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)

	dataBytes, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var tv resp.TokenVerifyResponse
	require.NoError(t, json.Unmarshal(dataBytes, &tv))
	require.True(t, tv.Valid)
	require.NotNil(t, tv.Claims)
	require.Equal(t, "1001", tv.Claims.UserID)
	require.Equal(t, "2002", tv.Claims.LoginIdentityID)
	// REST verify 仍暴露 legacy *int64 tenant_id；新 token 授权域/org 以 gRPC 为准
	require.Nil(t, tv.Claims.TenantID)

	// 与 gRPC 声明对齐（时间字段由同一套 claims 产生）
	require.Equal(t, gresp.Claims.UserId, tv.Claims.UserID)
	require.Equal(t, gresp.Claims.LoginIdentityId, tv.Claims.LoginIdentityID)
	require.Equal(t, gresp.Claims.Amr, tv.Claims.Amr)
	require.Equal(t, "+8613800138000", tv.Claims.Attributes["phone_number"])
}

func TestIntegration_VerifyToken_RejectsIssuerOrAudienceMismatch(t *testing.T) {
	ctx := context.Background()
	tokens, _ := newTestTokenStack(t)

	principal := &authentication.Principal{
		UserID:          meta.FromUint64(7),
		LoginIdentityID: meta.FromUint64(8),
		TenantID:        meta.FromUint64(9),
	}
	pair, err := tokens.AuthenticationGrantIssuer.IssueAuthentication(ctx, principal)
	require.NoError(t, err)

	grpcSrv := &authServiceServer{tokenVerifier: tokens.Verifier}

	respIssuer, err := grpcSrv.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:    pair.AccessToken.Value,
		ExpectedIssuer: "https://issuer.invalid",
	})
	require.NoError(t, err)
	require.False(t, respIssuer.Valid)

	respAudience, err := grpcSrv.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:      pair.AccessToken.Value,
		ExpectedAudience: []string{"wrong-audience"},
	})
	require.NoError(t, err)
	require.False(t, respAudience.Valid)
}

// 可选：gRPC VerifyToken 在 IncludeMetadata 时返回元数据（与 Claims 同源签发链）。
func TestIntegration_VerifyToken_GRPC_IncludeMetadata(t *testing.T) {
	ctx := context.Background()
	tokens, _ := newTestTokenStack(t)

	principal := &authentication.Principal{
		UserID:          meta.FromUint64(42),
		LoginIdentityID: meta.FromUint64(43),
		TenantID:        meta.FromUint64(44),
	}
	pair, err := tokens.AuthenticationGrantIssuer.IssueAuthentication(ctx, principal)
	require.NoError(t, err)

	grpcSrv := &authServiceServer{tokenVerifier: tokens.Verifier}
	gresp, err := grpcSrv.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:     pair.AccessToken.Value,
		IncludeMetadata: true,
	})
	require.NoError(t, err)
	require.True(t, gresp.Valid)
	require.NotNil(t, gresp.Metadata)
}
