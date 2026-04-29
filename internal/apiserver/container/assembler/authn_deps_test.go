package assembler

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/event"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestAuthnModuleDepsPreserveTypedDependencies(t *testing.T) {
	db := &gorm.DB{}
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	hasher := authnHasherStub{}
	publisher := authnPublisherStub{}
	idp := &IDPModule{}

	deps := AuthnModuleDeps{
		DB:             db,
		RedisClient:    redisClient,
		PasswordHasher: hasher,
		IDPModule:      idp,
		EventPublisher: publisher,
	}
	if deps.DB != db {
		t.Fatalf("DB dependency was not preserved")
	}
	if deps.RedisClient != redisClient {
		t.Fatalf("RedisClient dependency was not preserved")
	}
	if deps.PasswordHasher != hasher {
		t.Fatalf("PasswordHasher dependency was not preserved")
	}
	if deps.IDPModule != idp {
		t.Fatalf("IDPModule dependency was not preserved")
	}
	if deps.EventPublisher != publisher {
		t.Fatalf("EventPublisher dependency was not preserved")
	}
}

func TestAuthnModuleInitializeWithDepsRejectsMissingRequiredDeps(t *testing.T) {
	module := NewAuthnModule()
	if err := module.InitializeWithDeps(AuthnModuleDeps{}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing DB error")
	}

	if err := module.InitializeWithDeps(AuthnModuleDeps{DB: &gorm.DB{}}); err == nil {
		t.Fatalf("InitializeWithDeps() error = nil, want missing Redis error")
	}
}

type authnHasherStub struct{}

func (authnHasherStub) Verify(_, _ string) bool { return true }
func (authnHasherStub) NeedRehash(string) bool  { return false }
func (authnHasherStub) Hash(string) (string, error) {
	return "hash", nil
}
func (authnHasherStub) Pepper() string { return "" }

var _ authentication.PasswordHasher = authnHasherStub{}

type authnPublisherStub struct{}

func (authnPublisherStub) Publish(context.Context, event.DomainEvent) error {
	return nil
}

func (authnPublisherStub) PublishAll(context.Context, []event.DomainEvent) error {
	return nil
}

var _ event.Publisher = authnPublisherStub{}
