package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func TestNewAccessAndRefreshTokensAndPair(t *testing.T) {
	userID := meta.FromUint64(1)
	acctID := meta.FromUint64(2)
	tenantID := meta.FromUint64(3)
	at := NewAccessToken("aid", "val", "sid-1", userID, acctID, tenantID, time.Minute)
	rt := NewRefreshToken("rid", "rval", "sid-1", userID, acctID, tenantID, nil, nil, time.Hour)

	assert.Equal(t, TokenTypeAccess, at.Type)
	assert.Equal(t, TokenTypeRefresh, rt.Type)
	pair := NewTokenPair(at, rt)
	assert.NotNil(t, pair)
	assert.Equal(t, "aid", pair.AccessToken.ID)
}

func TestTokenExpiryAndRemaining(t *testing.T) {
	userID := meta.FromUint64(3)
	acctID := meta.FromUint64(4)
	tenantID := meta.FromUint64(5)
	// expired token by negative duration
	texp := NewAccessToken("e", "v", "sid-expired", userID, acctID, tenantID, -time.Minute)
	assert.True(t, texp.IsExpired())
	assert.Equal(t, time.Duration(0), texp.RemainingDuration())

	// valid token
	tvalid := NewAccessToken("v", "val", "sid-valid", userID, acctID, tenantID, time.Minute)
	assert.False(t, tvalid.IsExpired())
	rd := tvalid.RemainingDuration()
	assert.Greater(t, rd, time.Duration(0))
}

func TestTokenClaimsExpiry(t *testing.T) {
	now := time.Now()
	uid := meta.FromUint64(5)
	aid := meta.FromUint64(6)
	tenantID := meta.FromUint64(7)
	claims := NewTokenClaims(TokenTypeAccess, "tid", uid.String(), "sid-expired", uid, aid, tenantID, "iam", nil, nil, nil, now.Add(-time.Minute), now.Add(-time.Second))
	assert.True(t, claims.IsExpired())

	uid2 := meta.FromUint64(7)
	aid2 := meta.FromUint64(8)
	tenantID2 := meta.FromUint64(9)
	claims2 := NewTokenClaims(TokenTypeAccess, "tid2", uid2.String(), "sid-valid", uid2, aid2, tenantID2, "iam", nil, nil, nil, now, now.Add(time.Minute))
	assert.False(t, claims2.IsExpired())
}
