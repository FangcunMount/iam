package authn

import (
	"context"

	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

var (
	_ authentication.LoginPhoneOTPVerifier  = (*loginPhoneOTPVerifierAdapter)(nil)
	_ linkingApp.PhoneLinkChallengeVerifier = (*phoneLinkOTPVerifierAdapter)(nil)
)

// loginPhoneOTPVerifierAdapter adapts the challenge application use case to the
// domain login OTP verifier port instead of letting application services satisfy
// domain ports directly.
type loginPhoneOTPVerifierAdapter struct {
	verifier challengeApp.LoginPhoneOTPVerifier
}

func newLoginPhoneOTPVerifierAdapter(verifier challengeApp.LoginPhoneOTPVerifier) authentication.LoginPhoneOTPVerifier {
	return &loginPhoneOTPVerifierAdapter{verifier: verifier}
}

func newPhoneOTPAuthStrategy(
	identityRepo authentication.LoginIdentityRepository,
	verifier challengeApp.LoginPhoneOTPVerifier,
) authentication.AuthStrategy {
	return authentication.NewPhoneOTPAuthStrategyWithLoginIdentity(identityRepo, newLoginPhoneOTPVerifierAdapter(verifier))
}

func (a *loginPhoneOTPVerifierAdapter) VerifyAndConsumeLoginPhoneOTP(ctx context.Context, phoneE164, code string) bool {
	if a == nil || a.verifier == nil {
		return false
	}
	return a.verifier.VerifyAndConsumeLoginPhoneOTP(ctx, phoneE164, code)
}

// phoneLinkOTPVerifierAdapter adapts challenge phone-link OTP verification to
// linking's local port.
type phoneLinkOTPVerifierAdapter struct {
	verifier challengeApp.PhoneLinkOTPVerifier
}

func newPhoneLinkOTPVerifierAdapter(verifier challengeApp.PhoneLinkOTPVerifier) linkingApp.PhoneLinkChallengeVerifier {
	return &phoneLinkOTPVerifierAdapter{verifier: verifier}
}

func (a *phoneLinkOTPVerifierAdapter) VerifyAndConsumePhoneLinkOTP(ctx context.Context, phoneE164, code string) bool {
	if a == nil || a.verifier == nil {
		return false
	}
	return a.verifier.VerifyAndConsumePhoneLinkOTP(ctx, phoneE164, code)
}
