// Package sdk 提供 IAM 官方 Go SDK 的统一入口。
//
// 对外稳定入口固定为：
//   - sdk.Client / sdk.NewClient
//   - sdk.WithRequestID / sdk.WithTraceID / sdk.GetRequestID / sdk.GetTraceID
//   - pkg/sdk/config、pkg/sdk/auth/client、pkg/sdk/auth/loginv2、pkg/sdk/auth/jwks、pkg/sdk/auth/verifier、pkg/sdk/auth/serviceauth
//   - pkg/sdk/authz、pkg/sdk/identity、pkg/sdk/idp、pkg/sdk/errors
package sdk
