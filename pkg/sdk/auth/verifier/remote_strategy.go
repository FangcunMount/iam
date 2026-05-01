package verifier

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/logger"
	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
)

// RemoteVerifyStrategy 远程验证策略（调用 IAM 服务）。
type RemoteVerifyStrategy struct {
	authClient VerifyTokenClient
	config     *config.TokenVerifyConfig
}

// NewRemoteVerifyStrategy 创建远程验证策略。
func NewRemoteVerifyStrategy(authClient VerifyTokenClient, cfg *config.TokenVerifyConfig) *RemoteVerifyStrategy {
	return &RemoteVerifyStrategy{
		authClient: authClient,
		config:     cfg,
	}
}

func (s *RemoteVerifyStrategy) Name() string {
	return "remote"
}

func (s *RemoteVerifyStrategy) Verify(ctx context.Context, tokenString string, opts *VerifyOptions) (*VerifyResult, error) {
	logger.L(ctx).Debugw("RemoteVerifyStrategy verify start", "strategy", s.Name(), "has_auth_client", s.authClient != nil)
	if s.authClient == nil {
		logger.L(ctx).Errorw("RemoteVerifyStrategy auth client not configured", "strategy", s.Name())
		return nil, fmt.Errorf("remote-strategy: auth client not configured")
	}
	if opts == nil {
		opts = &VerifyOptions{}
	}

	resp, err := s.authClient.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:      tokenString,
		ForceRemote:      opts.ForceRemote,
		IncludeMetadata:  opts.IncludeMetadata,
		ExpectedIssuer:   s.expectedIssuer(opts),
		ExpectedAudience: s.expectedAudience(opts),
	})
	if err != nil {
		logger.L(ctx).Warnw("RemoteVerifyStrategy verify failed", "strategy", s.Name(), "error", err.Error())
		return nil, err
	}
	if !resp.Valid {
		logger.L(ctx).Warnw("RemoteVerifyStrategy token invalid", "strategy", s.Name())
		return nil, fmt.Errorf("remote-strategy: verify token invalid")
	}

	claims := &TokenClaims{
		TokenID:   resp.Claims.TokenId,
		Subject:   resp.Claims.Subject,
		SessionID: resp.Claims.SessionId,
		UserID:    resp.Claims.UserId,
		AccountID: resp.Claims.AccountId,
		TenantID:  resp.Claims.TenantId,
		Issuer:    resp.Claims.Issuer,
		Audience:  resp.Claims.Audience,
		AMR:       append([]string(nil), resp.Claims.Amr...),
		Extra:     make(map[string]interface{}),
	}
	if resp.Claims.ExpiresAt != nil {
		claims.ExpiresAt = resp.Claims.ExpiresAt.AsTime()
	}
	if resp.Claims.IssuedAt != nil {
		claims.IssuedAt = resp.Claims.IssuedAt.AsTime()
	}
	if resp.Claims.Attributes != nil {
		for k, v := range resp.Claims.Attributes {
			claims.Extra[k] = v
		}
	}

	logger.L(ctx).Debugw("RemoteVerifyStrategy verify success", "strategy", s.Name(), "subject", claims.Subject, "tenant_id", claims.TenantID)
	metadata := buildVerifyMetadataFromProto(resp.Metadata)
	if metadata == nil {
		metadata = buildVerifyMetadataFromClaims(claims)
	}
	return &VerifyResult{
		Valid:    true,
		Claims:   claims,
		Metadata: metadata,
	}, nil
}

func (s *RemoteVerifyStrategy) expectedAudience(opts *VerifyOptions) []string {
	if opts != nil && len(opts.ExpectedAudience) > 0 {
		return append([]string(nil), opts.ExpectedAudience...)
	}
	if s.config != nil && len(s.config.AllowedAudience) > 0 {
		return append([]string(nil), s.config.AllowedAudience...)
	}
	return nil
}

func (s *RemoteVerifyStrategy) expectedIssuer(opts *VerifyOptions) string {
	if opts != nil && opts.ExpectedIssuer != "" {
		return opts.ExpectedIssuer
	}
	if s.config != nil {
		return s.config.AllowedIssuer
	}
	return ""
}
