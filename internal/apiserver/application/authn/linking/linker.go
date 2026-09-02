package linking

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type linker struct {
	deps Dependencies
}

var _ Linker = (*linker)(nil)

// NewLinker 创建登录身份绑定应用服务。
func NewLinker(deps Dependencies) Linker {
	return &linker{deps: deps}
}

// Link 绑定登录身份（对外入口）。
func (l *linker) Link(ctx context.Context, req LinkRequest) (*LinkResult, error) {
	return l.link(ctx, req)
}

// link 模板方法：prepare（多态 Input）→ ensure（固定步骤）。
func (l *linker) link(ctx context.Context, req LinkRequest) (*LinkResult, error) {
	// 检查用户 ID 是否有效。
	if err := requireUserID(req.UserID); err != nil {
		return nil, err
	}

	// 如果输入为空，则返回错误。
	if req.Input == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "link input is required")
	}

	// 准备登录身份。
	prepared, err := req.Input.prepareLink(ctx, l.prepareDeps(), req.UserID)
	if err != nil {
		return nil, err
	}

	// 检查全局标识符是否唯一。
	if prepared.requireGlobalUniqueness {
		if err := l.ensureGlobalIdentifierAvailable(ctx, req.UserID, prepared.key); err != nil {
			return nil, err
		}
	}

	// 确保提供者密钥唯一并创建或复用登录身份。
	return l.ensureProviderKey(ctx, req.UserID, prepared.key, prepared.build)
}

// prepareDeps 准备依赖。
func (l *linker) prepareDeps() linkPrepareDeps {
	return linkPrepareDeps{
		phoneLinkOTP: l.deps.PhoneLinkOTP,
		resolver:     l.deps.ExternalIdentity,
		now:          l.deps.Now,
	}
}

// repo 获取登录身份仓库。
func (l *linker) repo() loginidentity.Repository {
	return l.deps.LoginIdentities
}

func (l *linker) identityUnlinker() loginidentity.AtomicIdentityUnlinker {
	if l.deps.IdentityUnlinker != nil {
		return l.deps.IdentityUnlinker
	}
	unlinker, _ := l.deps.LoginIdentities.(loginidentity.AtomicIdentityUnlinker)
	return unlinker
}

// now 获取当前时间。
func (l *linker) now() time.Time {
	return l.prepareDeps().currentTime()
}

// recentAuthWindow 获取最近认证窗口。
func (l *linker) recentAuthWindow() time.Duration {
	if l.deps.RecentAuthWindow > 0 {
		return l.deps.RecentAuthWindow
	}
	return 10 * time.Minute
}

// requireUserID 检查用户 ID 是否有效。
func requireUserID(userID meta.ID) error {
	if userID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "user_id is required")
	}
	return nil
}

// verifiedIdentityBuild 构建已验证登录身份。
func verifiedIdentityBuild(userID meta.ID, key loginidentity.ProviderKey, verifiedAt time.Time) func() (*loginidentity.LoginIdentity, error) {
	return func() (*loginidentity.LoginIdentity, error) {
		return loginidentity.NewBuilder(userID).
			FromProviderKey(key).
			WithVerifiedAt(verifiedAt).
			Build()
	}
}
