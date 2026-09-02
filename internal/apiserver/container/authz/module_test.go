package authz

import (
	"testing"

	objectattributeadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAuthzModuleInitializeWithDepsRequiresDB(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}
}

func TestAuthzModuleInitializeWithDepsRequiresEventStager(t *testing.T) {
	module := NewAuthzModule()

	if err := module.InitializeWithDeps(AuthzModuleDeps{DB: &gorm.DB{}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing event stager error")
	}
}

func TestApplicationCapabilitiesExposeConfiguredObjectAttributeAdmissionPolicy(t *testing.T) {
	injected := &configuredObjectAttributePolicy{}
	module := &AuthzModule{objectAttributeAdmissionPolicy: injected}

	capabilities := module.ApplicationCapabilities()
	require.Same(t, injected, capabilities.ObjectAttributeAdmissionPolicy)
}

type configuredObjectAttributePolicy struct{}

func (configuredObjectAttributePolicy) AuthorizeAttribute(objectattributeadmission.Request) error {
	return nil
}
