package cache

import cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"

// Family 表示 IAM 中一个稳定的缓存族标识。
type Family = cachegovernance.Family

const (
	FamilyAuthnRefreshToken        = cachegovernance.FamilyAuthnRefreshToken
	FamilyAuthnRevokedAccessToken  = cachegovernance.FamilyAuthnRevokedAccessToken
	FamilyAuthnSession             = cachegovernance.FamilyAuthnSession
	FamilyAuthnUserSessionIndex    = cachegovernance.FamilyAuthnUserSessionIndex
	FamilyAuthnAccountSessionIndex = cachegovernance.FamilyAuthnAccountSessionIndex
	FamilyAuthnLoginOTP            = cachegovernance.FamilyAuthnLoginOTP
	FamilyAuthnLoginOTPSendGate    = cachegovernance.FamilyAuthnLoginOTPSendGate
	FamilyIDPWechatAccessToken     = cachegovernance.FamilyIDPWechatAccessToken
	FamilyIDPWechatSDK             = cachegovernance.FamilyIDPWechatSDK
	FamilyAuthnJWKSPublishSnapshot = cachegovernance.FamilyAuthnJWKSPublishSnapshot
)

// BackendKind 表示缓存后端类型。
type BackendKind = cachegovernance.BackendKind

const (
	BackendKindRedis  = cachegovernance.BackendKindRedis
	BackendKindMemory = cachegovernance.BackendKindMemory
)

// DataRole 表示缓存族承载的数据角色。
type DataRole = cachegovernance.DataRole

const (
	DataRoleAuthoritativeState = cachegovernance.DataRoleAuthoritativeState
	DataRoleMarkerState        = cachegovernance.DataRoleMarkerState
	DataRoleRemoteTokenCache   = cachegovernance.DataRoleRemoteTokenCache
	DataRoleDerivedSnapshot    = cachegovernance.DataRoleDerivedSnapshot
)

// GovernanceCapability 表示第一版治理面对 family 暴露的能力。
type GovernanceCapability = cachegovernance.GovernanceCapability

const (
	GovernanceCapabilityInspect = cachegovernance.GovernanceCapabilityInspect
)
