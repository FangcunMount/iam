// Package token implements AuthN token application use cases.
//
// The package is organized around these collaborations:
//
// TokenApplicationService 是 token 应用服务的接口，实际职责委托给更小的用例组件。
//
//	TokenApplicationService
//	  -> sessionTokenIssuer	签发用户会话令牌
//	  -> serviceTokenIssuer	签发服务间访问令牌
//	  -> refresher	刷新令牌
//	  -> verifier	验证令牌
//	  -> revoker	撤销令牌和关联会话
//
// issuer 是 token 包内部的签发门面，实际职责委托给更小的用例组件。
//	issuer
//	  -> sessionTokenIssuer	签发用户会话令牌
//	  -> serviceTokenIssuer	签发服务间访问令牌
//
// revoker 是 token 包内部的撤销门面，实际职责委托给更小的用例组件。
//	revoker
//	  -> revokeAccessToken	撤销令牌和关联会话
//
// verifier 是 token 包内部的验证门面，实际职责委托给更小的用例组件。
//	verifier
//	  -> verifyAccessToken	验证令牌
//
// refresher 是 token 包内部的刷新门面，实际职责委托给更小的用例组件。
//	refresher
//	  -> refreshToken	刷新令牌
//	  -> revokeRefreshToken	删除刷新令牌

package token
