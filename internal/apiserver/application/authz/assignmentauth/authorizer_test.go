package assignmentauth

import (
	"errors"
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
