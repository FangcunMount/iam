package handler

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	binding "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRoleHandlerToRoleResponse(t *testing.T) {
	handler := &RoleHandler{}
	source, err := role.NewRole(
		"admin",
		"Administrator",
		"tenant-a",
		role.WithID(meta.FromUint64(11)),
		role.WithDescription("full access"),
	)
	require.NoError(t, err)

	resp := handler.toRoleResponse(&source)

	if resp.ID.Uint64() != 11 ||
		resp.Name != "admin" ||
		resp.DisplayName != "Administrator" ||
		resp.TenantID != "tenant-a" ||
		resp.Description != "full access" {
		t.Fatalf("unexpected role response: %#v", resp)
	}
}

func TestRoleBindingHandlerToBindingResponse(t *testing.T) {
	handler := &RoleBindingHandler{}
	source, err := binding.NewBinding(
		binding.SubjectTypeUser,
		meta.FromUint64(1),
		meta.FromUint64(22),
		"tenant-a",
		binding.WithID(binding.NewBindingID(12)),
		binding.WithGrantedBy("operator-1"),
	)
	require.NoError(t, err)

	resp := handler.toAssignmentResponse(&source)

	if resp.ID.Uint64() != 12 ||
		resp.SubjectType != "user" ||
		resp.SubjectID != meta.FromUint64(1) ||
		resp.RoleID.Uint64() != 22 ||
		resp.TenantID != "tenant-a" ||
		resp.GrantedBy != "operator-1" {
		t.Fatalf("unexpected binding response: %#v", resp)
	}
}

func TestConvertToSubjectType(t *testing.T) {
	subjectType, err := convertToSubjectType("user")
	if err != nil {
		t.Fatalf("convert user subject type: %v", err)
	}
	if subjectType != binding.SubjectTypeUser {
		t.Fatalf("unexpected subject type: %s", subjectType)
	}

	if _, err := convertToSubjectType("service"); err == nil {
		t.Fatal("expected unsupported subject type error")
	}
}

func TestResourceHandlerToResourceResponse(t *testing.T) {
	handler := &ResourceHandler{}
	source, err := resource.NewResource(
		"scale:form:template:*",
		[]string{"read", "write"},
		resource.WithID(resource.NewResourceID(13)),
		resource.WithDisplayName("Form"),
		resource.WithAppName("scale"),
		resource.WithDomain("form"),
		resource.WithType("template"),
		resource.WithDescription("form resource"),
	)
	require.NoError(t, err)

	resp := handler.toResourceResponse(&source)

	if resp.ID.Uint64() != 13 ||
		resp.Key != "scale:form:template:*" ||
		resp.DisplayName != "Form" ||
		resp.AppName != "scale" ||
		resp.Domain != "form" ||
		resp.Type != "template" ||
		resp.Description != "form resource" ||
		len(resp.Actions) != 2 ||
		resp.Actions[0] != "read" ||
		resp.Actions[1] != "write" {
		t.Fatalf("unexpected resource response: %#v", resp)
	}
}

func TestPolicyRuleAndVersionResponses(t *testing.T) {
	perm, err := permission.New("admin", "tenant-a", "scale:form:template:*", "read")
	if err != nil {
		t.Fatalf("new permission: %v", err)
	}

	ruleResponses := toPermissionResponses([]permission.Permission{perm})
	if len(ruleResponses) != 1 ||
		ruleResponses[0].Subject != "role:admin" ||
		ruleResponses[0].Domain != "tenant-a" ||
		ruleResponses[0].Object != "scale:form:template:*" ||
		ruleResponses[0].Action != "read" ||
		ruleResponses[0].ScopeType != "all" ||
		ruleResponses[0].ScopeValue != "*" {
		t.Fatalf("unexpected permission responses: %#v", ruleResponses)
	}

	empty := emptyPolicyVersionResponse("tenant-a")
	if empty.TenantID != "tenant-a" || empty.Version != 0 || empty.ChangedBy != "" || empty.Reason != "" {
		t.Fatalf("unexpected empty policy version response: %#v", empty)
	}

	version := policy.NewPolicyVersion(
		"tenant-a",
		3,
		policy.WithChangedBy("operator-1"),
		policy.WithReason("test"),
	)
	resp := toPolicyVersionResponse(&version)
	if resp.TenantID != "tenant-a" || resp.Version != 3 || resp.ChangedBy != "operator-1" || resp.Reason != "test" {
		t.Fatalf("unexpected policy version response: %#v", resp)
	}
}
