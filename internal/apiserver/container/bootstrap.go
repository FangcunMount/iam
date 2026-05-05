package container

import (
	"fmt"

	"github.com/FangcunMount/component-base/pkg/log"
)

type bootstrapStep struct {
	name string
	run  func() error
}

func (c *Container) bootstrapPlan() []bootstrapStep {
	return []bootstrapStep{
		{name: moduleEventRuntime, run: c.initEventing},
		{name: moduleIDP, run: c.initIDPModule},
		{name: moduleAuthn, run: c.initAuthModule},
		{name: moduleAuthz, run: c.initAuthzModule},
		{name: moduleUser, run: c.initUserModule},
		{name: moduleSuggest, run: c.initSuggestModule},
		{name: moduleCacheGovernance, run: func() error {
			c.initCacheGovernance()
			return nil
		}},
	}
}

func (c *Container) runBootstrapPlan() []error {
	errors := make([]error, 0)
	for _, step := range c.bootstrapPlan() {
		if step.run == nil {
			continue
		}
		if err := step.run(); err != nil {
			log.Warnf("Failed to initialize %s: %v", step.name, err)
			c.recordBootstrapFailure(step.name, err)
			errors = append(errors, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	return errors
}

func (c *Container) logBootstrapStatus() {
	log.Infof("🏗️  Container initialization completed:")
	if c.IDPModule != nil {
		log.Info("   ✅ IDP module")
	} else {
		log.Warn("   ❌ IDP module failed")
	}
	if c.AuthnModule != nil {
		log.Info("   ✅ Authn module")
	} else {
		log.Warn("   ❌ Authn module failed")
	}
	if c.UserModule != nil {
		log.Info("   ✅ User module")
	} else {
		log.Warn("   ❌ User module failed")
	}
	if c.AuthzModule != nil {
		log.Info("   ✅ Authz module")
	} else {
		log.Warn("   ❌ Authz module failed")
	}
	if c.SuggestModule != nil && c.SuggestModule.IsInitialized() {
		log.Info("   ✅ Suggest module")
	} else {
		log.Warn("   ⚠️  Suggest module not initialized or disabled")
	}
	if c.outboxStore != nil {
		log.Info("   ✅ Event outbox")
	} else {
		log.Warn("   ⚠️  Event outbox not initialized")
	}
}
