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
	verifiedAt       *time.Time        // 验证时间
	profile          map[string]string // 资料
	meta             map[string]string // 元数据
}

// NewBuilder 创建登录身份构造器。
func NewBuilder(userID meta.ID) *Builder {
	return &Builder{userID: userID}
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

// WechatOpen 使用微信开放平台 provider 构造登录身份。
func (b *Builder) WechatOpen(appID, openID, unionID string) *Builder {
	return b.FromProviderKey(WechatOpenProviderKey(appID, openID, unionID))
}

// Wecom 使用企业微信 provider 构造登录身份。
func (b *Builder) Wecom(corpID, userIDInWecom string) *Builder {
	return b.FromProviderKey(WecomProviderKey(corpID, userIDInWecom))
}

// FromProviderKey 使用已经解析好的 ProviderKey 构造登录身份。
// 该方法用于从已经解析好的 ProviderKey 构造登录身份，通常用于从外部系统导入登录身份。
func (b *Builder) FromProviderKey(key ProviderKey) *Builder {
	if b == nil {
		return b
	}

	// 设置登录身份的提供者
	b.provider = key.Provider
	// 设置登录身份的域
	b.realm = key.Realm
	// 设置登录身份的标识
	b.identifier = key.Identifier
	// 设置登录身份的全局标识
	b.globalIdentifier = key.GlobalIdentifier
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
	now := time.Now()
	profile := cloneStringMap(b.profile)
	if profile == nil {
		profile = map[string]string{}
	}
	metaData := cloneStringMap(b.meta)
	if metaData == nil {
		metaData = map[string]string{}
	}

	return &LoginIdentity{
		UserID:           b.userID,
		Provider:         b.provider,
		Realm:            realm,
		Identifier:       identifier,
		GlobalIdentifier: strings.TrimSpace(b.globalIdentifier),
		Status:           StatusActive,
		VerifiedAt:       copyTimePtr(b.verifiedAt),
		LinkedAt:         now,
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
