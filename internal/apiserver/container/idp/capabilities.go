package idp

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"
	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

// ApplicationCapabilities contains IDP application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	WechatAppService           wechatapp.WechatAppApplicationService
	WechatAppCredentialService wechatapp.WechatAppCredentialApplicationService
	WechatAppTokenService      wechatapp.WechatAppTokenApplicationService
	WechatAppRepository        wechatappDomain.Repository
	SecretVault                wechatappDomain.SecretVault
}
