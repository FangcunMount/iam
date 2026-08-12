package sdk_test

import (
	"context"
	"testing"
	"time"

	sdk "github.com/FangcunMount/iam/v3/pkg/sdk"
	authchallenge "github.com/FangcunMount/iam/v3/pkg/sdk/auth/challenge"
	authclient "github.com/FangcunMount/iam/v3/pkg/sdk/auth/client"
	authjwks "github.com/FangcunMount/iam/v3/pkg/sdk/auth/jwks"
	authloginidentity "github.com/FangcunMount/iam/v3/pkg/sdk/auth/loginidentity"
	authloginv2 "github.com/FangcunMount/iam/v3/pkg/sdk/auth/loginv2"
	authserviceauth "github.com/FangcunMount/iam/v3/pkg/sdk/auth/serviceauth"
	authsignup "github.com/FangcunMount/iam/v3/pkg/sdk/auth/signup"
	authverifier "github.com/FangcunMount/iam/v3/pkg/sdk/auth/verifier"
	"github.com/FangcunMount/iam/v3/pkg/sdk/authz"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"github.com/FangcunMount/iam/v3/pkg/sdk/identity"
	"github.com/FangcunMount/iam/v3/pkg/sdk/idp"
)

type compileMetrics struct{}

func (m *compileMetrics) RecordRequest(method, code string, duration time.Duration) {}

type compileTracing struct{}

func (t *compileTracing) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	return ctx, func() {}
}

func (t *compileTracing) SetAttributes(context.Context, map[string]string) {}
func (t *compileTracing) RecordError(context.Context, error)               {}

func TestPublicAPISurfaceCompiles(t *testing.T) {
	t.Parallel()

	var _ *sdk.Client
	var _ = sdk.NewClient
	var _ = (*sdk.Client).Identity
	var _ = (*sdk.Client).Profile
	var _ = (*sdk.Client).ProfileLink
	var _ = sdk.WithRequestID
	var _ = sdk.WithTraceID
	var _ = sdk.GetRequestID
	var _ = sdk.GetTraceID
	var _ = sdk.ConfigFromEnv
	var _ = sdk.ConfigFromViper
	var _ = sdk.NewViperLoader
	var _ = sdk.DefaultObservabilityConfig

	var _ *sdk.Config
	var _ *sdk.TLSConfig
	var _ *sdk.RetryConfig
	var _ *sdk.JWKSConfig
	var _ *sdk.TokenVerifyConfig
	var _ *sdk.CircuitBreakerConfig
	var _ *sdk.ObservabilityConfig
	var _ *sdk.ServiceAuthConfig

	var _ sdk.MetricsCollector = (*compileMetrics)(nil)
	var _ sdk.TracingHook = (*compileTracing)(nil)
	var _ config.MetricsCollector = (*compileMetrics)(nil)
	var _ config.TracingHook = (*compileTracing)(nil)

	var opt sdk.ClientOption
	opt = sdk.WithMetricsCollector(&compileMetrics{})
	_ = opt
	opt = sdk.WithTracingHook(&compileTracing{})
	_ = opt

	var _ = authclient.NewClient
	var _ *authchallenge.Client
	var _ = authchallenge.NewClient
	var _ = (*authchallenge.Client).SendLoginPhoneOTP
	var _ = (*authchallenge.Client).StartWechatOpenAuthorize
	var _ = authchallenge.WithHTTPClient
	var _ = authchallenge.WithHeader
	var _ = authchallenge.SendLoginPhoneOTPRequest{}
	var _ = authchallenge.WechatOpenAuthorizeRequest{}
	var _ = authchallenge.WechatOpenAuthorizeResponse{}
	var _ = authchallenge.MessageResponse{}
	var _ *authloginv2.Client
	var _ = authloginv2.NewClient
	var _ = (*authloginv2.Client).Login
	var _ = authloginv2.WithHTTPClient
	var _ = authloginv2.WithHeader
	var _ authloginv2.AuthMethod = authloginv2.AuthMethodPassword
	var _ authloginv2.AuthMethod = authloginv2.AuthMethodPhoneOTP
	var _ authloginv2.AuthMethod = authloginv2.AuthMethodWechat
	var _ authloginv2.AuthMethod = authloginv2.AuthMethodWechatScan
	var _ authloginv2.AuthMethod = authloginv2.AuthMethodWecom
	var loginReq authloginv2.LoginRequest
	var _ = loginReq.Validate
	var _ = authloginv2.PasswordPayload{}
	var _ = authloginv2.PhoneOTPPayload{}
	var _ = authloginv2.WechatPayload{}
	var _ = authloginv2.WechatScanPayload{}
	var _ = authloginv2.WecomPayload{}
	var _ = authloginv2.TokenPair{}
	var _ *authsignup.Client
	var _ = authsignup.NewClient
	var _ = (*authsignup.Client).SignUpWithWechatMiniProgram
	var _ = (*authsignup.Client).EnsureMockConsumer
	var _ = authsignup.WithHTTPClient
	var _ = authsignup.WithHeader
	var _ = authsignup.WithSeedMockSecret
	var _ = authsignup.WechatMiniProgramRequest{}
	var _ = authsignup.SignupResult{}
	var _ = authsignup.SignupCredential{}
	var _ = authsignup.EnsureMockConsumerRequest{}
	var _ = authsignup.EnsureMockConsumerResult{}
	var _ *authloginidentity.Client
	var _ = authloginidentity.NewClient
	var _ = (*authloginidentity.Client).List
	var _ = (*authloginidentity.Client).SendPhoneLinkChallenge
	var _ = (*authloginidentity.Client).LinkPhone
	var _ = (*authloginidentity.Client).LinkWechatMiniProgram
	var _ = (*authloginidentity.Client).LinkWecom
	var _ = (*authloginidentity.Client).Unlink
	var _ = authloginidentity.WithHTTPClient
	var _ = authloginidentity.WithHeader
	var _ = authloginidentity.WithBearerToken
	var _ = authloginidentity.LoginIdentity{}
	var _ = authloginidentity.ListResponse{}
	var _ = authloginidentity.LinkResponse{}
	var _ = authloginidentity.MessageResponse{}
	var _ = authloginidentity.LinkPhoneChallengeRequest{}
	var _ = authloginidentity.LinkPhoneRequest{}
	var _ = authloginidentity.LinkWechatMiniProgramRequest{}
	var _ = authloginidentity.LinkWecomRequest{}
	var _ = authjwks.NewJWKSManager
	var _ = authverifier.NewTokenVerifier
	var _ = (*authverifier.TokenClaims).AuthorizationDomain
	var _ = (*authverifier.TokenClaims).BusinessOrgID
	var _ = authserviceauth.NewServiceAuthHelper

	var _ *authz.Client
	var _ = authz.NewClient
	var _ *identity.Client
	var _ = identity.NewClient
	var _ = identity.NewClientFromConn
	var _ *identity.ProfileClient
	var _ = identity.NewProfileClient
	var _ = identity.NewProfileClientFromConn
	var _ *identity.ProfileLinkClient
	var _ = identity.NewProfileLinkClient
	var _ = identity.NewProfileLinkClientFromConn
	var _ *idp.Client
	var _ = idp.NewClient

	var _ = sdkerrors.Wrap
	var _ = sdkerrors.WrapWithCode
	var _ = sdkerrors.IsNotFound
	var _ = sdkerrors.IsRetryable
	var _ = sdkerrors.AsIAMError
	var _ = sdkerrors.GRPCCode
	var _ = sdkerrors.Message
	var _ = sdkerrors.ToHTTPStatus
}
