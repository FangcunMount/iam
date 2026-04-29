package keyset

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyStatus_StringValues(t *testing.T) {
	assert.Equal(t, "active", KeyActive.String())
	assert.Equal(t, "grace", KeyGrace.String())
	assert.Equal(t, "retired", KeyRetired.String())
}

func mustStr(s string) *string { return &s }

func TestKey_ValidityAndExpiry(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	jwk := PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: "k1", N: mustStr("n"), E: mustStr("e")}
	k := NewKey("k1", jwk)
	// initially active and no expiry
	assert.True(t, k.IsActive())
	// debug: print values if something unexpected happens
	t.Logf("k.NotAfter=%v k.NotBefore=%v", k.NotAfter, k.NotBefore)
	require.Nil(t, k.NotAfter)
	require.Nil(t, k.NotBefore)
	t.Logf("IsExpired(now)=%v IsNotYetValid(now)=%v", k.IsExpired(now), k.IsNotYetValid(now))
	assert.False(t, k.IsExpired(now))
	assert.False(t, k.IsNotYetValid(now))
	assert.True(t, k.IsValidAt(now))
	assert.True(t, k.CanSign())
	assert.True(t, k.CanVerify())
	assert.True(t, k.ShouldPublish())

	// set not after in the past -> expired
	k2 := NewKey("k2", jwk, WithNotAfter(past))
	assert.True(t, k2.IsExpired(now))
	assert.False(t, k2.IsValidAt(now))

	// set not before in the future -> not yet valid
	k3 := NewKey("k3", jwk, WithNotBefore(future))
	assert.True(t, k3.IsNotYetValid(now))
	assert.False(t, k3.IsValidAt(now))
}

func TestKey_StateTransitions(t *testing.T) {
	jwk := PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: "kid", N: mustStr("n"), E: mustStr("e")}
	k := NewKey("kid", jwk)
	require.False(t, k.CreatedAt.IsZero())
	require.False(t, k.UpdatedAt.IsZero())
	createdAt := k.CreatedAt
	updatedAt := k.UpdatedAt

	// enter grace from active
	require.NoError(t, k.EnterGrace())
	assert.True(t, k.IsGrace())
	assert.Equal(t, createdAt, k.CreatedAt)
	assert.False(t, k.UpdatedAt.Before(updatedAt))

	// cannot enter grace again
	err := k.EnterGrace()
	assert.Error(t, err)

	// retire from grace
	require.NoError(t, k.Retire())
	assert.True(t, k.IsRetired())

	// retire when already retired should fail (since not grace)
	err = k.Retire()
	assert.Error(t, err)

	// force retire always sets retired
	k4 := NewKey("kid2", jwk)
	k4.ForceRetire()
	assert.True(t, k4.IsRetired())
}

func TestKey_ValidateAndJWKValidation(t *testing.T) {
	// valid RSA
	jwkRSA := PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: "r1", N: mustStr("n"), E: mustStr("e")}
	k := NewKey("r1", jwkRSA)
	require.NoError(t, k.Validate())

	// missing kid
	jwkBad := jwkRSA
	jwkBad.Kid = ""
	kBad := NewKey("r2", jwkBad)
	assert.Error(t, kBad.Validate())

	// mismatch kid
	jwkMismatch := jwkRSA
	jwkMismatch.Kid = "other"
	kMismatch := NewKey("r3", jwkMismatch)
	assert.Error(t, kMismatch.Validate())

	// unsupported kty
	jwkUnsupported := PublicJWK{Kty: "XYZ", Use: "sig", Alg: "X", Kid: "x1"}
	kUnsupported := NewKey("x1", jwkUnsupported)
	assert.Error(t, kUnsupported.Validate())

	// EC requires crv,x,y
	jwkEC := PublicJWK{Kty: "EC", Use: "sig", Alg: "ES256", Kid: "e1", Crv: mustStr("P-256"), X: mustStr("x"), Y: mustStr("y")}
	kEC := NewKey("e1", jwkEC)
	require.NoError(t, kEC.Validate())

	// OKP requires crv,x
	jwkOKP := PublicJWK{Kty: "OKP", Use: "sig", Alg: "EdDSA", Kid: "o1", Crv: mustStr("Ed25519"), X: mustStr("x")}
	kOKP := NewKey("o1", jwkOKP)
	require.NoError(t, kOKP.Validate())

	// invalid time range
	nb := time.Now().Add(10 * time.Hour)
	na := time.Now().Add(-10 * time.Hour)
	jwkTime := jwkRSA
	kTime := NewKey("t1", jwkTime, WithNotBefore(nb), WithNotAfter(na))
	assert.Error(t, kTime.Validate())
}

func TestJWKS_ValidateAndHelpers(t *testing.T) {
	// empty JWKS invalid
	j := &JWKS{}
	assert.Error(t, j.Validate())

	// valid JWKS
	jwk := PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: "a1", N: mustStr("n"), E: mustStr("e")}
	j2 := &JWKS{Keys: []PublicJWK{jwk}}
	require.NoError(t, j2.Validate())
	assert.Equal(t, 1, j2.Count())
	assert.False(t, j2.IsEmpty())
	found := j2.FindByKid("a1")
	require.NotNil(t, found)
	assert.Equal(t, "a1", found.Kid)
}

