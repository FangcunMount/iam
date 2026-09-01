package jwks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var testJWKSJSON = []byte(`{"keys":[]}`)

type jwksClientStub struct {
	resp *authnv2.GetJWKSResponse
	err  error
}

func (s *jwksClientStub) GetJWKS(context.Context, *authnv2.GetJWKSRequest) (*authnv2.GetJWKSResponse, error) {
	return s.resp, s.err
}

type failingFetcher struct {
	err error
}

func (f *failingFetcher) Fetch(context.Context) (jwk.Set, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, fmt.Errorf("forced failure")
}

func (f *failingFetcher) Name() string {
	return "failing"
}

type jwksServiceServer struct {
	authnv2.UnimplementedJWKSServiceServer
	resp *authnv2.GetJWKSResponse
	err  error
}

func (s *jwksServiceServer) GetJWKS(context.Context, *authnv2.GetJWKSRequest) (*authnv2.GetJWKSResponse, error) {
	return s.resp, s.err
}

func TestHTTPFetcherFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(testJWKSJSON)
	}))
	defer server.Close()

	fetcher := NewHTTPFetcher(server.URL)
	keySet, err := fetcher.Fetch(context.Background())
	stats := fetcher.Stats()
	require.NoError(t, err)
	require.NotNil(t, keySet)
	require.Equal(t, int64(1), (&stats).Attempts())
	require.Equal(t, int64(1), (&stats).Successes())
}

func TestGRPCFetcherFetch(t *testing.T) {
	fetcher := NewGRPCFetcher(&jwksClientStub{
		resp: &authnv2.GetJWKSResponse{
			Jwks:         testJWKSJSON,
			LastModified: timestamppb.New(time.Now()),
		},
	})

	keySet, err := fetcher.Fetch(context.Background())
	stats := fetcher.Stats()
	require.NoError(t, err)
	require.NotNil(t, keySet)
	require.Equal(t, int64(1), (&stats).Successes())
}

func TestGRPCEndpointFetcherFetch(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = lis.Close() }()

	server := grpc.NewServer()
	authnv2.RegisterJWKSServiceServer(server, &jwksServiceServer{
		resp: &authnv2.GetJWKSResponse{Jwks: testJWKSJSON},
	})
	defer server.Stop()

	go func() {
		_ = server.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	fetcher := NewGRPCEndpointFetcher(lis.Addr().String())
	keySet, err := fetcher.Fetch(ctx)
	require.NoError(t, err)
	require.NotNil(t, keySet)
	require.NoError(t, fetcher.Close())
}

func TestCacheFetcherRejectsStaleDataByDefault(t *testing.T) {
	keySet, err := jwk.Parse(testJWKSJSON)
	require.NoError(t, err)

	cache := NewCacheFetcher(
		WithCacheTTL(time.Millisecond),
		WithCacheNext(&failingFetcher{}),
	)
	cache.Update(keySet)
	cache.updated = time.Now().Add(-time.Minute)

	got, err := cache.Fetch(context.Background())
	stats := cache.Stats()
	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, int64(1), (&stats).Failures())
}

func TestCacheFetcherFallsBackWithinMaxStale(t *testing.T) {
	keySet, err := jwk.Parse(testJWKSJSON)
	require.NoError(t, err)

	cache := NewCacheFetcher(
		WithCacheTTL(time.Millisecond),
		WithCacheMaxStale(time.Hour),
		WithCacheFallbackOnError(true),
		WithCacheNext(&failingFetcher{}),
	)
	cache.Update(keySet)
	cache.updated = time.Now().Add(-time.Minute)

	got, err := cache.Fetch(context.Background())
	stats := cache.Stats()
	require.NoError(t, err)
	require.Same(t, keySet, got)
	require.Equal(t, int64(1), (&stats).Successes())
}

func TestCacheFetcherRejectsDataBeyondMaxStale(t *testing.T) {
	keySet, err := jwk.Parse(testJWKSJSON)
	require.NoError(t, err)

	cache := NewCacheFetcher(
		WithCacheTTL(time.Millisecond),
		WithCacheMaxStale(time.Hour),
		WithCacheFallbackOnError(true),
		WithCacheNext(&failingFetcher{}),
	)
	cache.Update(keySet)
	cache.updated = time.Now().Add(-2 * time.Hour)

	got, err := cache.Fetch(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestDefaultChainSeparatesRefreshAndMaximumStaleWindows(t *testing.T) {
	refreshInterval := time.Minute
	cacheTTL := 24 * time.Hour
	chain := buildDefaultChain(&managerBuilder{
		config: &config.JWKSConfig{
			URL:             "https://unused.invalid/.well-known/jwks.json",
			RefreshInterval: refreshInterval,
			CacheTTL:        cacheTTL,
			FallbackOnError: true,
		},
		enableCache: true,
	})

	cache, ok := chain.(*CacheFetcher)
	require.True(t, ok)
	require.Equal(t, refreshInterval, cache.ttl)
	require.Equal(t, cacheTTL, cache.maxStale)
	require.True(t, cache.fallbackOnError)
}

func TestDefaultChainNeverTreatsCacheAsFreshBeyondMaximumStale(t *testing.T) {
	cacheTTL := time.Hour
	chain := buildDefaultChain(&managerBuilder{
		config: &config.JWKSConfig{
			URL:             "https://unused.invalid/.well-known/jwks.json",
			RefreshInterval: 24 * time.Hour,
			CacheTTL:        cacheTTL,
			FallbackOnError: true,
		},
		enableCache: true,
	})

	cache, ok := chain.(*CacheFetcher)
	require.True(t, ok)
	require.Equal(t, cacheTTL, cache.ttl)
	require.Equal(t, cacheTTL, cache.maxStale)
}

func TestHTTPFetcherFallsBackToSeed(t *testing.T) {
	seed, err := NewSeedFetcher(testJWKSJSON)
	require.NoError(t, err)

	fetcher := NewHTTPFetcher("http://127.0.0.1:1/.well-known/jwks.json", WithHTTPNext(seed), WithHTTPClient(&http.Client{
		Timeout: 10 * time.Millisecond,
	}))

	keySet, err := fetcher.Fetch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, keySet)
}

func TestCircuitBreakerFetcherOpensAfterFailures(t *testing.T) {
	fetcher := NewCircuitBreakerFetcher(&failingFetcher{}, &config.CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     time.Minute,
		SuccessThreshold: 1,
	})

	_, err := fetcher.Fetch(context.Background())
	require.Error(t, err)
	require.Equal(t, CircuitOpen, fetcher.State())

	_, err = fetcher.Fetch(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "circuit is open")
}
