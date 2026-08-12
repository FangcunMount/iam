package authentication

import (
	"context"
	"time"

	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ================== Repository Interfaces (Driven Ports) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// LoginIdentityRepository 登录身份仓储（查询登录身份）
// 职责：提供登录身份查询能力
type LoginIdentityRepository interface {
	// -- 查询登录身份 ——
	FindUsernameIdentity(ctx context.Context, tenantID meta.ID, username string) (*LoginIdentityLookup, error)
	FindLoginIdentityByProviderKey(ctx context.Context, provider loginidentity.Provider, realm, identifier string) (*LoginIdentityLookup, error)
	FindLoginIdentityByGlobalIdentifier(ctx context.Context, provider loginidentity.Provider, globalIdentifier string) (*LoginIdentityLookup, error)

	// -- 判断登录身份是否活动 ——
	IsLoginIdentityActive(ctx context.Context, loginIdentityID meta.ID) (bool, error)
}

// LoginIdentityCredentialRepository 凭据仓储（查询认证凭据）
// 职责：提供 LoginIdentity 绑定的长期认证材料查询能力
type LoginIdentityCredentialRepository interface {
	// -- 查询密码凭据 ——
	FindPasswordCredentialByLoginIdentity(ctx context.Context, loginIdentityID meta.ID) (*PasswordCredentialLookup, error)
}

// PasswordCredentialLookup 是密码认证所需的长期凭据读模型
type PasswordCredentialLookup struct {
	CredentialID meta.ID
	PasswordHash string
	Status       credDomain.CredentialStatus
	LockedUntil  *time.Time
}

// LoginIdentityLookup 是登录身份的读模型
type LoginIdentityLookup struct {
	LoginIdentityID  meta.ID
	UserID           meta.ID
	Provider         loginidentity.Provider
	Realm            string
	Identifier       string
	GlobalIdentifier string
	Status           loginidentity.Status
	ScopedTenantID   meta.ID
}
