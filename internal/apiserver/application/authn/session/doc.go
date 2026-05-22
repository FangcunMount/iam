// Package session 编排 AuthN 用户会话用例门面。
//
//   - Login：委托 signin.SignIn（Method → Proof → Authenticate → IssueToken）
//   - RenewSession：委托 token.RefreshToken（HTTP 路由仍为 /refresh_token）
//   - Logout：委托 token 撤销
//
// 管理员撤销见 Revoker。登录实现细节与依赖装配在 signin 包，不在本门面 Dependencies 中重复展开。
package session
