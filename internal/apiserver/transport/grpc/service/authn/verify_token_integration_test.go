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
	redisinfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	tokenjwt "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/jwt"
	authhandler "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/handler"
	resp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/response"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/core"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
func (noopTokenStore) MarkBearerTokenRevoked(context.Context, string, time.Duration) error {
	return nil
}
func (noopTokenStore) IsBearerTokenRevoked(context.Context, string) (bool, error) { return false, nil }

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

type fixedJWSKeySource struct {
	kid string
	key *rsa.PrivateKey
}

func (s fixedJWSKeySource) ActiveSigningKey(context.Context) (*tokenjwt.SigningKey, error) {
	return &tokenjwt.SigningKey{Kid: s.kid, Algorithm: "RS256", PrivateKey: s.key}, nil
}

func (s fixedJWSKeySource) VerificationKey(context.Context, string) (*tokenjwt.VerificationKey, error) {
	return &tokenjwt.VerificationKey{Kid: s.kid, Algorithm: "RS256", PublicKey: &s.key.PublicKey}, nil
}

func newTestTokenStack(t *testing.T) (
	tokenapp.Capabilities,
	*tokenjwt.JWSCompactTokenCodec,
) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kid := "integration-test-kid"
	gen := tokenjwt.NewJWSCompactTokenCodec("https://iam.integration.test", []string{"qs-api", "collection-api"}, fixedJWSKeySource{kid: kid, key: priv})
	store := noopTokenStore{}
	sessionStore := &memorySessionStore{}
	lifetime := sessiondomain.NewLifetimePolicy(24*time.Hour, 24*time.Hour)
	tokens := tokenapp.NewCapabilities(tokenapp.Dependencies{
		BearerTokenCodec:      gen,
		TokenStore:            store,
		SessionCreator:        sessiondomain.NewCreator(sessionStore, lifetime),
		SessionLoader:         sessiondomain.NewLoader(sessionStore, lifetime),
		SessionRevoker:        sessiondomain.NewRevoker(sessionStore),
		SessionExtender:       sessiondomain.NewExtender(sessionStore, lifetime),
		SessionRefreshExpirer: sessiondomain.NewRefreshExpirer(lifetime),
		AdmissionPolicy:       allowAllAdmissionPolicy{},
		LegacyContextDecoder:  tokenapp.NewLegacyAuthenticationContextSnapshotDecoder(),
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
		AuthContext:     authentication.NewAuthenticationContext(authentication.MethodPassword, "global", []authentication.AMR{authentication.AMRPassword}, time.Now().UTC()),
		TokenContext: authentication.TokenContext{
			TenantDomain: "fangcun",
			OrgID:        meta.FromUint64(9001),
		},
	}

	// 与登录成功后的签发路径一致：IssueToken → access_token JWT
	pair, err := tokens.AuthenticationGrantIssuer.IssueAuthentication(ctx, principal)
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.NotNil(t, pair.AccessToken)
	access := pair.AccessToken.Value

	// 本地解析（与 apiserver 验签链相同的 JWSCompactTokenCodec）
	parsed, err := gen.VerifyBearerToken(ctx, access)
	require.NoError(t, err)
	require.Equal(t, "fangcun", parsed.TenantDomain)
	require.Equal(t, meta.FromUint64(9001), parsed.OrgID)
	require.Equal(t, "1001", parsed.UserID.String())
	require.Equal(t, "2002", parsed.LoginIdentityID.String())
	require.Equal(t, []string{string(authentication.AMRPassword)}, parsed.AMR)
	require.NotContains(t, parsed.Attributes, "phone_number")
	require.NotZero(t, parsed.AuthenticatedAt)
	require.Contains(t, parsed.Attributes, "auth_time")

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
	require.NotContains(t, gresp.Claims.Attributes, "phone_number")
	require.Contains(t, gresp.Claims.Attributes, "auth_time")

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
	require.Nil(t, tv.Claims.TenantID)
	require.Equal(t, "fangcun", tv.Claims.TenantDomain)
	require.Equal(t, "9001", tv.Claims.OrgID)

	// 与 gRPC 声明对齐（时间字段由同一套 claims 产生）
	require.Equal(t, gresp.Claims.UserId, tv.Claims.UserID)
	require.Equal(t, gresp.Claims.LoginIdentityId, tv.Claims.LoginIdentityID)
	require.Equal(t, gresp.Claims.TenantDomain, tv.Claims.TenantDomain)
	require.Equal(t, gresp.Claims.OrgId, tv.Claims.OrgID)
	require.Equal(t, gresp.Claims.Amr, tv.Claims.Amr)
	require.Equal(t, "access", tv.Claims.TokenType)
	require.Equal(t, authnv2.TokenType_TOKEN_TYPE_ACCESS, gresp.Claims.TokenType)
	require.NotZero(t, tv.Claims.NotBefore)
	require.NotZero(t, tv.Claims.AuthenticatedAt)
	require.NotContains(t, tv.Claims.Attributes, "phone_number")
	require.Contains(t, tv.Claims.Attributes, "auth_time")
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

func TestIntegration_ServiceTokenIssueVerifyRevokeRoundTrip(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	codec := tokenjwt.NewJWSCompactTokenCodec(
		"https://iam.integration.test",
		[]string{"qs-api"},
		fixedJWSKeySource{kid: "service-integration-kid", key: privateKey},
	)
	tokens := tokenapp.NewCapabilities(tokenapp.Dependencies{
		BearerTokenCodec: codec,
		TokenStore:       redisinfra.NewRedisStore(client),
		AccessTTL:        time.Hour,
	})
	server := &authServiceServer{
		serviceTokenIssuer: tokens.ServiceTokenIssuer,
		tokenVerifier:      tokens.Verifier,
		tokenRevoker:       tokens.Revoker,
	}

	_, err = server.IssueServiceToken(ctx, &authnv2.IssueServiceTokenRequest{Subject: "service:worker"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	issued, err := server.IssueServiceToken(ctx, &authnv2.IssueServiceTokenRequest{
		Subject:  "service:worker",
		Audience: []string{"qs-api"},
	})
	require.NoError(t, err)
	require.NotNil(t, issued.TokenPair)
	require.NotEmpty(t, issued.TokenPair.AccessToken)

	verified, err := server.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:        issued.TokenPair.AccessToken,
		ExpectedAudience:   []string{"qs-api"},
		AcceptedTokenTypes: []authnv2.TokenType{authnv2.TokenType_TOKEN_TYPE_SERVICE},
	})
	require.NoError(t, err)
	require.True(t, verified.Valid)
	require.Equal(t, authnv2.TokenType_TOKEN_TYPE_SERVICE, verified.Claims.TokenType)
	require.Equal(t, "service:worker", verified.Claims.Subject)

	_, err = server.RevokeToken(ctx, &authnv2.RevokeTokenRequest{AccessToken: issued.TokenPair.AccessToken})
	require.NoError(t, err)

	verified, err = server.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:        issued.TokenPair.AccessToken,
		ExpectedAudience:   []string{"qs-api"},
		AcceptedTokenTypes: []authnv2.TokenType{authnv2.TokenType_TOKEN_TYPE_SERVICE},
	})
	require.NoError(t, err)
	require.False(t, verified.Valid)
	require.Nil(t, verified.Claims)
}
