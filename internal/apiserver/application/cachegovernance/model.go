package cachegovernance

// FamilyView 表示某个缓存族的静态描述与运行状态。
type FamilyView struct {
	Descriptor FamilyDescriptor
	Status     FamilyStatus
}

// Overview 表示 IAM 缓存治理面的只读总览。
type Overview struct {
	RuntimeStatuses []RuntimeStatus
	Families        []FamilyView
}

// FamilyStatus 描述某个缓存族当前的只读状态。
type FamilyStatus struct {
	Family          Family
	Configured      bool
	Healthy         bool
	EntryCountKnown bool
	Notes           []string
}

// RuntimeStatus 描述某个缓存后端当前的运行状态。
type RuntimeStatus struct {
	Backend    BackendKind
	Configured bool
	Healthy    bool
	Notes      []string
}
