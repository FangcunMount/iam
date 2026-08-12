package token

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ========================================================
// ============= TokenApplicationService 实现 =============
// ========================================================

// tokenApplicationService 实现 TokenApplicationService 接口
type tokenApplicationService struct {
	accessTokenIssuer  accessTokenIssuerPort  // 用户 access/refresh 令牌签发（登录 + refresh mint）
	serviceTokenIssuer serviceTokenIssuerPort // 服务令牌签发器
	refresher          refresherPort          // 令牌刷新器
	verifier           verifierPort           // 令牌验证器
	revoker            revokerPort            // 令牌撤销器
}

// TokenApplicationDependencies 是 TokenApplicationService 的装配依赖。
type TokenApplicationDependencies struct {
	AccessTokenCodec      AccessTokenCodec       // 令牌编码器
	TokenStore            Store                  // 令牌存储
	SessionCreator        SessionCreator         // 会话创建器
	SessionLoader         SessionLoader          // 会话加载器
	SessionRevoker        SessionRevoker         // 会话撤销器
	SessionExtender       SessionExtender        // 会话延期器
	SessionRefreshExpirer SessionRefreshExpirer  // refresh token 过期时间计算器
	AccessChecker         SubjectAccessEvaluator // 主体访问状态评估器
	RefreshClaimsCodec    RefreshClaimsCodec     // refresh/session claims 快照编解码
	AccessTTL             time.Duration          // 令牌有效期
}

// 确保 tokenApplicationService 实现 TokenApplicationService 接口
var _ TokenApplicationService = (*tokenApplicationService)(nil)

// NewTokenApplicationService 创建 TokenApplicationService。
func NewTokenApplicationService(deps TokenApplicationDependencies) TokenApplicationService {
	issuerComponents := newIssuer(
		deps.AccessTokenCodec,
		deps.TokenStore,
		deps.SessionCreator,
		deps.SessionRefreshExpirer,
		deps.RefreshClaimsCodec,
		deps.AccessTTL,
	)
	tokenRefresher := newRefresher(
		issuerComponents.accessTokenIssuer,
		deps.TokenStore,
		deps.SessionLoader,
		deps.SessionRevoker,
		deps.SessionExtender,
		deps.AccessChecker,
		deps.RefreshClaimsCodec,
	)
	tokenVerifier := newVerifier(
		deps.AccessTokenCodec,
		deps.TokenStore,
		deps.SessionLoader,
		deps.AccessChecker,
	)
	revoker := newRevoker(
		deps.AccessTokenCodec,
		deps.TokenStore,
		deps.SessionRevoker,
	)
	return &tokenApplicationService{
		accessTokenIssuer:  issuerComponents.accessTokenIssuer,
		serviceTokenIssuer: issuerComponents.serviceTokenIssuer,
		refresher:          tokenRefresher,
		verifier:           tokenVerifier,
		revoker:            revoker,
	}
}

// IssueToken 在登录完成后签发用户会话令牌。
func (s *tokenApplicationService) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	return s.accessTokenIssuer.IssueToken(ctx, principal)
}

// IssueServiceToken 签发服务间访问令牌。
// 服务令牌不绑定 session，也不生成 refresh token。
func (s *tokenApplicationService) IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error) {
	tokenPair, err := s.serviceTokenIssuer.IssueServiceToken(ctx, req.Subject, req.Audience, req.Attributes, req.TTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to issue service token")
	}

	return &TokenIssueResult{TokenPair: tokenPair}, nil
}

// RefreshToken 使用 refresh token 轮换出新的 access/refresh token pair。
// 具体流程由内部 refresher 完成，包括 refresh token 读取、旧 token 删除和 session 延期。
func (s *tokenApplicationService) RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	tokenPair, err := s.refresher.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	return &TokenRefreshResult{
		TokenPair: tokenPair,
	}, nil
}

// RevokeAccessToken 撤销单个 access token 及其关联会话。
func (s *tokenApplicationService) RevokeAccessToken(ctx context.Context, accessToken string) error {
	err := s.revoker.RevokeBearerToken(ctx, accessToken)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenRevokeFailed, "failed to revoke access token")
	}

	return nil
}

// RevokeRefreshToken 删除 refresh token；如果 refresh token 关联 session，则同步撤销 session。
func (s *tokenApplicationService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	err := s.refresher.RevokeRefreshToken(ctx, refreshToken)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenRevokeFailed, "failed to revoke refresh token")
	}
	return nil
}

// VerifyToken 在线验证 access token，并检查可选的 issuer/audience 约束。
func (s *tokenApplicationService) VerifyToken(ctx context.Context, req VerifyTokenRequest) (*TokenVerifyResult, error) {
	claims, err := s.verifier.VerifyToken(ctx, req.AccessToken)
	if err != nil {
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
