package loginidentity

import (
	"strings"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ProviderKey 唯一键，用于解析登录身份
// 用于唯一标识一个登录身份，包括提供者、域、标识和全局标识。
// 例如：username:tenant_id:username、phone:global:+1234567890、wechat_minip:appid:openid:unionid、wecom:corp_id:userid。
type ProviderKey struct {
	Provider         Provider // 提供者
	Realm            string   // 域
	Identifier       string   // 标识
	GlobalIdentifier string   // 全局标识
}

// NewProviderKey 创建唯一键
func NewProviderKey(provider Provider, realm, identifier string) ProviderKey {
	return ProviderKey{
		Provider:   provider,
		Realm:      strings.TrimSpace(realm),
		Identifier: strings.TrimSpace(identifier),
	}
}

// UsernameProviderKey 用户名唯一键
func UsernameProviderKey(tenantID meta.ID, username string) ProviderKey {
	return NewProviderKey(ProviderUsername, UsernameRealm(tenantID), username)
}

// MockConsumerProviderKey 模拟消费者唯一键
func MockConsumerProviderKey(username string) ProviderKey {
	return NewProviderKey(ProviderUsername, RealmDefault, username)
}

// PhoneProviderKey 手机唯一键
func PhoneProviderKey(phone string) ProviderKey {
	return NewProviderKey(ProviderPhone, RealmGlobal, phone)
}

// WechatMinipProviderKey 微信小程序唯一键
func WechatMinipProviderKey(appID, openID, unionID string) ProviderKey {
	key := NewProviderKey(ProviderWechatMinip, appID, openID)
	key.GlobalIdentifier = strings.TrimSpace(unionID)
	return key
}

// WechatOpenProviderKey 微信开放平台唯一键
func WechatOpenProviderKey(appID, openID, unionID string) ProviderKey {
	key := NewProviderKey(ProviderWechatOpen, appID, openID)
	key.GlobalIdentifier = strings.TrimSpace(unionID)
	return key
}

// WecomProviderKey 企业微信唯一键
func WecomProviderKey(corpID, userIDInWecom string) ProviderKey {
	return NewProviderKey(ProviderWecom, corpID, userIDInWecom)
}

// UsernameRealm 用户名域
func UsernameRealm(tenantID meta.ID) string {
	if tenantID.IsZero() {
		return RealmDefault
	}
	return tenantID.String()
}

// IsValid 是否有效
func (k ProviderKey) IsValid() bool {
	return k.Provider.Validate() &&
		strings.TrimSpace(k.Realm) != "" &&
		strings.TrimSpace(k.Identifier) != ""
}
