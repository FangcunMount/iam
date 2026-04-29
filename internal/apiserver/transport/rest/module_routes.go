package rest

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/log"
	authnhttp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn"
	authzhttp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz"
	userhttp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity"
	idphttp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/idp"
	suggesthttp "github.com/FangcunMount/iam/internal/apiserver/transport/rest/suggest"
	authnMiddleware "github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

func (r *Router) registerModuleRoutes(engine *gin.Engine, deps routeDependencies, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	r.registerAuthnRoutes(engine, deps)
	r.registerAuthzRoutes(engine, deps.authz, authMiddleware)
	r.registerIDPRoutes(engine, deps)
	r.registerIdentityRoutes(engine, deps.user, authMiddleware)
	r.registerSuggestRoutes(engine, deps.suggest, authMiddleware)
}

func (r *Router) registerAuthnRoutes(engine *gin.Engine, deps routeDependencies) {
	if authnRoutesAvailable(deps.authn) {
		authnDeps := authnhttp.Dependencies{
			AuthHandler:      deps.authn.AuthHandler,
			AccountHandler:   deps.authn.AccountHandler,
			JWKSHandler:      deps.authn.JWKSHandler,
			AdminMiddlewares: deps.adminMiddlewares,
		}
		authnhttp.Register(engine, authnDeps)
		if r.deps.SeedMockAuth.Enabled {
			secret := strings.TrimSpace(r.deps.SeedMockAuth.SharedSecret)
			if secret == "" {
				log.Warn("⚠️  seed_mock_auth.enabled=true but seed_mock_auth.shared_secret is empty; internal mock-consumer route not registered")
			} else {
				authnhttp.RegisterSeedMock(engine, deps.authn.AccountHandler, secret)
				log.Info("✅ Authn seed mock routes registered")
			}
		}
		log.Info("✅ Authn module routes registered")
		return
	}
	log.Warn("⚠️  Authn module not initialized, routes not registered")
}

func (r *Router) registerAuthzRoutes(engine *gin.Engine, deps AuthzDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if authzRoutesAvailable(deps) && authMiddleware != nil {
		authzhttp.Register(engine, authzhttp.Dependencies{
			RoleHandler:       deps.RoleHandler,
			AssignmentHandler: deps.AssignmentHandler,
			PolicyHandler:     deps.PolicyHandler,
			ResourceHandler:   deps.ResourceHandler,
			CheckHandler:      deps.CheckHandler,
			AuthMiddleware:    authMiddleware.AuthRequired(),
		})
		log.Info("✅ Authz module routes registered")
		return
	}
	if authzRoutesAvailable(deps) {
		log.Warn("⚠️  Authz module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  Authz module not initialized, routes not registered")
}

func (r *Router) registerIDPRoutes(engine *gin.Engine, deps routeDependencies) {
	if r.deps.ModuleStatus.IDP {
		idphttp.Register(engine, idphttp.Dependencies{
			WechatAppHandler: deps.idp.WechatAppHandler,
			AdminMiddlewares: deps.adminMiddlewares,
		})
		log.Info("✅ IDP module routes registered")
		return
	}
	log.Warn("⚠️  IDP module not initialized, routes not registered")
}

func (r *Router) registerIdentityRoutes(engine *gin.Engine, deps UserDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if userRoutesAvailable(deps) && authMiddleware != nil {
		userhttp.Register(engine, userhttp.Dependencies{
			UserHandler:         deps.UserHandler,
			ChildHandler:        deps.ChildHandler,
			GuardianshipHandler: deps.GuardianshipHandler,
			AuthMiddleware:      authMiddleware.AuthRequired(),
		})
		log.Info("✅ User module routes registered")
		return
	}
	if userRoutesAvailable(deps) {
		log.Warn("⚠️  User module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  User module not initialized, routes not registered")
}

func (r *Router) registerSuggestRoutes(engine *gin.Engine, deps SuggestDeps, authMiddleware *authnMiddleware.JWTAuthMiddleware) {
	if deps.Service != nil && authMiddleware != nil {
		suggesthttp.Register(engine, suggesthttp.Dependencies{
			Service:        deps.Service,
			AuthMiddleware: authMiddleware.AuthRequired(),
		})
		log.Info("✅ Suggest module routes registered")
		return
	}
	if deps.Service != nil {
		log.Warn("⚠️  Suggest module initialized but JWT middleware unavailable; protected routes not registered")
		return
	}
	log.Warn("⚠️  Suggest module not initialized or disabled, routes not registered")
}

func authnRoutesAvailable(deps AuthnDeps) bool {
	return deps.AuthHandler != nil || deps.AccountHandler != nil || deps.JWKSHandler != nil
}

func authzRoutesAvailable(deps AuthzDeps) bool {
	return deps.RoleHandler != nil || deps.AssignmentHandler != nil || deps.PolicyHandler != nil ||
		deps.ResourceHandler != nil || deps.CheckHandler != nil
}

func userRoutesAvailable(deps UserDeps) bool {
	return deps.UserHandler != nil || deps.ChildHandler != nil || deps.GuardianshipHandler != nil
}
