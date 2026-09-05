package rest

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	authnhttp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn"
	authzhttp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz"
	userhttp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity"
	idphttp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/idp"
	suggesthttp "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/suggest"
	authnMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
	authzMiddleware "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authz"
)

// registerModuleRoutes 注册模块路由
func (r *Router) registerModuleRoutes(
	engine *gin.Engine,
	deps routeDependencies,
	authMiddleware *authnMiddleware.JWTAuthMiddleware,
	authorizationMiddleware *authzMiddleware.Middleware,
) {
	// 注册 AuthN 模块 认证路由
	r.registerAuthnRoutes(engine, deps, authMiddleware, authorizationMiddleware)
	// 注册 AuthZ 模块 授权路由
	r.registerAuthzRoutes(engine, deps.authz, authMiddleware, authorizationMiddleware)
	// 注册 IDP 模块 IDP路由
	r.registerIDPRoutes(engine, deps, authMiddleware, authorizationMiddleware)
	// 注册 Identity 模块 身份路由
	r.registerIdentityRoutes(engine, deps.user, authMiddleware)
	// 注册 Suggest 模块 建议路由
	r.registerSuggestRoutes(engine, deps.suggest, authMiddleware, authorizationMiddleware)
}

// registerAuthnRoutes 注册 AuthN 模块 认证路由
func (r *Router) registerAuthnRoutes(engine *gin.Engine, deps routeDependencies, authMiddleware *authnMiddleware.JWTAuthMiddleware, authorizationMiddleware *authzMiddleware.Middleware) {
	if r.deps.ModuleStatus.authnAvailable() && authnRoutesAvailable(deps.authn) {
		var authRequired gin.HandlerFunc
		if authMiddleware != nil {
			authRequired = authMiddleware.AuthRequired()
		}
		var permissionOrGlobal func(resource, action string) gin.HandlerFunc
		if authorizationMiddleware != nil {
			permissionOrGlobal = authorizationMiddleware.RequirePermissionOrGlobal
		}
		authnDeps := authnhttp.Dependencies{
			AuthHandler:            deps.authn.AuthHandler,
			OnboardingHandler:      deps.authn.OnboardingHandler,
			LoginIdentityHandler:   deps.authn.LoginIdentityHandler,
			WechatOpenLoginHandler: deps.authn.WechatOpenLoginHandler,
			JWKSHandler:            deps.authn.JWKSHandler,
			AuthMiddleware:         authRequired,
			PermissionOrGlobal:     permissionOrGlobal,
		}
		authnhttp.Register(engine, authnDeps)
		if r.deps.SeedMockAuth.Enabled {
			secret := strings.TrimSpace(r.deps.SeedMockAuth.SharedSecret)
			if secret == "" {
				log.Warn("⚠️  seed_mock_auth.enabled=true but seed_mock_auth.shared_secret is empty; internal mock-consumer route not registered")
			} else {
				authnhttp.RegisterSeedMock(engine, deps.authn.OnboardingHandler, secret)
				log.Info("✅ Authn seed mock routes registered")
			}
		}
		log.Info("✅ Authn module routes registered")
		return
	}
	log.Warn("⚠️  Authn module not initialized, routes not registered")
}

