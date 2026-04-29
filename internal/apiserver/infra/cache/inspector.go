package cache

import cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"

// FamilyInspector 负责读取某个缓存族的只读状态。
type FamilyInspector = cachegovernance.FamilyInspector

// RuntimeStatusReader 负责读取某类缓存后端的聚合运行状态。
type RuntimeStatusReader = cachegovernance.RuntimeStatusReader
