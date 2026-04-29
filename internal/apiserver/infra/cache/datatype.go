package cache

import cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"

// RedisDataType 表示治理面视角下的 Redis 数据结构。
type RedisDataType = cachegovernance.RedisDataType

const (
	RedisDataTypeNone   = cachegovernance.RedisDataTypeNone
	RedisDataTypeString = cachegovernance.RedisDataTypeString
	RedisDataTypeHash   = cachegovernance.RedisDataTypeHash
	RedisDataTypeSet    = cachegovernance.RedisDataTypeSet
	RedisDataTypeZSet   = cachegovernance.RedisDataTypeZSet
)

// ValueCodecKind 表示 family 的 value 编码方式。
type ValueCodecKind = cachegovernance.ValueCodecKind

const (
	ValueCodecKindMemoryObject = cachegovernance.ValueCodecKindMemoryObject
	ValueCodecKindJSON         = cachegovernance.ValueCodecKindJSON
	ValueCodecKindMarker       = cachegovernance.ValueCodecKindMarker
	ValueCodecKindString       = cachegovernance.ValueCodecKindString
	ValueCodecKindLeaseToken   = cachegovernance.ValueCodecKindLeaseToken
)
