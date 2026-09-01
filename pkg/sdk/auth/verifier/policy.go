package verifier

import (
	"errors"
	"fmt"
	"time"

	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	iamerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type verificationPolicy struct {
	issuer            string
	audience          []string
	clockSkew         time.Duration
	requiredClaims    []string
	allowedAlgorithms map[string]struct{}
}

func newVerificationPolicy(cfg *config.TokenVerifyConfig, opts *VerifyOptions) verificationPolicy {
	policy := verificationPolicy{allowedAlgorithms: make(map[string]struct{})}
	if cfg != nil {
		policy.issuer = cfg.AllowedIssuer
		policy.audience = append([]string(nil), cfg.AllowedAudience...)
		policy.clockSkew = cfg.ClockSkew
		policy.requiredClaims = append([]string(nil), cfg.RequiredClaims...)
		if cfg.RequireExpirationTime && !containsString(policy.requiredClaims, jwt.ExpirationKey) {
			policy.requiredClaims = append(policy.requiredClaims, jwt.ExpirationKey)
		}
	}
	if opts != nil {
		if opts.ExpectedIssuer != "" {
			policy.issuer = opts.ExpectedIssuer
		}
		if len(opts.ExpectedAudience) > 0 {
			policy.audience = append([]string(nil), opts.ExpectedAudience...)
		}
	}
	for _, algorithm := range configuredAlgorithms(cfg) {
		policy.allowedAlgorithms[algorithm.String()] = struct{}{}
	}
	return policy
}

func (p verificationPolicy) appendParseOptions(options []jwt.ParseOption) []jwt.ParseOption {
	for _, audience := range p.audience {
		options = append(options, jwt.WithAudience(audience))
	}
	if p.issuer != "" {
		options = append(options, jwt.WithIssuer(p.issuer))
	}
	if p.clockSkew > 0 {
		options = append(options, jwt.WithAcceptableSkew(p.clockSkew))
	}
	for _, claim := range p.requiredClaims {
		options = append(options, jwt.WithRequiredClaim(claim))
	}
	return options
}

// validateTokenEnvelope checks reject-only constraints on an untrusted compact JWT.
// Signature authenticity remains the responsibility of the selected local or remote strategy.
func (p verificationPolicy) validateTokenEnvelope(tokenString string) error {
	if err := p.validateAlgorithm(tokenString); err != nil {
		return err
	}
	token, err := jwt.ParseInsecure([]byte(tokenString))
	if err != nil {
		return invalidTokenError("parse token claims: %v", err)
	}

	var options []jwt.ValidateOption
	for _, audience := range p.audience {
		options = append(options, jwt.WithAudience(audience))
	}
	if p.issuer != "" {
		options = append(options, jwt.WithIssuer(p.issuer))
	}
	if p.clockSkew > 0 {
		options = append(options, jwt.WithAcceptableSkew(p.clockSkew))
	}
	for _, claim := range p.requiredClaims {
		options = append(options, jwt.WithRequiredClaim(claim))
	}
	if err := jwt.Validate(token, options...); err != nil {
		return mapTokenValidationError(err)
	}
	return nil
}

func (p verificationPolicy) validateAlgorithm(tokenString string) error {
	message, err := jws.Parse([]byte(tokenString))
	if err != nil {
		return invalidTokenError("parse token header: %v", err)
	}
	signatures := message.Signatures()
	if len(signatures) != 1 || signatures[0].ProtectedHeaders() == nil {
		return invalidTokenError("token must contain exactly one protected signature")
	}
	algorithm := signatures[0].ProtectedHeaders().Algorithm().String()
	if _, allowed := p.allowedAlgorithms[algorithm]; !allowed {
		return invalidTokenError("signing algorithm %q is not allowed", algorithm)
	}
	return nil
}

func mapTokenValidationError(err error) error {
	if errors.Is(err, jwt.ErrTokenExpired()) {
		return iamerrors.ErrTokenExpired
	}
	return invalidTokenError("validation failed: %v", err)
}

func invalidTokenError(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", iamerrors.ErrTokenInvalid, fmt.Sprintf(format, args...))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
