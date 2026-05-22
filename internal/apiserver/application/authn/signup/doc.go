// Package signup 实现 AuthN 登录身份开通（SignUp / Provision）用例。
//
// 与 REST /authn/signups、pkg/sdk/auth/signup 对齐；覆盖微信小程序开通与
// 内部 mock C 端 ensure 等入口。固定流水线：
//
//	PrepareStep → ResolveUserStep → EnsureLoginIdentityStep → EnsureCredentialStep
//
// 与 application/authn/session 的边界：
//
//   - login：已有 LoginIdentity + Credential，认证并签发 TokenPair
//   - signup（本包）：创建或复用 User、LoginIdentity，按需创建长期 Credential
package signup
