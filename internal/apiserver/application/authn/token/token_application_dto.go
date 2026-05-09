package token

import "time"

// IssueServiceTokenRequest 服务令牌签发请求。
type IssueServiceTokenRequest struct {
	Subject    string
	Audience   []string
	TTL        time.Duration
	Attributes map[string]string
}

// TokenIssueResult 令牌签发结果 DTO。
type TokenIssueResult struct {
	TokenPair *TokenPair
}

// TokenRefreshResult 令牌刷新结果 DTO。
type TokenRefreshResult struct {
	TokenPair *TokenPair
}

// VerifyTokenRequest 令牌验证请求 DTO。
type VerifyTokenRequest struct {
	AccessToken      string
	ExpectedIssuer   string
	ExpectedAudience []string
}

// TokenVerifyResult 令牌验证结果 DTO。
type TokenVerifyResult struct {
	Valid       bool
	Claims      *TokenClaims
	FailureCode int
}
