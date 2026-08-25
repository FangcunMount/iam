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
	provider, err := wechatProvider(identity.Provider())
	if err != nil {
		return WechatIdentity{}, err
	}
	openID, ok := identity.Identifier(idpidentity.IdentifierOpenID)
	if !ok || openID == "" {
		return WechatIdentity{}, fmt.Errorf("verified wechat identity is missing open_id")
	}
	unionID, _ := identity.Identifier(idpidentity.IdentifierUnionID)
	return WechatIdentity{
		Provider:   provider,
		Realm:      identity.Realm(),
		OpenID:     openID,
		UnionID:    unionID,
		VerifiedAt: identity.VerifiedAt(),
	}, nil
}

func Wecom(identity idpidentity.ExternalIdentity) (WecomIdentity, error) {
	if identity.Provider() != idpidentity.ProviderWecom {
		return WecomIdentity{}, fmt.Errorf("external identity provider %q is not wecom", identity.Provider())
	}
	userID, _ := identity.Identifier(idpidentity.IdentifierUserID)
	openUserID, _ := identity.Identifier(idpidentity.IdentifierOpenUserID)
	if userID == "" && openUserID == "" {
		return WecomIdentity{}, fmt.Errorf("verified wecom identity has no usable identifier")
	}
	return WecomIdentity{
		Realm:      identity.Realm(),
		UserID:     userID,
		OpenUserID: openUserID,
		VerifiedAt: identity.VerifiedAt(),
	}, nil
}

func ProviderKey(identity idpidentity.ExternalIdentity) (loginidentity.ProviderKey, error) {
	switch identity.Provider() {
	case idpidentity.ProviderWechatMinip, idpidentity.ProviderWechatOpen:
		wechatIdentity, err := Wechat(identity)
		if err != nil {
			return loginidentity.ProviderKey{}, err
		}
		if wechatIdentity.Provider == loginidentity.ProviderWechatMinip {
			return loginidentity.NewWechatMinipProviderKey(wechatIdentity.Realm, wechatIdentity.OpenID, wechatIdentity.UnionID)
		}
		return loginidentity.NewWechatOpenProviderKey(wechatIdentity.Realm, wechatIdentity.OpenID, wechatIdentity.UnionID)
	case idpidentity.ProviderWecom:
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

func wechatProvider(provider idpidentity.Provider) (loginidentity.Provider, error) {
	switch provider {
	case idpidentity.ProviderWechatMinip:
		return loginidentity.ProviderWechatMinip, nil
	case idpidentity.ProviderWechatOpen:
		return loginidentity.ProviderWechatOpen, nil
	default:
		return "", fmt.Errorf("external identity provider %q is not wechat", provider)
	}
}
