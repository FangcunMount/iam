package authn

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

func TestApplyVerifiedClaimsSetsTenantIDForRoleResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/identity/me", nil)

	now := time.Now()
	claims, err := tokendomain.NewVerifiedUserTokenClaims(tokendomain.VerifiedTokenClaims{
		TokenID: "token-1", Subject: meta.ID(110001).String(), SessionID: "sid-1",
		UserID: meta.ID(110001), LoginIdentityID: meta.ID(613486856213901870), OrgID: meta.ID(1),
		TenantDomain: tenant.DefaultID, Issuer: "https://iam.fangcunmount.cn",
		Audience: []string{"qs-api"}, AMR: []string{"pwd"},
		IssuedAt: now, NotBefore: now, ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)

	applyVerifiedClaims(c, claims)

	if got := requestctx.TenantIDOrDefault(c); got != tenant.DefaultID {
		t.Fatalf("TenantIDOrDefault() = %q, want %q", got, tenant.DefaultID)
	}
	if got, exists := c.Get(requestctx.KeyUserID); !exists || got != meta.ID(110001) {
		t.Fatalf("gin user_id = %v exists=%v, want %v", got, exists, meta.ID(110001))
	}
	if got, exists := c.Get(requestctx.KeyLoginIdentityID); !exists || got != meta.ID(613486856213901870) {
		t.Fatalf("gin login_identity_id = %v exists=%v, want %v", got, exists, meta.ID(613486856213901870))
	}
	if got, exists := c.Get(requestctx.KeyTokenID); !exists || got != "token-1" {
		t.Fatalf("gin token_id = %v exists=%v, want %q", got, exists, "token-1")
	}
}
