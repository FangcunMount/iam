package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ensureProviderKey 确保 provider key 唯一并创建或复用登录身份。
func (l *linker) ensureProviderKey(
	ctx context.Context,
	userID meta.ID,
	key loginidentity.ProviderKey,
	build func() (*loginidentity.LoginIdentity, error),
) (*LinkResult, error) {
	if !key.IsValid() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity provider key")
	}

	// 查询是否存在相同的登录身份。
	existing, err := l.repo().GetByProviderKey(ctx, key.Provider, key.Realm, key.Identifier)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// 如果存在相同的登录身份，则检查是否属于当前用户。
		if existing.UserID != userID {
			return nil, perrors.WithCode(code.ErrLoginIdentityExists, "login identity already belongs to another user")
		}
		// 如果登录身份不是活跃的，则返回错误。
		if !existing.IsActive() {
			return nil, perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is not active")
		}
		return &LinkResult{Identity: existing, Reused: true}, nil
	}

	// 创建新的登录身份。
	identity, err := build()
	if err != nil {
		return nil, err
	}

	// 创建新的登录身份。
	if err := l.repo().Create(ctx, identity); err != nil {
		return nil, err
	}

	// 返回新的登录身份。
	return &LinkResult{Identity: identity}, nil
}

// ensureGlobalIdentifierAvailable 确保全局标识符唯一。
func (l *linker) ensureGlobalIdentifierAvailable(ctx context.Context, userID meta.ID, key loginidentity.ProviderKey) error {
	// 如果全局标识符为空，则返回成功。
	if strings.TrimSpace(key.GlobalIdentifier) == "" {
		return nil
	}

	// 查询是否存在相同的登录身份。
	existing, err := l.repo().GetByGlobalIdentifier(ctx, key.Provider, key.GlobalIdentifier)
	if err != nil {
		return err
	}

	// 如果存在相同的登录身份，则检查是否属于当前用户。
	if existing != nil && existing.UserID != userID {
		return perrors.WithCode(code.ErrGlobalIdentifierExists, "global identifier already belongs to another user")
	}
	return nil
}
