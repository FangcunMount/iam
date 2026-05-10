package response

// SignupResult 登录身份开通结果响应。
type SignupResult struct {
	UserID          uint64            `json:"userId"`
	UserName        string            `json:"userName"`
	Phone           string            `json:"phone"`
	Email           string            `json:"email,omitempty"`
	LoginIdentityID uint64            `json:"loginIdentityId"`
	Credential      *SignupCredential `json:"credential"`
	IsNewUser       bool              `json:"isNewUser"`
	IsNewIdentity   bool              `json:"isNewIdentity"`
}

type SignupCredential struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
}

// EnsureMockConsumerResult 内部 mock C 端 ensure 结果。
type EnsureMockConsumerResult struct {
	UserID          string `json:"user_id"`
	LoginIdentityID string `json:"login_identity_id"`
	LoginID         string `json:"login_id"`
	IsNewUser       bool   `json:"is_new_user"`
	IsNewIdentity   bool   `json:"is_new_identity"`
}
