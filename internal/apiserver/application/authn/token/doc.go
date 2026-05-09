// Package token implements AuthN token application use cases.
//
// The package is organized around these collaborations:
//
// TokenApplicationService 是 token 应用服务的接口，实际职责委托给更小的用例组件。
//
//		TokenApplicationService
//		  -> serviceTokenIssuer	签发服务令牌
//		  -> refresher	刷新访问令牌和刷新令牌
//		  -> verifier	验证访问令牌
//		  -> accessTokenRevoker	撤销访问令牌
//
//	 issuer 是 token 包内部的门面，装配并负责驱动以下组件：
//		issuer
//		  -> sessionTokenIssuer	签发用户会话令牌
//		  -> serviceTokenIssuer 签发服务令牌
//		  -> accessTokenRevoker 撤销访问令牌
//
//	 sessionTokenIssuer 是 token 包内部的门面，装配并负责驱动以下组件：
//		sessionTokenIssuer
//		  -> SessionManager.Create 创建会话
//		  -> sessionTokenPairIssuer.IssueTokenPair 签发 token pair
//
//	 sessionTokenPairIssuer 是 token 包内部的门面，装配并负责驱动以下组件：
//		sessionTokenPairIssuer
//		  -> AccessTokenCodec.IssueAccessToken 签发访问令牌
//		  -> Store.SaveRefreshToken 保存刷新令牌
package token
