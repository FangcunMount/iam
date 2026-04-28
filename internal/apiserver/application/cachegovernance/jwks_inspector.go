package cachegovernance

import (
	"context"
	"fmt"

	jwksdomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
)

type jwksPublishSnapshotInspector struct {
	builder *jwksdomain.KeySetBuilder
}

// NewJWKSPublishSnapshotInspector 创建 JWKS 发布快照的只读状态读取器。
func NewJWKSPublishSnapshotInspector(builder *jwksdomain.KeySetBuilder) cacheinfra.FamilyInspector {
	return &jwksPublishSnapshotInspector{builder: builder}
}

func (i *jwksPublishSnapshotInspector) Descriptor() cacheinfra.FamilyDescriptor {
	descriptor, ok := cacheinfra.GetFamily(cacheinfra.FamilyAuthnJWKSPublishSnapshot)
	if !ok {
		return cacheinfra.FamilyDescriptor{
			Family:    cacheinfra.FamilyAuthnJWKSPublishSnapshot,
			Backend:   cacheinfra.BackendKindMemory,
			RedisType: cacheinfra.RedisDataTypeNone,
			Codec:     cacheinfra.ValueCodecKindMemoryObject,
		}
	}
	return descriptor
}

func (i *jwksPublishSnapshotInspector) Status(context.Context) (cacheinfra.FamilyStatus, error) {
	status := cacheinfra.FamilyStatus{
		Family:          cacheinfra.FamilyAuthnJWKSPublishSnapshot,
		Configured:      i.builder != nil,
		Healthy:         false,
		EntryCountKnown: false,
		Notes:           []string{},
	}
	if i.builder == nil {
		status.Notes = append(status.Notes, "JWKS 构建器未配置。")
		return status, nil
	}

	snapshot := i.builder.SnapshotStatus()
	status.Healthy = true
	if !snapshot.Cached {
		status.Notes = append(status.Notes, "尚未构建 JWKS 进程内快照。")
		return status, nil
	}

	status.EntryCountKnown = true
	status.Notes = append(status.Notes, fmt.Sprintf("当前快照包含 %d 个公钥。", snapshot.KeyCount))
	if snapshot.LastBuildTime != nil {
		status.Notes = append(status.Notes, fmt.Sprintf("最后构建时间: %s", snapshot.LastBuildTime.Format("2006-01-02 15:04:05 -0700 MST")))
	}
	if snapshot.CacheTag.ETag != "" {
		status.Notes = append(status.Notes, fmt.Sprintf("当前缓存标签: %s", snapshot.CacheTag.ETag))
	}
	return status, nil
}
