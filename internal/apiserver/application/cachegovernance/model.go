package cachegovernance

import cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"

// FamilyView 表示某个缓存族的静态描述与运行状态。
type FamilyView struct {
	Descriptor cacheinfra.FamilyDescriptor
	Status     cacheinfra.FamilyStatus
}

// Overview 表示 IAM 缓存治理面的只读总览。
type Overview struct {
	RuntimeStatuses []cacheinfra.RuntimeStatus
	Families        []FamilyView
}
