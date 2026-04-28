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
	RegisterCredentialBuilder(AuthJWTToken, newJWTTokenCredential)
}

// ====================== 认证凭据（认证所需的数据） ========================

// JWTTokenCredential JWT Token 认证凭据
type JWTTokenCredential struct {
	TenantID    meta.ID
	RemoteIP    string
	UserAgent   string
	AccessToken string
}

// Scenario 返回认证场景
func (c *JWTTokenCredential) Scenario() Scenario {
	return AuthJWTToken
}

// newJWTTokenCredential 构造 JWT Token 认证凭据
func newJWTTokenCredential(input AuthInput) (AuthCredential, error) {
	if input.AccessToken == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "jwt token is required for jwt token authentication")
	}
	return &JWTTokenCredential{
		TenantID:    input.TenantID,
		RemoteIP:    input.RemoteIP,
		UserAgent:   input.UserAgent,
		AccessToken: input.AccessToken,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// JWTTokenAuthStrategy JWT Token 认证策略
// 用于 API 调用场景，使用 JWT 访问令牌进行认证
type JWTTokenAuthStrategy struct {
	scenario      Scenario
	tokenVerifier TokenVerifier
}

// 实现认证策略接口
var _ AuthStrategy = (*JWTTokenAuthStrategy)(nil)

// NewJWTTokenAuthStrategy 构造函数（注入依赖）
func NewJWTTokenAuthStrategy(
	tokenVerifier TokenVerifier,
) *JWTTokenAuthStrategy {
	return &JWTTokenAuthStrategy{
		scenario:      AuthJWTToken,
		tokenVerifier: tokenVerifier,
	}
}

// Kind 返回认证策略类型
func (j *JWTTokenAuthStrategy) Kind() Scenario {
	return j.scenario
}

// Authenticate 执行 JWT Token 认证
// 认证流程：
// 1. 验证 JWT Token（签名、过期、撤销标记）
// 2. 从 Token 中提取用户ID、账户ID
// 3. 返回认证判决
func (j *JWTTokenAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	l := logger.L(ctx)

	tokenCredential, ok := credential.(*JWTTokenCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("jwt token strategy expects *JWTTokenCredential, got %T", credential)
	}

	l.Debugw("JWT Token认证：步骤1 - 验证令牌",
		"scenario", string(AuthJWTToken),
	)

	// Step 1: 验证 JWT Token
	userID, accountID, tenantID, err := j.tokenVerifier.VerifyAccessToken(ctx, tokenCredential.AccessToken)
	if err != nil {
		// Token 无效/过期/被撤销 - 返回业务失败
		l.Warnw("令牌验证失败",
			"scenario", string(AuthJWTToken),
			"error", err.Error(),
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrInvalidCredential, // 使用统一的凭据无效错误码
		}, nil
	}

	l.Debugw("JWT Token认证：步骤2 - 构造认证主体",
		"scenario", string(AuthJWTToken),
		"user_id", userID.String(),
		"account_id", accountID.String(),
	)

	l.Debugw("JWT Token认证成功",
		"scenario", string(AuthJWTToken),
		"user_id", userID.String(),
		"account_id", accountID.String(),
	)

	// Step 3: 认证成功，构造认证主体
	principal := &Principal{
		UserID:    userID,
		AccountID: accountID,
		TenantID:  tenantID,
		AMR:       []string{string(AMRJWTToken)}, // 记录认证方法
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
		CredentialID: meta.FromUint64(0), // JWT Token 认证不对应具体的凭据记录
	}, nil
}
