package handler

import (
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	_ "github.com/FangcunMount/iam/v2/pkg/core" // imported for swagger
)

// JWKSHandler JWKS HTTP 处理器
type JWKSHandler struct {
	*BaseHandler
	keyManagementApp *jwksApp.KeyManagementAppService
	keyLifecycleApp  *jwksApp.KeyLifecycleAppService
	keyPublishApp    *jwksApp.KeyPublishAppService
}

// NewJWKSHandler 创建 JWKS 处理器
func NewJWKSHandler(
	keyManagementApp *jwksApp.KeyManagementAppService,
	keyLifecycleApp *jwksApp.KeyLifecycleAppService,
	keyPublishApp *jwksApp.KeyPublishAppService,
) *JWKSHandler {
	return &JWKSHandler{
		BaseHandler:      NewBaseHandler(),
		keyManagementApp: keyManagementApp,
		keyLifecycleApp:  keyLifecycleApp,
		keyPublishApp:    keyPublishApp,
	}
}
