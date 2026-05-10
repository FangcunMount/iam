package authentication

import (
	"context"
	"time"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ================== Repository Interfaces (Driven Ports) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// LoginIdentityCredentialRepository 凭据仓储（查询认证凭据）
// 职责：提供 LoginIdentity 绑定的长期认证材料查询能力
type LoginIdentityCredentialRepository interface {
	FindPasswordCredentialByLoginIdentity(ctx context.Context, loginIdentityID meta.ID) (*PasswordCredentialLookup, error)
}

// PasswordCredentialLookup 是密码认证所需的长期 Credential 读模型。
type PasswordCredentialLookup struct {
	CredentialID meta.ID
	PasswordHash string
	Status       credDomain.CredentialStatus
	LockedUntil  *time.Time
}

// LoginIdentityLookup is the authentication read model for LoginIdentity.
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

// LoginIdentityRepository resolves login identities before credential lookup.
type LoginIdentityRepository interface {
	FindUsernameIdentity(ctx context.Context, tenantID meta.ID, username string) (*LoginIdentityLookup, error)
	FindLoginIdentityByProviderKey(ctx context.Context, provider loginidentity.Provider, realm, identifier string) (*LoginIdentityLookup, error)
	FindLoginIdentityByGlobalIdentifier(ctx context.Context, provider loginidentity.Provider, globalIdentifier string) (*LoginIdentityLookup, error)
	IsLoginIdentityActive(ctx context.Context, loginIdentityID meta.ID) (bool, error)
}
