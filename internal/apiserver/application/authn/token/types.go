package token

import (
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
)

// SessionCreator 是 token 用例依赖的会话创建协作者。
type SessionCreator = sessiondomain.Creator

// SessionLoader 是 token 用例依赖的会话加载协作者。
type SessionLoader = sessiondomain.Loader

// SessionRevoker 是 token 用例依赖的会话撤销协作者。
type SessionRevoker = sessiondomain.Revoker

// SessionExtender 是 token 用例依赖的会话延期协作者。
type SessionExtender = sessiondomain.Extender

// SessionRefreshExpirer 是 token 用例依赖的 refresh 过期时间计算协作者。
type SessionRefreshExpirer = sessiondomain.RefreshExpirer

// SubjectAccessEvaluator 是 token 用例依赖的主体访问状态领域协作者。
type SubjectAccessEvaluator = sessiondomain.SubjectAccessEvaluator

// TokenType 表示 IAM 内部令牌用途。
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"  // 访问令牌
	TokenTypeRefresh TokenType = "refresh" // 刷新令牌
	TokenTypeService TokenType = "service" // 服务令牌
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
