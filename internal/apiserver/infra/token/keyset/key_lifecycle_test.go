package keyset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestKeyManagerCreateKeyAtomicallyReplacesActive(t *testing.T) {
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	old := lifecycleTestKey("old", KeyActive, now.Add(-time.Hour), now.Add(30*24*time.Hour))
	repo := &lifecycleRepositoryStub{keys: map[string]*Key{"old": old}}
	store := NewPEMPrivateKeyStorage(t.TempDir())
	manager := NewKeyManagerWithPolicy(repo, NewRSAKeyGenerator(), store, RotationPolicy{
		RotationInterval: 30 * 24 * time.Hour,
		GracePeriod:      7 * 24 * time.Hour,
		MaxKeysInJWKS:    3,
	})
	manager.now = func() time.Time { return now }

	active, err := manager.CreateKey(context.Background(), "RS256", nil, nil)
	require.NoError(t, err)
	require.True(t, active.IsActive())
	require.Contains(t, active.Kid, "key-")
	require.Equal(t, KeyGrace, repo.keys["old"].Status)
	require.Equal(t, now.Add(7*24*time.Hour), *repo.keys["old"].NotAfter)
	require.FileExists(t, filepath.Join(store.keysDir, active.Kid+".pem"))
	require.Len(t, repo.activeKeys(), 1)
}

func TestKeyManagerActivationFailureRemovesCandidatePEM(t *testing.T) {
	repo := &lifecycleRepositoryStub{
		keys:        map[string]*Key{},
		activateErr: errors.New("database unavailable"),
	}
	dir := t.TempDir()
	manager := NewKeyManagerWithPolicy(repo, NewRSAKeyGenerator(), NewPEMPrivateKeyStorage(dir), DefaultRotationPolicy())

	_, err := manager.CreateKey(context.Background(), "RS256", nil, nil)
	require.Error(t, err)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

func TestKeyManagerConcurrentBootstrapCreatesOneActive(t *testing.T) {
	repo := &lifecycleRepositoryStub{keys: map[string]*Key{}}
	manager := NewKeyManagerWithPolicy(
		repo,
		NewRSAKeyGenerator(),
		NewPEMPrivateKeyStorage(t.TempDir()),
		DefaultRotationPolicy(),
	)

	const workers = 4
	var wg sync.WaitGroup
	wg.Add(workers)
	errs := make(chan error, workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, _, err := manager.BootstrapKey(context.Background(), "RS256")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Len(t, repo.activeKeys(), 1)
}

func TestKeyManagerValidateActiveKeyMatchesPEM(t *testing.T) {
	dir := t.TempDir()
	store := NewPEMPrivateKeyStorage(dir)
	generator := NewRSAKeyGenerator()
	pair, err := generator.GenerateKeyPair(context.Background(), "RS256", "key-match")
	require.NoError(t, err)
	require.NoError(t, store.SavePrivateKey(context.Background(), "key-match", pair.PrivateKey, "RS256"))
	now := time.Now()
	repo := &lifecycleRepositoryStub{keys: map[string]*Key{
		"key-match": NewKey(
			"key-match",
			pair.PublicJWK,
			WithStatus(KeyActive),
			WithNotBefore(now.Add(-time.Minute)),
			WithNotAfter(now.Add(time.Hour)),
		),
	}}
	manager := NewKeyManagerWithPolicy(repo, generator, store, DefaultRotationPolicy())
	resolver := NewPEMPrivateKeyResolver(dir)

	_, err = manager.ValidateActiveKey(context.Background(), resolver)
	require.NoError(t, err)

	otherPair, err := generator.GenerateKeyPair(context.Background(), "RS256", "key-match")
	require.NoError(t, err)
	require.NoError(t, store.SavePrivateKey(context.Background(), "key-match", otherPair.PrivateKey, "RS256"))
	_, err = manager.ValidateActiveKey(context.Background(), resolver)
	require.True(t, perrors.IsCode(err, code.ErrInvalidJWK), "error = %v", err)
}

func TestKeyManagerValidateActiveKeyFailsClosedForMissingAndMalformedPEM(t *testing.T) {
	now := time.Now()
	key := lifecycleTestKey("key-material", KeyActive, now.Add(-time.Minute), now.Add(time.Hour))
	repo := &lifecycleRepositoryStub{keys: map[string]*Key{key.Kid: key}}
	dir := t.TempDir()
	manager := NewKeyManagerWithPolicy(repo, NewRSAKeyGenerator(), NewPEMPrivateKeyStorage(dir), DefaultRotationPolicy())
	resolver := NewPEMPrivateKeyResolver(dir)

	_, err := manager.ValidateActiveKey(context.Background(), resolver)
	require.Error(t, err, "missing PEM must fail closed")

	require.NoError(t, os.WriteFile(filepath.Join(dir, key.Kid+".pem"), []byte("not a PEM"), 0o600))
	_, err = manager.ValidateActiveKey(context.Background(), resolver)
	require.Error(t, err, "malformed PEM must fail closed")
}

func TestKeyRotationCreateAndActivateRunsSharedCleanup(t *testing.T) {
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	expired := lifecycleTestKey("expired", KeyRetired, now.Add(-48*time.Hour), now.Add(-time.Hour))
	repo := &lifecycleRepositoryStub{
		keys:    map[string]*Key{"expired": expired},
		expired: []*Key{expired},
	}
	manager := NewKeyManagerWithPolicy(
		repo,
		NewRSAKeyGenerator(),
		NewPEMPrivateKeyStorage(t.TempDir()),
		DefaultRotationPolicy(),
	)
	manager.now = func() time.Time { return now }
	lifecycle := NewKeyRotation(manager, DefaultRotationPolicy(), log.New(log.NewOptions()))

	key, changed, err := lifecycle.CreateAndActivate(context.Background(), "RS256", nil, nil)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, key.IsActive())
	require.NotContains(t, repo.keys, "expired")
}

func TestKeyRotationKeepsCommittedActivationWhenCleanupFails(t *testing.T) {
	repo := &lifecycleRepositoryStub{
		keys:       map[string]*Key{},
		expiredErr: errors.New("cleanup unavailable"),
	}
	manager := NewKeyManagerWithPolicy(
		repo,
		NewRSAKeyGenerator(),
		NewPEMPrivateKeyStorage(t.TempDir()),
		DefaultRotationPolicy(),
	)
	lifecycle := NewKeyRotation(manager, DefaultRotationPolicy(), log.New(log.NewOptions()))

	key, changed, err := lifecycle.CreateAndActivate(context.Background(), "RS256", nil, nil)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, key.IsActive())
	require.Len(t, repo.activeKeys(), 1)
}

