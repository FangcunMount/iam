package authn

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

const SeedMockSecretHeader = "X-IAM-Seed-Secret"

func RequireSeedMockSecret(secret string) gin.HandlerFunc {
	expected := strings.TrimSpace(secret)
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader(SeedMockSecretHeader))
		if expected == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":      code.ErrUnauthenticated,
				"message":   errors.ParseCoder(errors.WithCode(code.ErrUnauthenticated, "seed mock secret invalid")).String(),
				"reference": "",
			})
			return
		}
		c.Next()
	}
}
