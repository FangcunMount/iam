package account

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// MockConsumerCreatorStrategy 内部 mock C 端账户创建策略。
type MockConsumerCreatorStrategy struct{}

var _ CreatorStrategy = (*MockConsumerCreatorStrategy)(nil)

func NewMockConsumerCreatorStrategy() *MockConsumerCreatorStrategy {
	return &MockConsumerCreatorStrategy{}
}

func (s *MockConsumerCreatorStrategy) Kind() AccountType {
	return TypeMockConsumer
}

func (s *MockConsumerCreatorStrategy) PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error) {
	_ = ctx

	if !input.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "mock-consumer account does not support scoped_tenant_id")
	}
	if input.Email.IsEmpty() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "mock-consumer account requires email")
	}

	externalID := strings.TrimSpace(input.Email.String())
	if externalID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "mock-consumer external_id cannot be empty")
	}

	return &CreationParams{
		UserID:      input.UserID,
		AccountType: TypeMockConsumer,
		AppID:       MockConsumerAppID,
		ExternalID:  ExternalID(externalID),
		Profile:     input.Profile,
		Meta:        input.Meta,
		ParamsJSON:  input.ParamsJSON,
	}, nil
}

func (s *MockConsumerCreatorStrategy) Create(ctx context.Context, params *CreationParams) (*Account, error) {
	_ = ctx

	account := NewAccount(
		params.UserID,
		TypeMockConsumer,
		params.ExternalID,
		WithAppID(params.AppID),
	)
	if err := account.ValidateScopedTenantInvariant(); err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "%v", err)
	}
	if len(params.Profile) > 0 {
		account.Profile = params.Profile
	}
	if len(params.Meta) > 0 {
		account.Meta = params.Meta
	}
	return account, nil
}
