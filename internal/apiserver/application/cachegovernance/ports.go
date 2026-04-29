package cachegovernance

import "context"

// FamilyInspector 负责读取某个缓存族的只读状态。
type FamilyInspector interface {
	Descriptor() FamilyDescriptor
	Status(ctx context.Context) (FamilyStatus, error)
}

// RuntimeStatusReader 负责读取某类缓存后端的聚合运行状态。
type RuntimeStatusReader interface {
	Backend() BackendKind
	Status(ctx context.Context) (RuntimeStatus, error)
}
