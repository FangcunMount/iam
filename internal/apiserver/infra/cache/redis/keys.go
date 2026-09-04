package redis

import (
	"crypto/sha256"
	"fmt"

	rediskeyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
)

var (
	refreshTokenKeyspace         = rediskeyspace.New("refresh_token")
	consumedRefreshTokenKeyspace = rediskeyspace.New("consumed_refresh_token")
	// revoked_access_token is a legacy physical keyspace name shared by access/service bearer-token markers.
	revokedAccessTokenKeyspace        = rediskeyspace.New("revoked_access_token")
	sessionKeyspace                   = rediskeyspace.New("session")
	userSessionIndexKeyspace          = rediskeyspace.New("user_session_index")
	loginIdentitySessionIndexKeyspace = rediskeyspace.New("login_identity_session_index")
	challengeKeyspace                 = rediskeyspace.New("authn").Child("challenge")
	otpKeyspace                       = rediskeyspace.New("otp")
	otpSendGateKeyspace               = otpKeyspace.Child("sendgate")
	otpSendQuotaKeyspace              = otpKeyspace.Child("quota")
	wechatAccessTokenKeyspace         = rediskeyspace.New("idp").Child("wechat").Child("token")
	wechatAccessTokenLockKeyspace     = wechatAccessTokenKeyspace.Child("lock")
)

func refreshTokenRedisKey(tokenValue string) string {
	return refreshTokenKeyspace.Prefix(tokenValue)
}

func consumedRefreshTokenRedisKey(tokenValue string) string {
	digest := sha256.Sum256([]byte(tokenValue))
	return consumedRefreshTokenKeyspace.Prefix(fmt.Sprintf("%x", digest))
}

func revokedBearerTokenRedisKey(tokenID string) string {
	return revokedAccessTokenKeyspace.Prefix(tokenID)
}

func sessionRedisKey(sessionID string) string {
	return sessionKeyspace.Prefix(sessionID)
}

func userSessionIndexRedisKey(userID string) string {
	return userSessionIndexKeyspace.Prefix(userID)
}

func loginIdentitySessionIndexRedisKey(loginIdentityID string) string {
	return loginIdentitySessionIndexKeyspace.Prefix(loginIdentityID)
}

func challengeRedisKey(challengeID string) string {
	return challengeKeyspace.Prefix(challengeID)
}

func otpSendGateRedisKey(phoneE164, scene string) string {
	return otpSendGateKeyspace.Prefix(fmt.Sprintf("%s:%s", scene, phoneE164))
}

func otpSendQuotaRedisKey(phoneE164, scene, dimension string) string {
	return otpSendQuotaKeyspace.Prefix(fmt.Sprintf("%s:%s:%s", scene, phoneE164, dimension))
}

func wechatAccessTokenRedisKey(appID string) string {
	return wechatAccessTokenKeyspace.Prefix(appID)
}

func wechatAccessTokenLockRedisKey(appID string) string {
	return wechatAccessTokenLockKeyspace.Prefix(appID)
}
