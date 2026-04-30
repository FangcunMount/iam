package token

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/logger"

	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

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
