package cachegovernance

import (
	"context"
	"fmt"

	jwksapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
)

type jwksPublishSnapshotInspector struct {
	reporter jwksapp.SnapshotReporter
}

// NewJWKSPublishSnapshotInspector 创建 JWKS 发布快照的只读状态读取器。
func NewJWKSPublishSnapshotInspector(reporter jwksapp.SnapshotReporter) FamilyInspector {
	return &jwksPublishSnapshotInspector{reporter: reporter}
}

func (i *jwksPublishSnapshotInspector) Descriptor() FamilyDescriptor {
	descriptor, ok := GetFamily(FamilyAuthnJWKSPublishSnapshot)
	if !ok {
		return FamilyDescriptor{
			Family:    FamilyAuthnJWKSPublishSnapshot,
			Backend:   BackendKindMemory,
			RedisType: RedisDataTypeNone,
			Codec:     ValueCodecKindMemoryObject,
		}
	}
	return descriptor
}

func (i *jwksPublishSnapshotInspector) Status(context.Context) (FamilyStatus, error) {
	status := FamilyStatus{
		Family:          FamilyAuthnJWKSPublishSnapshot,
		Configured:      i.reporter != nil,
		Healthy:         false,
		EntryCountKnown: false,
		Notes:           []string{},
	}
	if i.reporter == nil {
		status.Notes = append(status.Notes, "JWKS 构建器未配置。")
		return status, nil
	}

	snapshot := i.reporter.SnapshotStatus()
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
