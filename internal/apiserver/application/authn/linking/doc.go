// Package linking 管理已认证用户的 LoginIdentity 绑定与解绑。
//
// 绑定采用模板方法（非策略模式）：
//
//	Linker.Link → linker.link（固定骨架）
//	  → LinkLoginIdentityInput.prepareLink（按 provider 多态，见 link_*.go）
//	  → ensureGlobalIdentifier（可选）
//	  → ensureProviderKey（查重 / 复用 / 创建，见 link_ensure.go）
//
// prepare 仅依赖 linkPrepareDeps，不依赖 *linker。对外契约见 interface.go。
// 手机号发码由 transport 直调 challenge（SceneLinkPhoneOTP）。
package linking
