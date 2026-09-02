package authorization_test

import (
	"context"
	"errors"
	"testing"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
	suggestauthz "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/authorization"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

type stubRouteAuth struct {
	allowPlatformProfiles bool
	allowPlatformMobile   bool
	allowTenantMobile     bool
	err                   error
}

func (s stubRouteAuth) CheckRoutePermission(_ context.Context, _, domain, _, action string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if domain == tenant.PlatformID {
		if action == appquery.ActionList {
			return s.allowPlatformProfiles, nil
		}
		if action == appquery.ActionSearchByMobile {
			return s.allowPlatformMobile, nil
		}
	}
	return action == appquery.ActionSearchByMobile && s.allowTenantMobile, nil
}

var _ authorizationapp.RoutePermissionChecker = stubRouteAuth{}

func TestFactsReaderNilReturnsEmpty(t *testing.T) {
	r := suggestauthz.NewFactsReader(nil)
	facts, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{OperatorID: 100})
	if err != nil {
		t.Fatal(err)
	}
	if facts.PlatformListAllowed || facts.TenantMobileSearchAllowed {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestFactsReaderPlatformProfilePermission(t *testing.T) {
	r := suggestauthz.NewFactsReader(stubRouteAuth{
		allowPlatformProfiles: true,
		allowPlatformMobile:   true,
	})
	facts, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !facts.PlatformListAllowed || !facts.PlatformMobileSearchAllowed {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestFactsReaderTenantMobilePermission(t *testing.T) {
	r := suggestauthz.NewFactsReader(stubRouteAuth{allowTenantMobile: true})
	facts, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.PlatformListAllowed || !facts.TenantMobileSearchAllowed {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestFactsReaderPlainUserNoMobile(t *testing.T) {
	r := suggestauthz.NewFactsReader(stubRouteAuth{})
	facts, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.PlatformListAllowed || facts.TenantMobileSearchAllowed {
		t.Fatalf("facts = %#v", facts)
	}
}

func TestFactsReaderCheckerErrorFails(t *testing.T) {
	wantErr := errors.New("authz down")
	r := suggestauthz.NewFactsReader(stubRouteAuth{err: wantErr})
	_, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestFactsReaderPlatformMobileExclusiveFromTenantMobile(t *testing.T) {
	r := suggestauthz.NewFactsReader(stubRouteAuth{
		allowPlatformProfiles: true,
		allowPlatformMobile:   true,
		allowTenantMobile:     false,
	})
	facts, err := r.ReadAuthorizationFacts(context.Background(), visibility.Principal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !facts.PlatformListAllowed || !facts.PlatformMobileSearchAllowed {
		t.Fatalf("facts = %#v", facts)
	}
}