func TestCacheTagAndETag(t *testing.T) {
	ct := CacheTag{}
	assert.True(t, ct.IsZero())

	ct.ETag = "v1"
	ct.LastModified = time.Now()
	assert.False(t, ct.IsZero())

	other := CacheTag{ETag: "v1", LastModified: ct.LastModified}
	assert.True(t, ct.Matches(other))

	et := GenerateETag([]byte("hello"))
	require.NotEmpty(t, et)
}

func TestRotationPolicy_Validation(t *testing.T) {
	p := DefaultRotationPolicy()
	require.NoError(t, p.Validate())

	// invalid rotation interval
	p2 := p
	p2.RotationInterval = 0
	assert.Error(t, p2.Validate())

	// invalid grace (too long)
	p3 := p
	p3.GracePeriod = p3.RotationInterval
	assert.Error(t, p3.Validate())
}

type snapshotTestRepository struct {
	publishable []*Key
	calls       atomic.Int64
}

func (r *snapshotTestRepository) Save(context.Context, *Key) error                { return nil }
func (r *snapshotTestRepository) Update(context.Context, *Key) error              { return nil }
func (r *snapshotTestRepository) Delete(context.Context, string) error            { return nil }
func (r *snapshotTestRepository) FindByKid(context.Context, string) (*Key, error) { return nil, nil }
func (r *snapshotTestRepository) FindByStatus(context.Context, KeyStatus) ([]*Key, error) {
	return nil, nil
}
func (r *snapshotTestRepository) FindPublishable(context.Context) ([]*Key, error) {
	r.calls.Add(1)
	return r.publishable, nil
}
func (r *snapshotTestRepository) FindExpired(context.Context) ([]*Key, error) { return nil, nil }
func (r *snapshotTestRepository) FindAll(context.Context, int, int) ([]*Key, int64, error) {
	return nil, 0, nil
}
func (r *snapshotTestRepository) CountByStatus(context.Context, KeyStatus) (int64, error) {
	return 0, nil
}

func TestKeySetBuilderSnapshotStatus(t *testing.T) {
	repo := &snapshotTestRepository{
		publishable: []*Key{
			NewKey("kid-1", PublicJWK{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: "kid-1",
				N:   mustStr("n"),
				E:   mustStr("e"),
			}),
		},
	}
	builder := NewKeySetBuilder(repo)

	initial := builder.SnapshotStatus()
	assert.False(t, initial.Cached)
	assert.Zero(t, initial.KeyCount)
	assert.Nil(t, initial.LastBuildTime)

	_, tag, err := builder.BuildJWKS(context.Background())
	require.NoError(t, err)

	snapshot := builder.SnapshotStatus()
	assert.True(t, snapshot.Cached)
	assert.Equal(t, 1, snapshot.KeyCount)
	require.NotNil(t, snapshot.LastBuildTime)
	assert.Equal(t, tag.ETag, snapshot.CacheTag.ETag)
}

func TestKeySetBuilderCurrentCacheTagUsesFreshSnapshotWithinTTL(t *testing.T) {
	repo := &snapshotTestRepository{
		publishable: []*Key{
			NewKey("kid-1", PublicJWK{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: "kid-1",
				N:   mustStr("n"),
				E:   mustStr("e"),
			}),
		},
	}
	builder := NewKeySetBuilder(repo)

	first, err := builder.GetCurrentCacheTag(context.Background())
	require.NoError(t, err)
	second, err := builder.GetCurrentCacheTag(context.Background())
	require.NoError(t, err)

	require.Equal(t, first.ETag, second.ETag)
	require.Equal(t, int64(1), repo.calls.Load())
}

func TestKeySetBuilderCurrentCacheTagRefreshesAfterTTL(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	repo := &snapshotTestRepository{
		publishable: []*Key{
			NewKey("kid-1", PublicJWK{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: "kid-1",
				N:   mustStr("n"),
				E:   mustStr("e"),
			}),
		},
	}
	builder := NewKeySetBuilder(repo)
	builder.setClockForTest(func() time.Time { return now })

	_, err := builder.GetCurrentCacheTag(context.Background())
	require.NoError(t, err)
	now = now.Add(2 * time.Minute)
	_, err = builder.GetCurrentCacheTag(context.Background())
	require.NoError(t, err)

	require.Equal(t, int64(2), repo.calls.Load())
}

func TestKeySetBuilderConcurrentSnapshotAccessIsStable(t *testing.T) {
	repo := &snapshotTestRepository{
		publishable: []*Key{
			NewKey("kid-1", PublicJWK{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: "kid-1",
				N:   mustStr("n"),
				E:   mustStr("e"),
			}),
		},
	}
	builder := NewKeySetBuilder(repo)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := builder.BuildJWKS(context.Background())
			require.NoError(t, err)
			_ = builder.SnapshotStatus()
		}()
	}
	wg.Wait()

	snapshot := builder.SnapshotStatus()
	require.True(t, snapshot.Cached)
	require.Equal(t, 1, snapshot.KeyCount)
}
