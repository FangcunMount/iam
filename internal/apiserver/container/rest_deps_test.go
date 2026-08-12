package container

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/suggest"
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
)

func TestBuildRESTDepsConstructsTransportHandlersFromModuleCapabilities(t *testing.T) {
	c := &Container{
		AuthnModule:    &authn.AuthnModule{},
		AuthzModule:    &authz.AuthzModule{},
		IDPModule:      &idp.IDPModule{},
		IdentityModule: &identity.IdentityModule{},
		SuggestModule:  &suggest.SuggestModule{},
		initialized:    true,
	}

	deps := c.BuildRESTDeps(resttransport.RouterOptions{})

	if !deps.ModuleStatus.Container.Bootstrapped || !deps.ModuleStatus.Container.Available {
		t.Fatalf("container module state = %#v, want bootstrapped and available", deps.ModuleStatus.Container)
	}
	if deps.Authn.AuthHandler == nil || deps.Authn.OnboardingHandler == nil || deps.Authn.JWKSHandler == nil || deps.Authn.SessionAdminHandler == nil {
		t.Fatalf("authn transport handlers were not constructed: %#v", deps.Authn)
	}
	if deps.Authz.RoleHandler == nil || deps.Authz.RoleBindingHandler == nil || deps.Authz.PolicyHandler == nil || deps.Authz.ResourceHandler == nil || deps.Authz.CheckHandler == nil {
		t.Fatalf("authz transport handlers were not constructed: %#v", deps.Authz)
	}
	if deps.IDP.WechatAppHandler == nil {
		t.Fatalf("idp transport handler was not constructed")
	}
	if deps.User.UserHandler == nil || deps.User.ProfileHandler == nil || deps.User.ProfileLinkHandler == nil {
		t.Fatalf("identity transport handlers were not constructed: %#v", deps.User)
	}
	identityState := deps.ModuleStatus.Modules["identity module"]
	if !identityState.Bootstrapped || !identityState.Available {
		t.Fatalf("identity module state = %#v, want bootstrapped and available", identityState)
	}
}

func TestBuildRESTDepsExposesModuleDegradedReasons(t *testing.T) {
	c := &Container{
		initialized: true,
		bootstrapErrors: map[string]string{
			moduleAuthn: "invalid redis parameter",
		},
	}

	deps := c.BuildRESTDeps(resttransport.RouterOptions{})

	state := deps.ModuleStatus.Modules[moduleAuthn]
	if !state.Bootstrapped {
		t.Fatalf("authn state = %#v, want bootstrapped", state)
	}
	if state.Available {
		t.Fatalf("authn state = %#v, want unavailable", state)
	}
	if state.DegradedReason != "invalid redis parameter" {
		t.Fatalf("authn degraded reason = %q", state.DegradedReason)
	}
}

func TestBuildRESTDepsUsesModuleStateAvailability(t *testing.T) {
	c := &Container{
		AuthnModule: &authn.AuthnModule{},
		initialized: false,
	}

	deps := c.BuildRESTDeps(resttransport.RouterOptions{})

	if deps.ModuleStatus.Container.Bootstrapped {
		t.Fatalf("container bootstrapped = true, want false")
	}
	if deps.ModuleStatus.Modules[moduleAuthn].Available {
		t.Fatalf("authn module state = %#v, want unavailable before bootstrap", deps.ModuleStatus.Modules[moduleAuthn])
	}
	if deps.Authn.AuthHandler != nil || deps.Authn.OnboardingHandler != nil || deps.Authn.JWKSHandler != nil {
		t.Fatalf("authn handlers were constructed before module availability: %#v", deps.Authn)
	}
}

func TestBuildRESTDepsHandlesNilContainer(t *testing.T) {
	var c *Container
	deps := c.BuildRESTDeps(resttransport.RouterOptions{})
	if deps.ModuleStatus.Container.Bootstrapped {
		t.Fatalf("nil container marked initialized")
	}
}
