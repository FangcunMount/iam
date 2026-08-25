// Package token implements AuthN token application use cases.
//
// 组合根通过 Capabilities 输出彼此独立的令牌用例能力：会话建立、刷新、
// 在线验证、撤销和服务令牌签发。调用方只依赖实际使用的窄接口。
//
// revoker 是 token 包内部的撤销门面，实际职责委托给更小的用例组件。
//
//	revoker
//	  -> revokeBearerToken	撤销令牌和关联会话
//
// verifier 是 token 包内部的验证门面，实际职责委托给更小的用例组件。
//
//	verifier
//	  -> verifyToken	验证令牌
//
// refresher 是 token 包内部的刷新门面，实际职责委托给更小的用例组件。
//
//	refresher
//	  -> refreshToken	刷新令牌
//	  -> RevokeRefreshToken	删除刷新令牌

package token
