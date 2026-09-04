package token

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissionapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	grantdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/grant"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// application 组合令牌用例协作者，并分别实现对外窄能力。
type application struct {
	grantIssuer        grantdomain.Issuer
	serviceTokenIssuer tokendomain.ServiceTokenIssuer
	refresher          tokendomain.Refresher
	verifier           tokendomain.Verifier
	revoker            tokendomain.Revoker
}

// Dependencies 是令牌用例能力的装配依赖。
type Dependencies struct {
	AccessTokenCodec      AccessTokenCodec      // 令牌编码器
	TokenStore            Store                 // 令牌存储
	SessionCreator        SessionCreator        // 会话创建器
	SessionLoader         SessionLoader         // 会话加载器
	SessionRevoker        SessionRevoker        // 会话撤销器
	SessionExtender       SessionExtender       // 会话延期器
	SessionRefreshExpirer SessionRefreshExpirer // refresh token 过期时间计算器
	AdmissionPolicy       AdmissionPolicy       // 认证准入策略
	RefreshClaimsCodec    RefreshClaimsCodec    // refresh/session claims 快照编解码
	AccessTTL             time.Duration         // 令牌有效期
}

var (
	_ AuthenticationGrantIssuer = (*application)(nil)
	_ ServiceTokenIssuer        = (*application)(nil)
	_ Refresher                 = (*application)(nil)
	_ Revoker                   = (*application)(nil)
	_ Verifier                  = (*application)(nil)
)

// NewCapabilities 装配并返回相互独立的令牌用例能力。
func NewCapabilities(deps Dependencies) Capabilities {
	domainCapabilities := tokendomain.NewCapabilities(tokendomain.Dependencies{
		AccessTokenCodec: deps.AccessTokenCodec, TokenStore: deps.TokenStore,
		SessionLoader:  deps.SessionLoader,
		SessionRevoker: deps.SessionRevoker, SessionExtender: deps.SessionExtender,
		SessionRefreshExpirer: deps.SessionRefreshExpirer, AdmissionPolicy: deps.AdmissionPolicy,
		RefreshClaimsCodec: deps.RefreshClaimsCodec, AccessTTL: deps.AccessTTL,
	})
	grantIssuer := grantdomain.NewIssuer(grantdomain.Dependencies{
		AdmissionPolicy: deps.AdmissionPolicy, SessionCreator: deps.SessionCreator,
		TokenSetMinter: domainCapabilities.TokenSetMinter, RefreshTokenSaver: deps.TokenStore,
	})
	app := &application{
		grantIssuer:        grantIssuer,
		serviceTokenIssuer: domainCapabilities.ServiceTokenIssuer,
		refresher:          domainCapabilities.Refresher,
		verifier:           domainCapabilities.Verifier,
		revoker:            domainCapabilities.Revoker,
	}
	return Capabilities{
		AuthenticationGrantIssuer: app,
		ServiceTokenIssuer:        app,
		Refresher:                 app,
		Revoker:                   app,
		Verifier:                  app,
	}
}

// IssueAuthentication 在认证完成后颁发 Session + TokenSet，并返回应用层 token pair。
func (s *application) IssueAuthentication(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	grant, err := s.grantIssuer.Issue(ctx, principal)
	if err != nil {
		return nil, admissionapp.MapError(err)
	}
	return tokenPairFromDomain(grant.TokenSet), nil
}

// IssueServiceToken 签发服务间访问令牌。
// 服务令牌不绑定 session，也不生成 refresh token。
func (s *application) IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error) {
	serviceToken, err := s.serviceTokenIssuer.IssueServiceToken(ctx, req.Subject, req.Audience, req.Attributes, req.TTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to issue service token")
	}

	return &TokenIssueResult{TokenPair: NewTokenPair(tokenFromService(serviceToken), nil)}, nil
}

// RefreshToken 使用 refresh token 轮换出新的 access/refresh token pair。
// 具体流程由内部 refresher 完成，包括 refresh token 读取、旧 token 删除和 session 延期。
func (s *application) RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	tokenSet, err := s.refresher.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, admissionapp.MapError(err)
	}

	return &TokenRefreshResult{
		TokenPair: tokenPairFromDomain(tokenSet),
	}, nil
}

// RevokeAccessToken 撤销单个 access token 及其关联会话。
func (s *application) RevokeAccessToken(ctx context.Context, accessToken string) error {
	err := s.revoker.RevokeBearerToken(ctx, accessToken)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenRevokeFailed, "failed to revoke access token")
	}

	return nil
}

// RevokeRefreshToken 删除 refresh token；如果 refresh token 关联 session，则同步撤销 session。
func (s *application) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	err := s.refresher.RevokeRefreshToken(ctx, refreshToken)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenRevokeFailed, "failed to revoke refresh token")
	}
	return nil
}

// VerifyToken 在线验证令牌，并检查场景级 issuer/audience/token type 约束。
// 密码学与 canonical issuer 由 codec 负责；撤销、Session、Admission 由 domain verifier 负责。
func (s *application) VerifyToken(ctx context.Context, req VerifyTokenRequest) (*TokenVerifyResult, error) {
	claims, err := s.verifier.VerifyToken(ctx, req.AccessToken)
	if err != nil {
		err = admissionapp.MapError(err)
		failureCode := tokenVerificationFailureCode(err)
		if failureCode == 0 {
			return nil, err
		}
		return &TokenVerifyResult{
			Valid:       false,
			Claims:      nil,
			FailureCode: failureCode,
		}, nil
	}

	if expectedIssuer := strings.TrimSpace(req.ExpectedIssuer); expectedIssuer != "" && claims.Issuer != expectedIssuer {
		return &TokenVerifyResult{Valid: false, Claims: nil}, nil
	}

	if len(req.ExpectedAudience) > 0 && !containsAnyAudience(claims.Audience, req.ExpectedAudience) {
		return &TokenVerifyResult{Valid: false, Claims: nil}, nil
	}

	acceptedTokenTypes := req.AcceptedTokenTypes
	if len(acceptedTokenTypes) == 0 {
		acceptedTokenTypes = []TokenType{TokenTypeAccess}
	}
	if !containsTokenType(acceptedTokenTypes, claims.TokenType) {
		return &TokenVerifyResult{Valid: false, Claims: nil}, nil
	}

	return &TokenVerifyResult{
		Valid:  true,
		Claims: claims,
	}, nil
}

// containsAnyAudience 检查实际受众是否包含预期受众
func containsAnyAudience(actual []string, expected []string) bool {
	if len(actual) == 0 || len(expected) == 0 {
		return false
	}

	actualSet := make(map[string]struct{}, len(actual))
	for _, aud := range actual {
		actualSet[aud] = struct{}{}
	}

	for _, aud := range expected {
		if _, ok := actualSet[aud]; ok {
			return true
		}
	}

	return false
}

func containsTokenType(accepted []TokenType, actual TokenType) bool {
	for _, tokenType := range accepted {
		if tokenType == actual {
			return true
		}
	}
	return false
}

func tokenVerificationFailureCode(err error) int {
	codeValue := perrors.ParseCoder(err).Code()
	switch codeValue {
	case code.ErrTokenInvalid,
		code.ErrExpired,
		code.ErrUserBlocked,
		code.ErrUserInactive,
		code.ErrLoginIdentityDisabled,
		code.ErrCredentialLocked,
		code.ErrSessionInactive:
		return codeValue
	default:
		return 0
	}
}
