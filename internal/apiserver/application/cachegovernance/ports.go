package cachegovernance

import "context"

// FamilyInspector 负责读取某个缓存族的只读状态。
type FamilyInspector interface {
	Descriptor() FamilyDescriptor
	Status(ctx context.Context) (FamilyStatus, error)
}
