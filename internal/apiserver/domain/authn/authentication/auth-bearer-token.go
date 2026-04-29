package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func init() {
	// 注册认证凭据构建器
	RegisterCredentialBuilder(AuthBearerToken, newBearerTokenCredential)
}

// ====================== 认证凭据（认证所需的数据） ========================

// BearerTokenCredential 表示基于 Bearer access token 的认证凭据。
type BearerTokenCredential struct {
	TenantID    meta.ID
	RemoteIP    string
	UserAgent   string
	AccessToken string
}

// Scenario 返回认证场景
func (c *BearerTokenCredential) Scenario() Scenario {
	return AuthBearerToken
}

func newBearerTokenCredential(input AuthInput) (AuthCredential, error) {
	if input.AccessToken == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "bearer token is required for bearer token authentication")
	}
	return &BearerTokenCredential{
		TenantID:    input.TenantID,
		RemoteIP:    input.RemoteIP,
		UserAgent:   input.UserAgent,
		AccessToken: input.AccessToken,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// BearerTokenAuthStrategy 用于 API 调用场景，使用 access token 认证。
type BearerTokenAuthStrategy struct {
	scenario      Scenario
	tokenVerifier TokenVerifier
}

// 实现认证策略接口
var _ AuthStrategy = (*BearerTokenAuthStrategy)(nil)

func NewBearerTokenAuthStrategy(
	tokenVerifier TokenVerifier,
) *BearerTokenAuthStrategy {
	return &BearerTokenAuthStrategy{
		scenario:      AuthBearerToken,
		tokenVerifier: tokenVerifier,
	}
}

// Kind 返回认证策略类型
func (j *BearerTokenAuthStrategy) Kind() Scenario {
	return j.scenario
}

func (j *BearerTokenAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	l := logger.L(ctx)

	tokenCredential, ok := credential.(*BearerTokenCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("bearer token strategy expects *BearerTokenCredential, got %T", credential)
	}

	l.Debugw("Bearer Token认证：步骤1 - 验证令牌",
		"scenario", string(AuthBearerToken),
	)

	// Step 1: 验证 access token
	userID, accountID, tenantID, err := j.tokenVerifier.VerifyAccessToken(ctx, tokenCredential.AccessToken)
	if err != nil {
		// Token 无效/过期/被撤销 - 返回业务失败
		l.Warnw("令牌验证失败",
			"scenario", string(AuthBearerToken),
			"error", err.Error(),
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrInvalidCredential, // 使用统一的凭据无效错误码
		}, nil
	}

	l.Debugw("Bearer Token认证：步骤2 - 构造认证主体",
		"scenario", string(AuthBearerToken),
		"user_id", userID.String(),
		"account_id", accountID.String(),
	)

	l.Debugw("Bearer Token认证成功",
		"scenario", string(AuthBearerToken),
		"user_id", userID.String(),
		"account_id", accountID.String(),
	)

	// Step 3: 认证成功，构造认证主体
	principal := &Principal{
		UserID:    userID,
		AccountID: accountID,
		TenantID:  tenantID,
		AMR:       []string{string(AMRBearerToken)}, // 记录认证方法
		Claims:    make(map[string]any),
	}

	// 可以添加额外的 claims
	principal.Claims["auth_method"] = "jwt_token"
	if tokenCredential.RemoteIP != "" {
		principal.Claims["remote_ip"] = tokenCredential.RemoteIP
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		CredentialID: meta.FromUint64(0), // Bearer token 认证不对应具体的凭据记录
	}, nil
}
