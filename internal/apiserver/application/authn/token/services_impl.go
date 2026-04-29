package token

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

// ============= TokenApplicationService 实现 =============

type tokenApplicationService struct {
	tokenIssuer    Issuer
	tokenRefresher Refresher
	tokenVerifier  Verifier
}

var _ TokenApplicationService = (*tokenApplicationService)(nil)

func NewTokenApplicationService(
	tokenIssuer Issuer,
	tokenRefresher Refresher,
	tokenVerifier Verifier,
) TokenApplicationService {
	return &tokenApplicationService{
		tokenIssuer:    tokenIssuer,
		tokenRefresher: tokenRefresher,
		tokenVerifier:  tokenVerifier,
	}
}

// IssueServiceToken 签发服务间访问令牌。
func (s *tokenApplicationService) IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error) {
	l := logger.L(ctx)

	l.Debugw("开始签发服务令牌",
		"action", logger.ActionCreate,
		"resource", logger.ResourceToken,
		"token_type", "service",
		"subject", req.Subject,
		"audience", req.Audience,
	)

	tokenPair, err := s.tokenIssuer.IssueServiceToken(ctx, req.Subject, req.Audience, req.Attributes, req.TTL)
	if err != nil {
		l.Warnw("签发服务令牌失败",
			"action", logger.ActionCreate,
			"resource", logger.ResourceToken,
			"token_type", "service",
			"subject", req.Subject,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	l.Debugw("服务令牌签发成功",
		"action", logger.ActionCreate,
		"resource", logger.ResourceToken,
		"token_type", "service",
		"subject", req.Subject,
		"result", logger.ResultSuccess,
	)

	return &TokenIssueResult{TokenPair: tokenPair}, nil
}

// RefreshToken 刷新访问令牌
func (s *tokenApplicationService) RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	l := logger.L(ctx)

	l.Debugw("开始刷新访问令牌",
		"action", logger.ActionRefresh,
		"resource", logger.ResourceToken,
		"token_hint", sanitize.MaskToken(refreshToken),
	)

	// 使用刷新令牌获取新的令牌对
	tokenPair, err := s.tokenRefresher.RefreshToken(ctx, refreshToken)
	if err != nil {
		l.Warnw("刷新令牌失败",
			"action", logger.ActionRefresh,
			"resource", logger.ResourceToken,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, perrors.WithCode(code.ErrTokenInvalid, "failed to refresh token: %v", err)
	}

	l.Debugw("访问令牌刷新成功",
		"action", logger.ActionRefresh,
		"resource", logger.ResourceToken,
		"result", logger.ResultSuccess,
	)

	return &TokenRefreshResult{
		TokenPair: tokenPair,
	}, nil
}

// RevokeAccessToken 撤销访问令牌
func (s *tokenApplicationService) RevokeAccessToken(ctx context.Context, accessToken string) error {
	l := logger.L(ctx)

	l.Debugw("开始撤销访问令牌",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_hint", sanitize.MaskToken(accessToken),
	)

	err := s.tokenIssuer.RevokeAccessToken(ctx, accessToken)
	if err != nil {
		l.Errorw("撤销访问令牌失败",
			"action", logger.ActionRevoke,
			"resource", logger.ResourceToken,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke token: %v", err)
	}

	l.Debugw("访问令牌撤销成功",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"result", logger.ResultSuccess,
	)
	return nil
}

// RevokeRefreshToken 撤销刷新令牌
func (s *tokenApplicationService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	l := logger.L(ctx)

	l.Debugw("开始撤销刷新令牌",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_type", "refresh",
		"token_hint", sanitize.MaskToken(refreshToken),
	)

	err := s.tokenRefresher.RevokeRefreshToken(ctx, refreshToken)
	if err != nil {
		l.Errorw("撤销刷新令牌失败",
			"action", logger.ActionRevoke,
			"resource", logger.ResourceToken,
			"token_type", "refresh",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke refresh token: %v", err)
	}

	l.Debugw("刷新令牌撤销成功",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_type", "refresh",
		"result", logger.ResultSuccess,
	)
	return nil
}

// VerifyToken 验证访问令牌
func (s *tokenApplicationService) VerifyToken(ctx context.Context, req VerifyTokenRequest) (*TokenVerifyResult, error) {
	l := logger.L(ctx)

	l.Debugw("开始验证访问令牌",
		"action", logger.ActionVerify,
		"resource", logger.ResourceToken,
		"token_hint", sanitize.MaskToken(req.AccessToken),
		"expected_issuer", req.ExpectedIssuer,
		"expected_audience", req.ExpectedAudience,
	)

	// 验证访问令牌
	claims, err := s.tokenVerifier.VerifyAccessToken(ctx, req.AccessToken)
	if err != nil {
		l.Warnw("访问令牌验证失败",
			"action", logger.ActionVerify,
			"resource", logger.ResourceToken,
			"error", err.Error(),
			"token_hint", sanitize.MaskToken(req.AccessToken),
			"result", logger.ResultFailed,
		)
		// 令牌无效
		return &TokenVerifyResult{
			Valid:  false,
			Claims: nil,
		}, nil
	}

	if expectedIssuer := strings.TrimSpace(req.ExpectedIssuer); expectedIssuer != "" && claims.Issuer != expectedIssuer {
		l.Warnw("访问令牌 issuer 不匹配",
			"action", logger.ActionVerify,
			"resource", logger.ResourceToken,
			"expected_issuer", expectedIssuer,
			"actual_issuer", claims.Issuer,
			"result", logger.ResultFailed,
		)
		return &TokenVerifyResult{Valid: false, Claims: nil}, nil
	}

	if len(req.ExpectedAudience) > 0 && !containsAnyAudience(claims.Audience, req.ExpectedAudience) {
		l.Warnw("访问令牌 audience 不匹配",
			"action", logger.ActionVerify,
			"resource", logger.ResourceToken,
			"expected_audience", req.ExpectedAudience,
			"actual_audience", claims.Audience,
			"result", logger.ResultFailed,
		)
		return &TokenVerifyResult{Valid: false, Claims: nil}, nil
	}

	l.Debugw("访问令牌验证成功",
		"action", logger.ActionVerify,
		"resource", logger.ResourceToken,
		"user_id", claims.UserID.String(),
		"result", logger.ResultSuccess,
	)
	l.Debugw("访问令牌验证成功", "claims", claims)

	return &TokenVerifyResult{
		Valid:  true,
		Claims: claims,
	}, nil
}

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
