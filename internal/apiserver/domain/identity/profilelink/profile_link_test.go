package profilelink

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestRef_IsActive_NoRevoked(t *testing.T) {
	g := &ProfileLink{ID: meta.FromUint64(1)}
	assert.True(t, g.IsActive())
}

func TestRef_IsActive_WithRevoked(t *testing.T) {
	now := time.Now()
	g := &ProfileLink{ID: meta.FromUint64(1), RevokedAt: &now}
	assert.False(t, g.IsActive())
}

func TestRef_Revoke_SetsRevokedAt(t *testing.T) {
	g := &ProfileLink{ID: meta.FromUint64(1)}
	tNow := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	g.Revoke(tNow)

	require.NotNil(t, g.RevokedAt)
	assert.Equal(t, tNow, *g.RevokedAt)
}
