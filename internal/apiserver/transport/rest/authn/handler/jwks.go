package handler

import (
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	signingkeyApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signingkey"
	_ "github.com/FangcunMount/iam/v3/pkg/core" // imported for swagger
)

// JWKSHandler JWKS HTTP 处理器
type JWKSHandler struct {
	*BaseHandler
	keyManagementApp *signingkeyApp.KeyManagementAppService
	keyLifecycleApp  *signingkeyApp.KeyLifecycleAppService
	keyPublishApp    *jwksApp.KeyPublishAppService
}

// NewJWKSHandler 创建 JWKS 处理器
func NewJWKSHandler(
	keyManagementApp *signingkeyApp.KeyManagementAppService,
	keyLifecycleApp *signingkeyApp.KeyLifecycleAppService,
	keyPublishApp *jwksApp.KeyPublishAppService,
) *JWKSHandler {
	return &JWKSHandler{
		BaseHandler:      NewBaseHandler(),
		keyManagementApp: keyManagementApp,
		keyLifecycleApp:  keyLifecycleApp,
		keyPublishApp:    keyPublishApp,
	}
}
