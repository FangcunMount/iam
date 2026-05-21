package loginidentity

import (
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Builder 用于构建 LoginIdentity 对象的构建器。
type Builder struct {
	userID           meta.ID
	provider         Provider          // 提供者
	realm            string            // 域
	identifier       string            // 标识
	globalIdentifier string            // 全局标识
	id               meta.ID           // 登录身份ID
	status           Status            // 状态
	verifiedAt       *time.Time        // 验证时间
	linkedAt         time.Time         // 绑定时间
	profile          map[string]string // 资料
	meta             map[string]string // 元数据
}

// NewBuilder 创建登录身份构造器。
func NewBuilder(userID meta.ID) *Builder {
	return &Builder{
		userID: userID,
		status: StatusActive,
	}
}

// Username 使用 username provider 构造登录身份。
func (b *Builder) Username(realm, username string) *Builder {
	return b.FromProviderKey(NewProviderKey(ProviderUsername, normalizeRealm(realm), username))
}

// Phone 使用 phone provider 构造登录身份。
func (b *Builder) Phone(phone string) *Builder {
	return b.FromProviderKey(PhoneProviderKey(phone))
}

// WechatMinip 使用微信小程序 provider 构造登录身份。
func (b *Builder) WechatMinip(appID, openID, unionID string) *Builder {
	return b.FromProviderKey(WechatMinipProviderKey(appID, openID, unionID))
}

// Wecom 使用企业微信 provider 构造登录身份。
func (b *Builder) Wecom(corpID, userIDInWecom string) *Builder {
	return b.FromProviderKey(WecomProviderKey(corpID, userIDInWecom))
}

// FromProviderKey 使用已经解析好的 ProviderKey 构造登录身份。
func (b *Builder) FromProviderKey(key ProviderKey) *Builder {
	if b == nil {
		return b
	}
	b.provider = key.Provider
	b.realm = key.Realm
	b.identifier = key.Identifier
	b.globalIdentifier = key.GlobalIdentifier
	return b
}

// WithID 设置登录身份 ID。
func (b *Builder) WithID(id meta.ID) *Builder {
	if b == nil {
		return b
	}
	b.id = id
	return b
}

// WithStatus 设置登录身份状态。
func (b *Builder) WithStatus(status Status) *Builder {
	if b == nil {
		return b
	}
	b.status = status
	return b
}

// WithVerifiedAt 设置验证时间。
func (b *Builder) WithVerifiedAt(verifiedAt time.Time) *Builder {
	if b == nil {
		return b
	}
	t := verifiedAt
	b.verifiedAt = &t
	return b
}

// WithProfile 设置资料。
func (b *Builder) WithProfile(profile map[string]string) *Builder {
	if b == nil {
		return b
	}
	b.profile = cloneStringMap(profile)
	return b
}

// WithMeta 设置元数据。
func (b *Builder) WithMeta(metaData map[string]string) *Builder {
	if b == nil {
		return b
	}
	b.meta = cloneStringMap(metaData)
	return b
}

// WithLinkedAt 设置绑定时间。
func (b *Builder) WithLinkedAt(linkedAt time.Time) *Builder {
	if b == nil {
		return b
	}
	b.linkedAt = linkedAt
	return b
}

// Build 构造 LoginIdentity 并执行领域校验。
func (b *Builder) Build() (*LoginIdentity, error) {
	if b == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login identity builder is nil")
	}
	if b.userID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "user_id is required")
	}
	if !b.provider.Validate() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity provider: %s", b.provider)
	}
	realm := strings.TrimSpace(b.realm)
	if realm == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "realm is required")
	}
	identifier := strings.TrimSpace(b.identifier)
	if identifier == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "identifier is required")
	}
	if !b.status.Validate() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity status: %s", b.status)
	}

	now := time.Now()
	linkedAt := b.linkedAt
	if linkedAt.IsZero() {
		linkedAt = now
	}
	profile := cloneStringMap(b.profile)
	if profile == nil {
		profile = map[string]string{}
	}
	metaData := cloneStringMap(b.meta)
	if metaData == nil {
		metaData = map[string]string{}
	}

	return &LoginIdentity{
		ID:               b.id,
		UserID:           b.userID,
		Provider:         b.provider,
		Realm:            realm,
		Identifier:       identifier,
		GlobalIdentifier: strings.TrimSpace(b.globalIdentifier),
		Status:           b.status,
		VerifiedAt:       copyTimePtr(b.verifiedAt),
		LinkedAt:         linkedAt,
		Profile:          profile,
		Meta:             metaData,
	}, nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	t := *src
	return &t
}
