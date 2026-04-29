package cache

import cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"

// FamilyStatus 描述某个缓存族当前的只读状态。
type FamilyStatus = cachegovernance.FamilyStatus

// RuntimeStatus 描述某个缓存后端当前的运行状态。
type RuntimeStatus = cachegovernance.RuntimeStatus
