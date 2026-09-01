package assignmentadmission

import (
	"errors"
	"reflect"
	"testing"
)

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
		Subject:        "user:10001",
		Domain:         "fangcun",
		RoleName:       "qs:staff",
		DelegatedActor: "user:20001",
	}
	if err := authorizer.AuthorizeAssignment(valid); err != nil {
		t.Fatalf("valid request denied: %v", err)
	}

	for name, mutate := range map[string]func(*Request){
		"service": func(r *Request) { r.CallerService = "unknown" },
		"domain":  func(r *Request) { r.Domain = "platform" },
		"subject": func(r *Request) { r.Subject = "service:10001" },
		"role":    func(r *Request) { r.RoleName = "super_admin" },
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
		CallerService: "qs-apiserver.svc", Subject: "user:10", Domain: "fangcun",
		RoleNames: []string{"qs:evaluator"}, DelegatedActor: "user:20",
	})
	if err != nil {
		t.Fatalf("AuthorizeReplacement() error = %v", err)
	}
	want := []string{"qs:evaluator", "qs:staff"}
	if !reflect.DeepEqual(managed, want) {
		t.Fatalf("managed roles = %v, want %v", managed, want)
	}
	managed, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: "user:10", Domain: "fangcun",
		RoleNames: []string{"qs:evaluator"}, DelegatedActor: "service:qs-apiserver.svc",
	})
	if err != nil || !reflect.DeepEqual(managed, want) {
		t.Fatalf("service-authored replacement = %v, %v, want %v", managed, err, want)
	}
	_, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: "user:10", Domain: "fangcun",
		RoleNames: []string{"qs:evaluator"}, DelegatedActor: "service:other.svc",
	})
	var denied *DeniedError
	if !errors.As(err, &denied) || denied.Reason != "delegated_actor_required" {
		t.Fatalf("mismatched service actor error = %v, want delegated_actor_required", err)
	}
	_, err = authorizer.AuthorizeReplacement(ReplacementRequest{
		CallerService: "qs-apiserver.svc", Subject: "user:10", Domain: "fangcun",
		RoleNames: []string{"tenant_admin"}, DelegatedActor: "user:20",
	})
	denied = nil
	if !errors.As(err, &denied) || denied.Reason != "role_not_allowed" {
		t.Fatalf("AuthorizeReplacement() error = %v, want role_not_allowed", err)
	}
}
