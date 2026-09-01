package verifier

import (
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwa"
)

func configuredAlgorithms(cfg *config.TokenVerifyConfig) []jwa.SignatureAlgorithm {
	if cfg == nil || len(cfg.Algorithms) == 0 {
		return []jwa.SignatureAlgorithm{jwa.RS256}
	}

	algorithms := make([]jwa.SignatureAlgorithm, 0, len(cfg.Algorithms))
	for _, alg := range cfg.Algorithms {
		switch alg {
		case "RS256":
			algorithms = append(algorithms, jwa.RS256)
		case "RS384":
			algorithms = append(algorithms, jwa.RS384)
		case "RS512":
			algorithms = append(algorithms, jwa.RS512)
		case "ES256":
			algorithms = append(algorithms, jwa.ES256)
		case "ES384":
			algorithms = append(algorithms, jwa.ES384)
		case "ES512":
			algorithms = append(algorithms, jwa.ES512)
		case "PS256":
			algorithms = append(algorithms, jwa.PS256)
		case "PS384":
			algorithms = append(algorithms, jwa.PS384)
		case "PS512":
			algorithms = append(algorithms, jwa.PS512)
		case "EdDSA":
			algorithms = append(algorithms, jwa.EdDSA)
		}
	}

	return algorithms
}
