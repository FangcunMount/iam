package token

import "time"

// Capabilities 是 AuthN Token 领域对外提供的窄角色能力集合。
type Capabilities struct {
	TokenSetMinter     TokenSetMinter
	ServiceTokenIssuer ServiceTokenIssuer
	Refresher          Refresher
	Verifier           Verifier
	Revoker            Revoker
}

// Dependencies 是 Token 领域服务所需的领域协作者与技术端口。
type Dependencies struct {
	AccessTokenCodec      AccessTokenCodec
	TokenStore            Store
	SessionLoader         SessionLoader
	SessionRevoker        SessionRevoker
	SessionExtender       SessionExtender
	SessionRefreshExpirer SessionRefreshExpirer
	AdmissionPolicy       AdmissionPolicy
	RefreshClaimsCodec    RefreshClaimsCodec
	AccessTTL             time.Duration
}

// NewCapabilities 装配 Token 的签发、刷新、验证和撤销领域能力。
func NewCapabilities(deps Dependencies) Capabilities {
	// 创建用户令牌颁发器
	minter := newTokenSetMinter(deps.AccessTokenCodec, deps.SessionRefreshExpirer, deps.RefreshClaimsCodec, deps.AccessTTL)
	// 创建服务令牌颁发器
	return Capabilities{
		// 创建用户令牌颁发器
		TokenSetMinter: minter,
		// 创建服务令牌颁发器
		ServiceTokenIssuer: newServiceTokenIssuer(deps.AccessTokenCodec, deps.AccessTTL),
		// 创建刷新器
		Refresher: newRefresher(
			minter,
			deps.TokenStore,
			deps.SessionLoader,
			deps.SessionRevoker,
			deps.SessionExtender,
			deps.AdmissionPolicy,
			deps.RefreshClaimsCodec,
		),
		// 创建验证器
		Verifier: newVerifier(deps.AccessTokenCodec, deps.TokenStore, deps.SessionLoader, deps.AdmissionPolicy),
		// 创建撤销器
		Revoker: newRevoker(deps.AccessTokenCodec, deps.TokenStore, deps.SessionRevoker),
	}
}
