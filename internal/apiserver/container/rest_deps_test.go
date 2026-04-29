package container

import (
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	resttransport "github.com/FangcunMount/iam/internal/apiserver/transport/rest"
)

func TestBuildRESTDepsConstructsTransportHandlersFromModuleCapabilities(t *testing.T) {
	c := &Container{
		AuthnModule:   &assembler.AuthnModule{},
		AuthzModule:   &assembler.AuthzModule{},
		IDPModule:     &assembler.IDPModule{},
		UserModule:    &assembler.UserModule{},
		SuggestModule: &assembler.SuggestModule{},
	}

	deps := c.BuildRESTDeps(resttransport.RouterOptions{})

	if !deps.ModuleStatus.ContainerInitialized {
		t.Fatalf("container status not set")
	}
	if !deps.ModuleStatus.Authn || !deps.ModuleStatus.Authz || !deps.ModuleStatus.IDP || !deps.ModuleStatus.User {
		t.Fatalf("module status = %#v, want initialized core modules", deps.ModuleStatus)
	}
	if deps.ModuleStatus.Suggest {
		t.Fatalf("suggest status = true, want false when capability service is nil")
	}
	if deps.ModuleStatus.AuthEnabled {
		t.Fatalf("auth enabled = true, want false when token service capability is nil")
	}

	if deps.Authn.AuthHandler == nil || deps.Authn.AccountHandler == nil || deps.Authn.JWKSHandler == nil || deps.Authn.SessionAdminHandler == nil {
		t.Fatalf("authn transport handlers were not constructed: %#v", deps.Authn)
	}
	if deps.Authz.RoleHandler == nil || deps.Authz.AssignmentHandler == nil || deps.Authz.PolicyHandler == nil || deps.Authz.ResourceHandler == nil || deps.Authz.CheckHandler == nil {
		t.Fatalf("authz transport handlers were not constructed: %#v", deps.Authz)
	}
	if deps.IDP.WechatAppHandler == nil {
		t.Fatalf("idp transport handler was not constructed")
	}
	if deps.User.UserHandler == nil || deps.User.ChildHandler == nil || deps.User.GuardianshipHandler == nil {
		t.Fatalf("identity transport handlers were not constructed: %#v", deps.User)
	}
}

func TestBuildRESTDepsHandlesNilContainer(t *testing.T) {
	var c *Container
	deps := c.BuildRESTDeps(resttransport.RouterOptions{})
	if deps.ModuleStatus.ContainerInitialized {
		t.Fatalf("nil container marked initialized")
	}
}
