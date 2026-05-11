// Package jwks 提供 Authn 应用用例，用于 JWKS 发布和 JWKS 密钥管理。
//
// 这个包拥有请求/响应 DTO 和传输层使用的端口。
// 签名密钥生命周期规则、PEM/RSA 处理、JWK 材料构造和缓存快照由 infra/token/keyset 实现。
//
// Authn 领域包不能依赖 JWKS 概念。
// JWKS 是一个公共的 token 协议边界，仅通过应用和传输层暴露。
package jwks
