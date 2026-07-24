package jwks

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/component-base/pkg/log"
)

func TestRotateIfDueRefreshesPublishCacheOnlyAfterRotation(t *testing.T) {
	rotator := &keyRotatorStub{
		key:     &ManagedKey{Kid: "key-new", Status: KeyActive, JWK: PublicJWK{Alg: "RS256"}},
		rotated: true,
	}
	publisher := &keyPublisherStub{}
	service := NewKeyRotationAppService(rotator, log.New(log.NewOptions()), publisher)

	response, err := service.RotateIfDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !response.Rotated || publisher.refreshCalls != 1 {
		t.Fatalf("Rotated=%v refreshCalls=%d, want true/1", response.Rotated, publisher.refreshCalls)
	}

	rotator.rotated = false
	if _, err := service.RotateIfDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if publisher.refreshCalls != 1 {
		t.Fatalf("no-op rotation refreshed cache; calls=%d", publisher.refreshCalls)
	}
}

func TestRotateKeyKeepsCommittedRotationWhenCacheRefreshFails(t *testing.T) {
	rotator := &keyRotatorStub{
		key:     &ManagedKey{Kid: "key-new", Status: KeyActive, JWK: PublicJWK{Alg: "RS256"}},
		rotated: true,
	}
	publisher := &keyPublisherStub{refreshErr: errors.New("refresh failed")}
	service := NewKeyRotationAppService(rotator, log.New(log.NewOptions()), publisher)

	response, err := service.RotateKey(context.Background())
	if err != nil {
		t.Fatalf("RotateKey() error = %v; cache refresh must not roll back committed rotation", err)
	}
	if response.NewKey.Kid != "key-new" || publisher.refreshCalls != 1 {
		t.Fatalf("response=%+v refreshCalls=%d", response, publisher.refreshCalls)
	}
}

type keyRotatorStub struct {
	key     *ManagedKey
	rotated bool
}

func (s *keyRotatorStub) RotateKey(context.Context) (*ManagedKey, error) {
	return s.key, nil
}

func (s *keyRotatorStub) RotateIfDue(context.Context) (*ManagedKey, bool, error) {
	return s.key, s.rotated, nil
}

func (s *keyRotatorStub) ShouldRotate(context.Context) (bool, error) {
	return s.rotated, nil
}

func (s *keyRotatorStub) GetRotationPolicy() RotationPolicy {
	return DefaultRotationPolicy()
}

func (s *keyRotatorStub) UpdateRotationPolicy(context.Context, RotationPolicy) error {
	return nil
}

type keyPublisherStub struct {
	refreshCalls int
	refreshErr   error
}

func (s *keyPublisherStub) BuildJWKS(context.Context) ([]byte, CacheTag, error) {
	return nil, CacheTag{}, nil
}

func (s *keyPublisherStub) GetPublishableKeys(context.Context) ([]*ManagedKey, error) {
	return nil, nil
}

func (s *keyPublisherStub) ValidateCacheTag(context.Context, CacheTag) (bool, error) {
	return false, nil
}

func (s *keyPublisherStub) GetCurrentCacheTag(context.Context) (CacheTag, error) {
	return CacheTag{}, nil
}

func (s *keyPublisherStub) RefreshCache(context.Context) error {
	s.refreshCalls++
	return s.refreshErr
}
