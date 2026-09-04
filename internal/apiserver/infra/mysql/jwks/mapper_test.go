package jwks

import (
	"testing"
	"time"

	domain "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
	"github.com/stretchr/testify/require"
)

func TestMapperToKeyEntityPreservesAuditTimestamps(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(90 * time.Minute)
	jwkJSON := []byte(`{"kty":"RSA","use":"sig","alg":"RS256","kid":"kid-1","n":"n","e":"AQAB"}`)
	po := &KeyPO{
		Kid:     "kid-1",
		Status:  int8(domain.KeyActive),
		Kty:     "RSA",
		Use:     "sig",
		Alg:     "RS256",
		JwkJSON: jwkJSON,
	}
	po.CreatedAt = createdAt
	po.UpdatedAt = updatedAt

	entity, err := NewMapper().ToKeyEntity(po)
	if err != nil {
		t.Fatalf("ToKeyEntity() error = %v", err)
	}

	if entity.CreatedAt != createdAt {
		t.Fatalf("CreatedAt = %v, want %v", entity.CreatedAt, createdAt)
	}
	if entity.UpdatedAt != updatedAt {
		t.Fatalf("UpdatedAt = %v, want %v", entity.UpdatedAt, updatedAt)
	}
}

func TestMapperRejectsPersistedAlgorithmThatDisagreesWithPublicJWK(t *testing.T) {
	t.Parallel()

	po := &KeyPO{
		Kid:     "kid-1",
		Status:  int8(domain.KeyActive),
		Kty:     "RSA",
		Use:     "sig",
		Alg:     "RS384",
		JwkJSON: []byte(`{"kty":"RSA","use":"sig","alg":"RS256","kid":"kid-1","n":"n","e":"AQAB"}`),
	}

	_, err := NewMapper().ToKeyEntity(po)
	require.Error(t, err)
}
