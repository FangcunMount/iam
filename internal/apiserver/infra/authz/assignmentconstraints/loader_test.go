package assignmentconstraints

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadWithACLValidatesAssignmentMutationCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		constraints string
		acl         string
		wantErr     string
	}{
		{
			name: "matching coverage",
			constraints: `
default_policy: deny
services:
  qs-apiserver.svc:
    domains: [fangcun]
    subject_types: [user]
    roles: [qs:admin]
  admin:
    allow_all: true
`,
			acl: `
default_policy: deny
services:
  - service_name: qs-apiserver.svc
    enabled: true
    allowed_methods:
      - /iam.authz.v3.AuthorizationService/GrantAssignment
      - /iam.authz.v3.AuthorizationService/RevokeAssignment
  - service_name: admin
    enabled: true
    allowed_methods:
      - /iam.authz.v3.AuthorizationService/*
`,
		},
		{
			name: "acl capability without constraint",
			constraints: `
default_policy: deny
services:
  admin:
    allow_all: true
`,
			acl: `
default_policy: deny
services:
  - service_name: admin
    enabled: true
    allowed_methods: [/iam.authz.v3.AuthorizationService/*]
  - service_name: unbounded.svc
    enabled: true
    allowed_methods: [/iam.authz.v3.AuthorizationService/GrantAssignment]
`,
			wantErr: "has no request constraint",
		},
		{
			name: "constraint cannot expand acl",
			constraints: `
default_policy: deny
services:
  qs-apiserver.svc:
    domains: [fangcun]
    subject_types: [user]
    roles: [qs:admin]
`,
			acl: `
default_policy: deny
services:
  - service_name: qs-apiserver.svc
    enabled: true
    allowed_methods: [/iam.authz.v3.AuthorizationService/Check]
`,
			wantErr: "is not allowed to mutate assignments",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			constraintsPath := filepath.Join(dir, "constraints.yaml")
			aclPath := filepath.Join(dir, "acl.yaml")
			require.NoError(t, os.WriteFile(constraintsPath, []byte(tt.constraints), 0o600))
			require.NoError(t, os.WriteFile(aclPath, []byte(tt.acl), 0o600))

			_, err := LoadWithACL(constraintsPath, aclPath)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