// registerAuthzRoutes 注册 AuthZ 模块 授权路由
func (r *Router) registerAuthzRoutes(engine *gin.Engine, deps AuthzDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware, authorizationMiddleware *authzMiddleware.Middleware) {
	if r.deps.ModuleStatus.authzAvailable() && authzRoutesAvailable(deps) && authMiddleware != nil && authorizationMiddleware != nil {
		authzhttp.Register(engine, authzhttp.Dependencies{
			RoleHandler:            deps.RoleHandler,
			AssignmentHandler:      deps.AssignmentHandler,
			PermissionGrantHandler: deps.PermissionGrantHandler,
			RoleInheritanceHandler: deps.RoleInheritanceHandler,
			ResourceHandler:        deps.ResourceHandler,
			AuthMiddleware:         authMiddleware.AuthRequired(),
			PermissionOrGlobal:     authorizationMiddleware.RequirePermissionOrGlobal,
			PlatformPermission:     authorizationMiddleware.RequirePlatformPermission,
		})
		log.Info("✅ Authz module routes registered")
		return
	}
	if r.deps.ModuleStatus.authzAvailable() && authzRoutesAvailable(deps) {
		log.Warn("⚠️  Authz module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  Authz module not initialized, routes not registered")
}

// registerIDPRoutes 注册 IDP 模块 IDP路由
func (r *Router) registerIDPRoutes(engine *gin.Engine, deps routeDependencies, authMiddleware *authnMiddleware.JWTAuthMiddleware, authorizationMiddleware *authzMiddleware.Middleware) {
	if r.deps.ModuleStatus.idpAvailable() && deps.idp.WechatAppHandler != nil && authMiddleware != nil && authorizationMiddleware != nil {
		idphttp.Register(engine, idphttp.Dependencies{
			WechatAppHandler:   deps.idp.WechatAppHandler,
			AuthMiddleware:     authMiddleware.AuthRequired(),
			PermissionOrGlobal: authorizationMiddleware.RequirePermissionOrGlobal,
		})
		log.Info("✅ IDP module routes registered")
		return
	}
	log.Warn("⚠️  IDP module not initialized, routes not registered")
}

// registerIdentityRoutes 注册 Identity 模块 身份路由
func (r *Router) registerIdentityRoutes(engine *gin.Engine, deps UserDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if r.deps.ModuleStatus.identityAvailable() && userRoutesAvailable(deps) && authMiddleware != nil {
		userhttp.Register(engine, userhttp.Dependencies{
			UserHandler:        deps.UserHandler,
			ProfileHandler:     deps.ProfileHandler,
			ProfileLinkHandler: deps.ProfileLinkHandler,
			AuthMiddleware:     authMiddleware.AuthRequired(),
		})
		log.Info("✅ User module routes registered")
		return
	}
	if r.deps.ModuleStatus.identityAvailable() && userRoutesAvailable(deps) {
		log.Warn("⚠️  User module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  User module not initialized, routes not registered")
}

// registerSuggestRoutes 注册 Suggest 模块 建议路由
func (r *Router) registerSuggestRoutes(engine *gin.Engine, deps SuggestDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware, authorizationMiddleware *authzMiddleware.Middleware) {
	if r.deps.ModuleStatus.suggestAvailable() && deps.Querier != nil && authMiddleware != nil {
		middlewares := []gin.HandlerFunc{authMiddleware.AuthRequired()}
		if r.deps.ModuleStatus.authzAvailable() && authorizationMiddleware != nil {
			middlewares = append(middlewares, authorizationMiddleware.RequirePermission(
				appquery.ResourceIAMProfileCollection,
				appquery.ActionSearch,
			))
		}
		suggesthttp.Register(engine, suggesthttp.Dependencies{
			Querier:     deps.Querier,
			Middlewares: middlewares,
			Metrics:     deps.Metrics,
			RateLimiter: deps.RateLimiter,
		})
		log.Info("✅ Suggest module routes registered")
		return
	}
	if r.deps.ModuleStatus.suggestAvailable() && deps.Querier != nil {
		log.Warn("⚠️  Suggest module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  Suggest module not initialized or disabled, routes not registered")
}

// authnRoutesAvailable 认证路由是否可用
func authnRoutesAvailable(deps AuthnDeps) bool {
	return deps.AuthHandler != nil || deps.OnboardingHandler != nil || deps.LoginIdentityHandler != nil || deps.JWKSHandler != nil
}

// authzRoutesAvailable 授权路由是否可用
func authzRoutesAvailable(deps AuthzDeps) bool {
	return deps.RoleHandler != nil || deps.AssignmentHandler != nil || deps.PermissionGrantHandler != nil ||
		deps.RoleInheritanceHandler != nil || deps.ResourceHandler != nil
}

// userRoutesAvailable 用户路由是否可用
func userRoutesAvailable(deps UserDeps) bool {
	return deps.UserHandler != nil || deps.ProfileHandler != nil || deps.ProfileLinkHandler != nil
}
