package admission

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// Subject 表示待准入的认证主体与登录身份组合。
//
// Authenticator 已经证明请求控制了某个 LoginIdentity；AdmissionPolicy
// 进一步确认该 LoginIdentity 仍属于声明的 User，且双方当前状态允许建立
// 或继续维持认证状态。它不是 AuthZ 的资源访问主体。
type Subject struct {
	UserID          meta.ID
	LoginIdentityID meta.ID
}
