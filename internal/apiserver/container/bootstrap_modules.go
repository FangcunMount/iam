package container

import (
	"fmt"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/platform"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/suggest"
)

func (c *Container) initEventing() error {
	eventing, err := platform.InitEventing(platform.EventingDeps{
		DB:          c.mysqlDB,
		EventBus:    c.eventBus,
		CatalogPath: c.runtimeOptions.Events.CatalogPath,
		OutboxBatch: c.runtimeOptions.Events.OutboxRelayBatchSize,
		OutboxRetry: c.runtimeOptions.Events.OutboxRelayRetryDelay,
	})
	if err != nil {
		return err
	}
	c.eventCatalog = eventing.Catalog
	c.eventPublisher = eventing.Publisher
	c.outboxStore = eventing.Outbox
	c.outboxRelay = eventing.Relay
	return nil
}

func (c *Container) initAuthnModule() error {
	authModule := authn.NewAuthnModule()
	if err := authModule.InitializeWithDeps(c.moduleGraph().authnModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize authn module: %w", err)
	}
	c.AuthnModule = authModule
	return nil
}

func (c *Container) initIdentityModule() error {
	identityModule := identity.NewIdentityModule()
	if err := identityModule.InitializeWithDeps(c.moduleGraph().identityModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize identity module: %w", err)
	}
	c.IdentityModule = identityModule
	return nil
}

func (c *Container) initAuthzModule() error {
	authzModule := authz.NewAuthzModule()
	if err := authzModule.InitializeWithDeps(c.moduleGraph().authzModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize authz module: %w", err)
	}
	c.AuthzModule = authzModule
	return nil
}

func (c *Container) initSuggestModule() error {
	suggestModule := suggest.NewSuggestModule()
	if err := suggestModule.InitializeWithDeps(c.moduleGraph().suggestModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize suggest module: %w", err)
	}
	if suggestModule.IsInitialized() {
		c.SuggestModule = suggestModule
	}
	return nil
}

func (c *Container) initIDPModule() error {
	idpModule := idp.NewIDPModule()
	if err := idpModule.InitializeWithDeps(c.moduleGraph().idpModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize idp module: %w", err)
	}
	c.IDPModule = idpModule
	return nil
}

type cacheInspectorProvider interface {
	CacheFamilyInspectors() []cachegovernance.FamilyInspector
}

func (c *Container) initCacheGovernance() {
	inspectors := make([]cachegovernance.FamilyInspector, 0, 8)
	for _, provider := range []cacheInspectorProvider{c.AuthnModule, c.IDPModule} {
		if provider == nil {
			continue
		}
		inspectors = append(inspectors, provider.CacheFamilyInspectors()...)
	}
	c.CacheGovernanceService = cachegovernance.NewReadService(inspectors)
}
