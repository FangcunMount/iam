package token

import (
	grantdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/grant"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
)

type SessionCreator = grantdomain.SessionCreator
type SessionLoader = tokendomain.SessionLoader
type SessionRevoker = tokendomain.SessionRevoker
type SessionExtender = tokendomain.SessionExtender
type SessionRefreshExpirer = tokendomain.SessionRefreshExpirer
type AdmissionPolicy = tokendomain.AdmissionPolicy
type TokenType = tokendomain.TokenType
type TokenClaims = tokendomain.TokenClaims
type VerifiedTokenClaims = tokendomain.VerifiedTokenClaims
type ConsumedRefreshToken = tokendomain.ConsumedRefreshToken

const (
	TokenTypeAccess  = tokendomain.TokenTypeAccess
	TokenTypeRefresh = tokendomain.TokenTypeRefresh
	TokenTypeService = tokendomain.TokenTypeService
)

// cloneStrings 克隆字符串切片
func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// cloneStringMap 克隆字符串映射
func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
