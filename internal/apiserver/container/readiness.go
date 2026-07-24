package container

import (
	"context"
	"errors"
	"fmt"
	"time"

	readinessapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/readiness"
	readinessmetrics "github.com/FangcunMount/iam/v2/internal/apiserver/infra/observability/readiness"
)

func (c *Container) ReadinessChecker() *readinessapp.Checker {
	if c == nil {
		return nil
	}
	if c.readiness != nil {
		return c.readiness
	}
	config := c.runtimeOptions.Health.Readiness
	c.readiness = readinessapp.New(readinessapp.Config{
		ComponentTimeout: config.ComponentTimeout,
		TotalTimeout:     config.TotalTimeout,
	}, []readinessapp.Component{
		{Name: "mysql", Required: true, Check: c.checkMySQLReady},
		{Name: "redis", Required: true, Check: c.checkRedisReady},
		{Name: "authn", Required: true, Check: c.checkAuthnReady},
		{Name: "jwks", Required: true, Check: c.checkJWKSReady},
		{Name: "authz", Required: true, Check: c.checkAuthzReady},
		{Name: "suggest", Required: c.runtimeOptions.Suggest.Required, Check: c.checkSuggestReady},
		{Name: "session_revocation", Required: true, Check: c.checkSessionRevocationReady},
	}, readinessmetrics.Recorder{})
	return c.readiness
}

func (c *Container) MarkDraining() {
	if checker := c.ReadinessChecker(); checker != nil {
		checker.MarkDraining()
	}
}

func (c *Container) DrainDelay() time.Duration {
	if c == nil {
		return 0
	}
	return c.runtimeOptions.Health.Readiness.DrainDelay
}

func (c *Container) checkMySQLReady(ctx context.Context) error {
	if c.mysqlDB == nil {
		return errors.New("mysql unavailable")
	}
	sqlDB, err := c.mysqlDB.DB()
	if err != nil {
		return errors.New("mysql unavailable")
	}
	return sqlDB.PingContext(ctx)
}

func (c *Container) checkRedisReady(ctx context.Context) error {
	if c.redisClient == nil {
		return errors.New("redis unavailable")
	}
	return c.redisClient.Ping(ctx).Err()
}

func (c *Container) checkAuthnReady(context.Context) error {
	if !c.initialized || !c.ModuleState(moduleAuthn).Available || c.AuthnModule == nil {
		return errors.New("authn unavailable")
	}
	if c.AuthnModule.ApplicationCapabilities().TokenService == nil {
		return errors.New("authn token service unavailable")
	}
	return nil
}

func (c *Container) checkJWKSReady(ctx context.Context) error {
	if c.AuthnModule == nil {
		return errors.New("jwks unavailable")
	}
	service := c.AuthnModule.ApplicationCapabilities().KeyManagementApp
	if service == nil {
		return errors.New("jwks unavailable")
	}
	active, err := service.GetActiveKey(ctx)
	if err != nil || active == nil {
		return errors.New("jwks active key unavailable")
	}
	return nil
}

func (c *Container) checkAuthzReady(context.Context) error {
	if !c.initialized || !c.ModuleState(moduleAuthz).Available || c.AuthzModule == nil {
		return errors.New("authz unavailable")
	}
	reporter := c.AuthzModule.ApplicationCapabilities().RuntimeHealth
	if reporter == nil {
		return errors.New("authz runtime unavailable")
	}
	healthy, _, _ := reporter.ReloadHealth()
	if !healthy {
		return errors.New("authz runtime unhealthy")
	}
	return nil
}

func (c *Container) checkSuggestReady(context.Context) error {
	if !c.runtimeOptions.Suggest.Enable {
		return nil
	}
	if c.SuggestModule == nil {
		return errors.New("suggest unavailable")
	}
	return c.SuggestModule.CheckHealth()
}

func (c *Container) checkSessionRevocationReady(ctx context.Context) error {
	if c.IdentityModule == nil || c.IdentityModule.SessionRevocationStore() == nil {
		return errors.New("session revocation unavailable")
	}
	age, err := c.IdentityModule.SessionRevocationStore().OldestUnfinishedAge(ctx, time.Now().UTC())
	if err != nil {
		return errors.New("session revocation unavailable")
	}
	if age > c.runtimeOptions.Health.Readiness.OutboxMaxPendingAge {
		return fmt.Errorf("session revocation backlog exceeded")
	}
	return nil
}
