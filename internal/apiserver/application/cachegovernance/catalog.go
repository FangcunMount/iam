package cachegovernance

import cachemodel "github.com/FangcunMount/iam/v3/internal/apiserver/cache"

// Family 表示 IAM 中一个稳定的缓存族标识。
type Family = cachemodel.Family

const (
	FamilyAuthnRefreshToken              = cachemodel.FamilyAuthnRefreshToken
	FamilyAuthnRevokedAccessToken        = cachemodel.FamilyAuthnRevokedAccessToken
	FamilyAuthnSession                   = cachemodel.FamilyAuthnSession
	FamilyAuthnUserSessionIndex          = cachemodel.FamilyAuthnUserSessionIndex
	FamilyAuthnLoginIdentitySessionIndex = cachemodel.FamilyAuthnLoginIdentitySessionIndex
	FamilyAuthnChallenge                 = cachemodel.FamilyAuthnChallenge
	FamilyAuthnLoginOTPSendGate          = cachemodel.FamilyAuthnLoginOTPSendGate
	FamilyAuthnLoginOTPSendQuota         = cachemodel.FamilyAuthnLoginOTPSendQuota
	FamilyIDPWechatAccessToken           = cachemodel.FamilyIDPWechatAccessToken
	FamilyIDPWechatSDK                   = cachemodel.FamilyIDPWechatSDK
	FamilyAuthnJWKSPublishSnapshot       = cachemodel.FamilyAuthnJWKSPublishSnapshot
	FamilySuggestRedisRateLimit          = cachemodel.FamilySuggestRedisRateLimit
	FamilySuggestMemoryRateLimit         = cachemodel.FamilySuggestMemoryRateLimit
)

// BackendKind 表示缓存后端类型。
type BackendKind = cachemodel.BackendKind

const (
	BackendKindRedis  = cachemodel.BackendKindRedis
	BackendKindMemory = cachemodel.BackendKindMemory
)

// DataRole 表示缓存族承载的数据角色。
type DataRole = cachemodel.DataRole

const (
	DataRoleAuthoritativeState = cachemodel.DataRoleAuthoritativeState
	DataRoleMarkerState        = cachemodel.DataRoleMarkerState
	DataRoleRemoteTokenCache   = cachemodel.DataRoleRemoteTokenCache
	DataRoleDerivedSnapshot    = cachemodel.DataRoleDerivedSnapshot
)

// GovernanceCapability 表示第一版治理面对 family 暴露的能力。
type GovernanceCapability = cachemodel.GovernanceCapability

const (
	GovernanceCapabilityInspect = cachemodel.GovernanceCapabilityInspect
)

// RedisDataType 表示治理面视角下的 Redis 数据结构。
type RedisDataType = cachemodel.RedisDataType

const (
	RedisDataTypeNone   = cachemodel.RedisDataTypeNone
	RedisDataTypeString = cachemodel.RedisDataTypeString
	RedisDataTypeHash   = cachemodel.RedisDataTypeHash
	RedisDataTypeSet    = cachemodel.RedisDataTypeSet
	RedisDataTypeZSet   = cachemodel.RedisDataTypeZSet
)

// ValueCodecKind 表示 family 的 value 编码方式。
type ValueCodecKind = cachemodel.ValueCodecKind

const (
	ValueCodecKindMemoryObject = cachemodel.ValueCodecKindMemoryObject
	ValueCodecKindJSON         = cachemodel.ValueCodecKindJSON
	ValueCodecKindMarker       = cachemodel.ValueCodecKindMarker
	ValueCodecKindString       = cachemodel.ValueCodecKindString
	ValueCodecKindLeaseToken   = cachemodel.ValueCodecKindLeaseToken
)

// FamilyPolicy 描述某个缓存族的静态策略。
type FamilyPolicy = cachemodel.FamilyPolicy

// FamilyDescriptor 描述一个缓存族的治理元数据。
type FamilyDescriptor = cachemodel.FamilyDescriptor

// Families 返回当前 IAM 缓存目录快照。
func Families() []FamilyDescriptor {
	return cachemodel.Families()
}

// GetFamily 返回指定 family 的静态描述。
func GetFamily(family Family) (FamilyDescriptor, bool) {
	return cachemodel.GetFamily(family)
}
