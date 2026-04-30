package authn

// Context keys 用于在 Gin Context 中存储认证信息
// 使用常量可以避免字符串拼写错误，提高代码安全性
const (
	// ContextKeyUserID 当前认证用户的 ID
	ContextKeyUserID = "user_id"

	// ContextKeyAccountID 当前认证账户的 ID
	ContextKeyAccountID = "account_id"

	// ContextKeyTenantID 当前认证租户的 ID（tenant domain）
	ContextKeyTenantID = "tenant_id"

	// ContextKeyTokenID 当前使用的 Token ID
	ContextKeyTokenID = "token_id"

	// ContextKeyClaims 当前认证请求的 Claims
	ContextKeyClaims = "claims"
)
