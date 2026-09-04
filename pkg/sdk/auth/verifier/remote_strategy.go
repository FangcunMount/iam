package verifier

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/logger"
	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
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
	policy := newVerificationPolicy(s.config, opts)
	if err := policy.validateTokenEnvelope(tokenString); err != nil {
		return nil, err
	}

	resp, err := s.authClient.VerifyToken(ctx, &authnv2.VerifyTokenRequest{
		AccessToken:        tokenString,
		ForceRemote:        opts.ForceRemote,
		IncludeMetadata:    opts.IncludeMetadata,
		ExpectedIssuer:     s.expectedIssuer(opts),
		ExpectedAudience:   s.expectedAudience(opts),
		AcceptedTokenTypes: acceptedProtoTokenTypes(opts),
	})
	if err != nil {
		logger.L(ctx).Warnw("RemoteVerifyStrategy verify failed", "strategy", s.Name(), "error", err.Error())
		return nil, err
	}
	if resp == nil {
		return nil, invalidTokenError("remote verification returned no response")
	}
	if !resp.Valid {
		logger.L(ctx).Warnw("RemoteVerifyStrategy token invalid", "strategy", s.Name())
		return nil, invalidTokenError("remote verification rejected token")
	}
	if resp.Claims == nil {
		return nil, invalidTokenError("remote verification returned no claims")
	}

	claims := &TokenClaims{
		TokenID:         resp.Claims.TokenId,
		Subject:         resp.Claims.Subject,
		SessionID:       resp.Claims.SessionId,
		UserID:          resp.Claims.UserId,
		LoginIdentityID: resp.Claims.LoginIdentityId,
		Issuer:          resp.Claims.Issuer,
		Audience:        resp.Claims.Audience,
		AMR:             append([]string(nil), resp.Claims.Amr...),
		TokenType:       protoTokenTypeString(resp.Claims.GetTokenType()),
		Attributes:      cloneStringMapValues(resp.Claims.Attributes),
		Extra:           make(map[string]interface{}),
	}
	applyTenantAndOrg(claims, resp.Claims.TenantId, resp.Claims.OrgId)
	if resp.Claims.GetTenantDomain() != "" {
		claims.TenantDomain = resp.Claims.GetTenantDomain()
	}
	if resp.Claims.ExpiresAt != nil {
		claims.ExpiresAt = resp.Claims.ExpiresAt.AsTime()
	}
	if resp.Claims.IssuedAt != nil {
		claims.IssuedAt = resp.Claims.IssuedAt.AsTime()
	}
	if resp.Claims.NotBefore != nil {
		claims.NotBefore = resp.Claims.NotBefore.AsTime()
	}
	if resp.Claims.AuthenticatedAt != nil {
		claims.AuthenticatedAt = resp.Claims.AuthenticatedAt.AsTime()
		claims.AuthTime = claims.AuthenticatedAt
	}
	if resp.Claims.Attributes != nil {
		for k, v := range resp.Claims.Attributes {
			claims.Extra[k] = v
		}
	}
	if err := policy.validateTokenType(claims.TokenType); err != nil {
		return nil, err
	}

	logger.L(ctx).Debugw("RemoteVerifyStrategy verify success", "strategy", s.Name(), "subject", claims.Subject, "tenant_domain", claims.TenantDomain, "org_id", claims.OrgID)
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

func cloneStringMapValues(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func acceptedProtoTokenTypes(opts *VerifyOptions) []authnv2.TokenType {
	if opts != nil && len(opts.AllowedTokenTypes) > 0 {
		return append([]authnv2.TokenType(nil), opts.AllowedTokenTypes...)
	}
	return []authnv2.TokenType{authnv2.TokenType_TOKEN_TYPE_ACCESS}
}

func protoTokenTypeString(tokenType authnv2.TokenType) string {
	switch tokenType {
	case authnv2.TokenType_TOKEN_TYPE_SERVICE:
		return "service"
	case authnv2.TokenType_TOKEN_TYPE_REFRESH:
		return "refresh"
	case authnv2.TokenType_TOKEN_TYPE_ACCESS:
		return "access"
	default:
		return "access" // 历史远端响应缺失 token_type 时保持有界兼容
	}
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
