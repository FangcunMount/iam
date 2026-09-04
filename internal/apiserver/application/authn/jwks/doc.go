// Package jwks 提供 Authn 的公共 JWK Set 发布用例。
//
// 签名密钥管理与生命周期位于 application/authn/signingkey；
// PEM/RSA 处理、JWK 材料构造和缓存快照由基础设施实现。
//
// Authn 领域包不能依赖 JWKS 概念。
// JWKS 是一个公共的 token 协议边界，仅通过应用和传输层暴露。
package jwks
