// Package signin 编排凭据登录
//
// 登录主线固定为：
//  1. MethodRegistry 根据 AuthMethod 选择登录方式并校验 method payload；
//  2. ProofFactory 根据 CredentialKind 构造领域 AuthCredential；
//  3. 领域 Authenticator 完成凭据认证并返回 Principal；
//  4. AuthenticationGrantIssuer 为 Principal 建立完整在线认证结果并返回 TokenPair。
//
// 请求上下文规范：
// TenantID、RemoteIP、UserAgent 必须由 transport / compatibility 层写入
// LoginRequest 顶层字段。method.Payload 只保存具体登录方式自己的字段，
// 不承载公共请求上下文。
//
// 新增登录方式时必须同步扩展：
// method.AuthMethod、method.LoginMethod、method.Payload、proof.Builder、
// proof.Factory/assembler 注册、领域 authentication 策略，以及公开协议需要的
// compatibility wire payload 解析。漏掉 proof.Builder 注册会在运行时得到
// ErrProofBuildFailed / unsupported credential kind。
package signin
