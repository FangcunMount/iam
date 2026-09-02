package assignmentadmission

import (
	"errors"
	"reflect"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
)

func mustSubject(t *testing.T, value string) subject.Ref {
	t.Helper()
	ref, err := subject.ParseRef(value)
	if err != nil {
		t.Fatalf("subject.ParseRef(%q) error = %v", value, err)
	}
	return ref
}

func mustTenant(t *testing.T, value string) tenant.ID {
	t.Helper()
	id, err := tenant.NewID(value)
	if err != nil {
		t.Fatalf("tenant.NewID(%q) error = %v", value, err)
	}
	return id
}

func mustRoleName(t *testing.T, value string) role.Name {
	t.Helper()
	name, err := role.NewName(value)
	if err != nil {
		t.Fatalf("role.NewName(%q) error = %v", value, err)
	}
	return name
}

func TestAuthorizerEnforcesServiceConstraints(t *testing.T) {
	authorizer, err := New(Config{
		DefaultPolicy: "deny",
		Services: map[string]ServiceConstraint{
			"qs-apiserver.svc": {
				Domains:                      []string{"fangcun"},
				SubjectTypes:                 []string{"user"},
				Roles:                        []string{"qs:staff"},
				RequireDelegatedActorOnGrant: true,
			},
			"admin": {AllowAll: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	valid := Request{
		CallerService:  "qs-apiserver.svc",
		Operation:      OperationGrant,
		Subject:        mustSubject(t, "user:10001"),
		Domain:         mustTenant(t, "fangcun"),
		RoleName:       mustRoleName(t, "qs:staff"),
		DelegatedActor: "user:20001",
	}
	if err := authorizer.AuthorizeAssignment(valid); err != nil {
		t.Fatalf("valid request denied: %v", err)
	}

	for name, mutate := range map[string]func(*Request){
		"service": func(r *Request) { r.CallerService = "unknown" },
		"domain":  func(r *Request) { r.Domain = mustTenant(t, "platform") },
		"subject": func(r *Request) { r.Subject = mustSubject(t, "service:10001") },
		"role":    func(r *Request) { r.RoleName = mustRoleName(t, "super_admin") },
		"actor":   func(r *Request) { r.DelegatedActor = "" },
	} {
		t.Run(name, func(t *testing.T) {
			request := valid
			mutate(&request)
			var denied *DeniedError
			if err := authorizer.AuthorizeAssignment(request); !errors.As(err, &denied) {
				t.Fatalf("AuthorizeAssignment() error = %v, want DeniedError", err)
			}
		})
	}
	if err := authorizer.AuthorizeAssignment(Request{CallerService: "admin"}); err != nil {
		t.Fatalf("admin allow-all denied: %v", err)
	}
}

func TestValidateRejectsNonAdminAllowAll(t *testing.T) {
	err := Validate(Config{
		DefaultPolicy: "deny",
		Services: map[string]ServiceConstraint{
			"qs-apiserver.svc": {AllowAll: true},
		},
	})
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
}

func TestAuthorizeReplacementReturnsEntireManagedSetAndRejectsEscalation(t *testing.T) {
	authorizer, err := New(Config{
		DefaultPolicy: "deny",
		Services: map[string]ServiceConstraint{
			"qs-apiserver.svc": {
				Domains: []string{"fangcun"}, SubjectTypes: []string{"user"},
				Roles:                        []string{"qs:staff", "qs:evaluator", "qs:staff"},
				RequireDelegatedActorOnGrant: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	managed, err := authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: mustSubject(t, "user:10"), Domain: mustTenant(t, "fangcun"),
		RoleNames: []role.Name{mustRoleName(t, "qs:evaluator")}, DelegatedActor: "user:20",
	})
	if err != nil {
		t.Fatalf("AuthorizeReplacement() error = %v", err)
	}
	want := []string{"qs:evaluator", "qs:staff"}
	if !reflect.DeepEqual(managed, want) {
		t.Fatalf("managed roles = %v, want %v", managed, want)
	}
	managed, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: mustSubject(t, "user:10"), Domain: mustTenant(t, "fangcun"),
		RoleNames: []role.Name{mustRoleName(t, "qs:evaluator")}, DelegatedActor: "service:qs-apiserver.svc",
	})
	if err != nil || !reflect.DeepEqual(managed, want) {
		t.Fatalf("service-authored replacement = %v, %v, want %v", managed, err, want)
	}
	_, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: mustSubject(t, "user:10"), Domain: mustTenant(t, "fangcun"),
		RoleNames: []role.Name{mustRoleName(t, "qs:evaluator")}, DelegatedActor: "service:other.svc",
	})
	var denied *DeniedError
	if !errors.As(err, &denied) || denied.Reason != "delegated_actor_required" {
		t.Fatalf("mismatched service actor error = %v, want delegated_actor_required", err)
	}
	_, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: mustSubject(t, "user:10"), Domain: mustTenant(t, "fangcun"),
		RoleNames: []role.Name{mustRoleName(t, "tenant_admin")}, DelegatedActor: "user:20",
	})
	denied = nil
	if !errors.As(err, &denied) || denied.Reason != "role_not_allowed" {
		t.Fatalf("AuthorizeReplacement() error = %v, want role_not_allowed", err)
	}
}
