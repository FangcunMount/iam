package requestctx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestUserIDUsesMetaIDAsPrimaryContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	SetUserID(c, meta.FromUint64(1001))

	raw, exists := c.Get(KeyUserID)
	require.True(t, exists)
	require.Equal(t, meta.FromUint64(1001), raw)

	id, ok := UserID(c)
	require.True(t, ok)
	require.Equal(t, meta.FromUint64(1001), id)

	id, err := RequiredUserID(c)
	require.NoError(t, err)
	require.Equal(t, meta.FromUint64(1001), id)

	require.Equal(t, "1001", id.String())
}

func TestUserIDKeepsStringCompatibilityForLegacyContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(KeyUserID, "1001")

	id, ok := UserID(c)
	require.True(t, ok)
	require.Equal(t, meta.FromUint64(1001), id)
}

func TestLoginIdentityIDUsesMetaIDAsPrimaryContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	SetLoginIdentityID(c, meta.FromUint64(2002))

	raw, exists := c.Get(KeyLoginIdentityID)
	require.True(t, exists)
	require.Equal(t, meta.FromUint64(2002), raw)

	id, ok := LoginIdentityID(c)
	require.True(t, ok)
	require.Equal(t, meta.FromUint64(2002), id)

	require.Equal(t, "2002", id.String())
}