func TestKeyRotationStateCountFailureRetainsGaugeValues(t *testing.T) {
	setKeyStateCounts(9, 8, 7)
	repo := &lifecycleRepositoryStub{
		keys:     map[string]*Key{},
		countErr: errors.New("count unavailable"),
	}
	manager := NewKeyManagerWithPolicy(repo, NewRSAKeyGenerator(), nil, DefaultRotationPolicy())
	lifecycle := NewKeyRotation(manager, DefaultRotationPolicy(), log.New(log.NewOptions()))

	require.Error(t, lifecycle.RefreshStateMetrics(context.Background()))
	require.Equal(t, float64(9), gaugeValue(t, keyStateCount.WithLabelValues("active")))
	require.Equal(t, float64(8), gaugeValue(t, keyStateCount.WithLabelValues("grace")))
	require.Equal(t, float64(7), gaugeValue(t, keyStateCount.WithLabelValues("retired")))
}

type metricWriter interface {
	Write(*dto.Metric) error
}

func gaugeValue(t *testing.T, gauge metricWriter) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, gauge.Write(metric))
	return metric.GetGauge().GetValue()
}

func lifecycleTestKey(kid string, status KeyStatus, notBefore, notAfter time.Time) *Key {
	n, e := "modulus", "AQAB"
	return &Key{
		Kid:       kid,
		Status:    status,
		JWK:       PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: kid, N: &n, E: &e},
		NotBefore: &notBefore,
		NotAfter:  &notAfter,
	}
}

type lifecycleRepositoryStub struct {
	mu          sync.Mutex
	keys        map[string]*Key
	activateErr error
	expired     []*Key
	expiredErr  error
	countErr    error
}

func (r *lifecycleRepositoryStub) Activate(_ context.Context, request ActivationRequest) (ActivationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activateErr != nil {
		return ActivationResult{}, r.activateErr
	}
	var active *Key
	for _, key := range r.keys {
		if key.IsActive() {
			active = key
			break
		}
	}
	if active != nil {
		if request.RequireNoActive {
			return ActivationResult{Active: active}, nil
		}
		if request.DueBefore != nil && active.NotBefore != nil && active.NotBefore.After(*request.DueBefore) &&
			(active.NotAfter == nil || active.NotAfter.After(request.Now)) {
			return ActivationResult{Active: active}, nil
		}
		active.Status = KeyGrace
		active.NotAfter = &request.GraceUntil
	}
	r.keys[request.Candidate.Kid] = request.Candidate
	return ActivationResult{Activated: true, Active: request.Candidate}, nil
}

func (r *lifecycleRepositoryStub) activeKeys() []*Key {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Key
	for _, key := range r.keys {
		if key.IsActive() {
			out = append(out, key)
		}
	}
	return out
}

func (r *lifecycleRepositoryStub) Save(_ context.Context, key *Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.Kid] = key
	return nil
}
func (r *lifecycleRepositoryStub) Update(_ context.Context, key *Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.Kid] = key
	return nil
}
func (r *lifecycleRepositoryStub) Delete(_ context.Context, kid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.keys, kid)
	return nil
}
func (r *lifecycleRepositoryStub) FindByKid(_ context.Context, kid string) (*Key, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.keys[kid], nil
}
func (r *lifecycleRepositoryStub) FindByStatus(_ context.Context, status KeyStatus) ([]*Key, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Key
	for _, key := range r.keys {
		if key.Status == status {
			out = append(out, key)
		}
	}
	return out, nil
}
func (r *lifecycleRepositoryStub) FindPublishable(context.Context) ([]*Key, error) {
	return nil, nil
}
func (r *lifecycleRepositoryStub) FindExpired(context.Context) ([]*Key, error) {
	return r.expired, r.expiredErr
}
func (r *lifecycleRepositoryStub) FindAll(context.Context, int, int) ([]*Key, int64, error) {
	return nil, 0, nil
}
func (r *lifecycleRepositoryStub) CountByStatus(ctx context.Context, status KeyStatus) (int64, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	keys, _ := r.FindByStatus(ctx, status)
	return int64(len(keys)), nil
}
