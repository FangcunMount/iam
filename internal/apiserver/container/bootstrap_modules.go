package container

import (
	"fmt"
	"net/http"
	"time"

	cachegovernance "github.com/FangcunMount/iam/v5/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/suggest"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messagefailure"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/nsqio/go-nsq"
)

func (c *Container) initEventing() error {
	eventing, err := platform.InitEventing(platform.EventingDeps{
		DB:                c.mysqlDB,
		ReliableMessaging: c.runtimeOptions.Events.ReliableMessaging,
		NSQEnabled:        c.runtimeOptions.NSQEnabled,
		NSQAddress:        c.runtimeOptions.NSQAddress,
		OutboxInterval:    c.runtimeOptions.Events.OutboxRelayInterval,
		CatalogPath:       c.runtimeOptions.Events.CatalogPath,
		OutboxBatch:       c.runtimeOptions.Events.OutboxRelayBatchSize,
		OutboxRetry:       c.runtimeOptions.Events.OutboxRelayRetryDelay,
	})
	if err != nil {
		return err
	}
	c.eventCatalog = eventing.Catalog
	c.eventPublisher = eventing.Publisher
	c.outboxStore = eventing.Outbox
	c.eventStager = eventing.Stager
	c.reliableRuntime = eventing.ReliableRuntime
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
	if !c.runtimeOptions.NSQConsumerSDKEnabled {
		return fmt.Errorf("legacy policy subscriber is retired; enable nsq.consumer-sdk-enabled")
	}
	if err := c.initSDKPolicySync(authzModule); err != nil {
		return fmt.Errorf("initialize SDK policy subscriber: %w", err)
	}
	return nil
}

func (c *Container) initSDKPolicySync(module *authz.AuthzModule) error {
	if !c.runtimeOptions.NSQEnabled || c.mysqlDB == nil {
		return fmt.Errorf("SDK policy subscriber requires NSQ and IAM MySQL")
	}
	// A missing audit table cannot be discovered only after terminal failure:
	// reject this cutover while the old consumer is still the selected path.
	if err := c.mysqlDB.Exec("SELECT id FROM iam_nsq_failure_audit LIMIT 0").Error; err != nil {
		return fmt.Errorf("schema 40 terminal failure audit required: %w", err)
	}
	sqlDB, err := c.mysqlDB.DB()
	if err != nil {
		return err
	}
	audit, err := messagefailure.New(sqlDB)
	if err != nil {
		return err
	}
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 7 * time.Second}, c.runtimeOptions.NSQDHTTPAddresses)
	if err != nil {
		return err
	}
	driver := nsq.NewConfig()
	driver.DialTimeout = 5 * time.Second
	driver.ReadTimeout = 60 * time.Second
	driver.WriteTimeout = 5 * time.Second
	config := sdknsq.SubscriberConfig{
		Driver: driver, MaxAttempts: c.runtimeOptions.NSQMaxAttempts,
		MaxInFlight:        c.runtimeOptions.NSQMaxInFlight,
		FailedHandoffGroup: authz.ChannelPrefix,
		Retry:              sdknsq.Backoff{BaseDelay: time.Duration(c.runtimeOptions.NSQRequeueDelaySeconds) * time.Second, MaxDelay: 5 * time.Minute},
	}
	if len(c.runtimeOptions.NSQLookupdAddresses) > 0 {
		config.LookupdAddresses = c.runtimeOptions.NSQLookupdAddresses
	} else {
		config.NSQDAddresses = []string{c.runtimeOptions.NSQAddress}
	}
	subscriber, err := sdknsq.NewSubscriber(config)
	if err != nil {
		return err
	}
	c.sdkPolicySync = module.SDKPolicySyncSubscriber(subscriber, provisioner, audit.Record)
	if c.sdkPolicySync == nil {
		return fmt.Errorf("SDK policy subscriber dependencies unavailable")
	}
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
	inspectors := make([]cachegovernance.FamilyInspector, 0, 12)
	for _, provider := range []cacheInspectorProvider{c.AuthnModule, c.IDPModule, c.SuggestModule} {
		if provider == nil {
			continue
		}
		inspectors = append(inspectors, provider.CacheFamilyInspectors()...)
	}
	c.CacheGovernanceService = cachegovernance.NewReadService(inspectors)
}
