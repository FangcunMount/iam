package redis

import (
	"fmt"

	rediskeyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
)

var (
	refreshTokenKeyspace          = rediskeyspace.New("refresh_token")
	revokedAccessTokenKeyspace    = rediskeyspace.New("revoked_access_token")
	sessionKeyspace               = rediskeyspace.New("session")
	userSessionIndexKeyspace      = rediskeyspace.New("user_session_index")
	accountSessionIndexKeyspace   = rediskeyspace.New("account_session_index")
	otpKeyspace                   = rediskeyspace.New("otp")
	otpSendGateKeyspace           = otpKeyspace.Child("sendgate")
	wechatAccessTokenKeyspace     = rediskeyspace.New("idp").Child("wechat").Child("token")
	wechatAccessTokenLockKeyspace = wechatAccessTokenKeyspace.Child("lock")
)

func refreshTokenRedisKey(tokenValue string) string {
	return refreshTokenKeyspace.Prefix(tokenValue)
}

func revokedAccessTokenRedisKey(tokenID string) string {
	return revokedAccessTokenKeyspace.Prefix(tokenID)
}

func sessionRedisKey(sessionID string) string {
	return sessionKeyspace.Prefix(sessionID)
}

func userSessionIndexRedisKey(userID string) string {
	return userSessionIndexKeyspace.Prefix(userID)
}

func accountSessionIndexRedisKey(accountID string) string {
	return accountSessionIndexKeyspace.Prefix(accountID)
}

func otpRedisKey(phoneE164, scene, code string) string {
	return otpKeyspace.Prefix(fmt.Sprintf("%s:%s:%s", scene, phoneE164, code))
}

func otpSendGateRedisKey(phoneE164, scene string) string {
	return otpSendGateKeyspace.Prefix(fmt.Sprintf("%s:%s", scene, phoneE164))
}

func wechatAccessTokenRedisKey(appID string) string {
	return wechatAccessTokenKeyspace.Prefix(appID)
}

func wechatAccessTokenLockRedisKey(appID string) string {
	return wechatAccessTokenLockKeyspace.Prefix(appID)
}
