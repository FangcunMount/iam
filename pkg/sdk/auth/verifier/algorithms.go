package verifier

import (
	"fmt"
	"strings"

	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwa"
)

func configuredAlgorithms(cfg *config.TokenVerifyConfig) []jwa.SignatureAlgorithm {
	if validateConfiguredAlgorithms(cfg) != nil {
		return nil
	}
	return []jwa.SignatureAlgorithm{jwa.RS256}
}

func validateConfiguredAlgorithms(cfg *config.TokenVerifyConfig) error {
	if cfg == nil || len(cfg.Algorithms) == 0 {
		return nil
	}
	for _, alg := range cfg.Algorithms {
		if alg != pkgauth.TokenProfileAlgorithm {
			return fmt.Errorf("token verification algorithm must be %s, got %q", pkgauth.TokenProfileAlgorithm, alg)
		}
	}
	return nil
}

func validateRequiredIssuerAudience(cfg *config.TokenVerifyConfig) error {
	if cfg == nil {
		return fmt.Errorf("token verify config is required")
	}
	if strings.TrimSpace(cfg.AllowedIssuer) == "" {
		return fmt.Errorf("allowed issuer is required")
	}
	if len(cfg.AllowedAudience) == 0 {
		return fmt.Errorf("allowed audience is required")
	}
	for _, audience := range cfg.AllowedAudience {
		if strings.TrimSpace(audience) == "" {
			return fmt.Errorf("allowed audience entries must be non-empty")
		}
	}
	return nil
}
