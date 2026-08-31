package externalidentity

import (
	"fmt"
	"time"

	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
)

// WechatIdentity is the AuthN view of a verified WeChat identity.
type WechatIdentity struct {
	Provider   loginidentity.Provider
	Realm      string
	OpenID     string
	UnionID    string
	VerifiedAt time.Time
}

// WecomIdentity is the AuthN view of a verified WeCom identity.
type WecomIdentity struct {
	Realm      string
	UserID     string
	OpenUserID string
	VerifiedAt time.Time
}

func Wechat(identity idpidentity.ExternalIdentity) (WechatIdentity, error) {
	// 验证外部身份信息是否为微信身份信息
	provider, err := wechatProvider(identity.Provider())
	if err != nil {
		return WechatIdentity{}, err
	}

	// 验证外部身份信息中的OpenID是否有效
	openID, ok := identity.Identifier(idpidentity.IdentifierOpenID)
	if !ok || openID == "" {
		return WechatIdentity{}, fmt.Errorf("verified wechat identity is missing open_id")
	}

	// 验证外部身份信息中的UnionID是否有效
	unionID, _ := identity.Identifier(idpidentity.IdentifierUnionID)

	// 构建微信身份信息
	return WechatIdentity{
		Provider:   provider,
		Realm:      identity.Realm(),
		OpenID:     openID,
		UnionID:    unionID,
		VerifiedAt: identity.VerifiedAt(),
	}, nil
}

func Wecom(identity idpidentity.ExternalIdentity) (WecomIdentity, error) {
	// 验证外部身份信息是否为企业微信身份信息
	if identity.Provider() != idpidentity.ProviderWecom {
		return WecomIdentity{}, fmt.Errorf("external identity provider %q is not wecom", identity.Provider())
	}

	// 验证外部身份信息中的UserID是否有效
	userID, _ := identity.Identifier(idpidentity.IdentifierUserID)
	openUserID, _ := identity.Identifier(idpidentity.IdentifierOpenUserID)

	// 验证外部身份信息中的OpenUserID是否有效
	if userID == "" && openUserID == "" {
		return WecomIdentity{}, fmt.Errorf("verified wecom identity has no usable identifier")
	}

	// 构建企业微信身份信息
	return WecomIdentity{
		Realm:      identity.Realm(),
		UserID:     userID,
		OpenUserID: openUserID,
		VerifiedAt: identity.VerifiedAt(),
	}, nil
}

// ProviderKey 验证外部身份信息是否为微信身份信息或企业微信身份信息
func ProviderKey(identity idpidentity.ExternalIdentity) (loginidentity.ProviderKey, error) {
	// 验证外部身份信息是否为微信身份信息或企业微信身份信息
	switch identity.Provider() {
	case idpidentity.ProviderWechatMinip, idpidentity.ProviderWechatOpen:
		// 验证外部身份信息是否为微信身份信息
		// 映射微信身份信息
		wechatIdentity, err := Wechat(identity)
		if err != nil {
			return loginidentity.ProviderKey{}, err
		}
		if wechatIdentity.Provider == loginidentity.ProviderWechatMinip {
			return loginidentity.NewWechatMinipProviderKey(wechatIdentity.Realm, wechatIdentity.OpenID, wechatIdentity.UnionID)
		}
		return loginidentity.NewWechatOpenProviderKey(wechatIdentity.Realm, wechatIdentity.OpenID, wechatIdentity.UnionID)
	case idpidentity.ProviderWecom:
		// 验证外部身份信息是否为企业微信身份信息
		// 映射企业微信身份信息
		wecomIdentity, err := Wecom(identity)
		if err != nil {
			return loginidentity.ProviderKey{}, err
		}
		// Binding intentionally keeps the historic user_id-only behavior.
		return loginidentity.NewWecomProviderKey(wecomIdentity.Realm, wecomIdentity.UserID)
	default:
		return loginidentity.ProviderKey{}, fmt.Errorf("unsupported external identity provider: %q", identity.Provider())
	}
}

// wechatProvider 验证外部身份信息是否为微信身份信息
func wechatProvider(provider idpidentity.Provider) (loginidentity.Provider, error) {
	// 验证外部身份信息是否为微信身份信息
	switch provider {
	case idpidentity.ProviderWechatMinip:
		return loginidentity.ProviderWechatMinip, nil
	case idpidentity.ProviderWechatOpen:
		return loginidentity.ProviderWechatOpen, nil
	default:
		return "", fmt.Errorf("external identity provider %q is not wechat", provider)
	}
}
