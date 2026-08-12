package serviceauth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

type issueServiceTokenClientStub struct {
	mu        sync.Mutex
	responses []*authnv2.IssueServiceTokenResponse
	errs      []error
	calls     int
}

func (s *issueServiceTokenClientStub) IssueServiceToken(context.Context, *authnv2.IssueServiceTokenRequest) (*authnv2.IssueServiceTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.calls
	s.calls++

	if idx < len(s.errs) && s.errs[idx] != nil {
		return nil, s.errs[idx]
	}
	if idx < len(s.responses) && s.responses[idx] != nil {
		return s.responses[idx], nil
	}
	return nil, errors.New("unexpected IssueServiceToken call")
}

func TestNewServiceAuthHelperGetsInitialToken(t *testing.T) {
	stub := &issueServiceTokenClientStub{
		responses: []*authnv2.IssueServiceTokenResponse{
			{
				TokenPair: &authnv2.TokenPair{
					AccessToken: "svc-token-1",
					ExpiresIn:   durationpb.New(time.Minute),
				},
			},
		},
	}

	helper, err := NewServiceAuthHelper(&config.ServiceAuthConfig{
		ServiceID:      "qs-service",
		TargetAudience: []string{"iam-service"},
		TokenTTL:       time.Minute,
		RefreshBefore:  5 * time.Second,
	}, stub)
	require.NoError(t, err)
	defer helper.Stop()

	token, err := helper.GetToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-token-1", token)
	require.Equal(t, "Bearer svc-token-1", AuthorizationMetadata(token).Get("authorization")[0])
}

func TestGetTokenFallsBackToCachedTokenOnRefreshFailure(t *testing.T) {
	stub := &issueServiceTokenClientStub{
		responses: []*authnv2.IssueServiceTokenResponse{
			{
				TokenPair: &authnv2.TokenPair{
					AccessToken: "svc-token-1",
					ExpiresIn:   durationpb.New(5 * time.Second),
				},
			},
		},
		errs: []error{
			nil,
			errors.New("refresh failed"),
		},
	}

	helper, err := NewServiceAuthHelper(&config.ServiceAuthConfig{
		ServiceID:      "qs-service",
		TargetAudience: []string{"iam-service"},
		TokenTTL:       time.Minute,
		RefreshBefore:  10 * time.Second,
	}, stub)
	require.NoError(t, err)
	defer helper.Stop()

	token, err := helper.GetToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-token-1", token)
	require.Equal(t, RefreshStateRetrying, helper.State())
}

func TestServiceAuthHelperOpensCircuitAfterMaxRetries(t *testing.T) {
	stub := &issueServiceTokenClientStub{
		responses: []*authnv2.IssueServiceTokenResponse{
			{
				TokenPair: &authnv2.TokenPair{
					AccessToken: "svc-token-1",
					ExpiresIn:   durationpb.New(5 * time.Second),
				},
			},
		},
		errs: []error{
			nil,
			errors.New("refresh failed 1"),
			errors.New("refresh failed 2"),
		},
	}

	helper, err := NewServiceAuthHelper(
		&config.ServiceAuthConfig{
			ServiceID:      "qs-service",
			TargetAudience: []string{"iam-service"},
			TokenTTL:       time.Minute,
			RefreshBefore:  10 * time.Second,
		},
		stub,
		WithRefreshStrategy(&RefreshStrategy{
			JitterRatio:         0,
			MinBackoff:          time.Millisecond,
			MaxBackoff:          time.Millisecond,
			BackoffMultiplier:   1,
			MaxRetries:          2,
			CircuitOpenDuration: time.Second,
		}),
	)
	require.NoError(t, err)
	defer helper.Stop()

	token, err := helper.GetToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-token-1", token)

	token, err = helper.GetToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-token-1", token)
	require.Equal(t, RefreshStateCircuitOpen, helper.State())
}
