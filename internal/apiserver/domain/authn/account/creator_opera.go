package account

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== 运营账户创建策略 ====================

// OperaCreatorStrategy 运营账户创建策略（TypeOpera）
// 运营账户可以绑定多种凭据：密码、手机OTP等
type OperaCreatorStrategy struct{}

var _ CreatorStrategy = (*OperaCreatorStrategy)(nil)

// NewOperaCreatorStrategy 创建运营账户创建策略
func NewOperaCreatorStrategy() *OperaCreatorStrategy {
	return &OperaCreatorStrategy{}
}

// Kind 返回策略支持的账户类型
func (s *OperaCreatorStrategy) Kind() AccountType {
	return TypeOpera
}

// PrepareData 准备运营账户创建参数
func (s *OperaCreatorStrategy) PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error) {
	if input.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "opera account requires scoped_tenant_id")
	}
	externalID, err := pickOperaExternalID(input)
	if err != nil {
		return nil, err
	}
	appID := AppId("opera")

	return &CreationParams{
		UserID:         input.UserID,
		AccountType:    TypeOpera,
		AppID:          appID,
		ExternalID:     externalID,
		ScopedTenantID: input.ScopedTenantID,
		Profile:        input.Profile,
		Meta:           input.Meta,
		ParamsJSON:     input.ParamsJSON,
	}, nil
}

// Create 创建运营账户实体
func (s *OperaCreatorStrategy) Create(ctx context.Context, params *CreationParams) (*Account, error) {
	// 创建账户实体
	account := NewAccount(
		params.UserID,
		TypeOpera,
		params.ExternalID,
		WithAppID(params.AppID),
		WithScopedTenantID(params.ScopedTenantID),
	)
	if err := account.ValidateScopedTenantInvariant(); err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "%v", err)
	}

	// 设置资料和元数据
	if len(params.Profile) > 0 {
		account.Profile = params.Profile
	}
	if len(params.Meta) > 0 {
		account.Meta = params.Meta
	}

	return account, nil
}

func pickOperaExternalID(input CreationInput) (ExternalID, error) {
	if s := strings.TrimSpace(input.OperaLoginID); s != "" {
		return ExternalID(s), nil
	}
	if !input.Email.IsEmpty() {
		return ExternalID(input.Email.String()), nil
	}
	if !input.Phone.IsEmpty() {
		return ExternalID(input.Phone.String()), nil
	}
	return "", perrors.WithCode(code.ErrInvalidArgument,
		"opera account: set opera_login_id, or provide email, or phone for external_id")
}
