package policy

import (
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// PolicyVersion 策略版本（用于缓存失效通知）
type PolicyVersion struct {
	ID        PolicyVersionID
	TenantID  tenant.ID // 租户ID
	Version   int64     // 版本号
	ChangedBy string    // 变更人
	Reason    string    // 变更原因
}

// NewPolicyVersion 创建新版本
func NewPolicyVersion(tenantID string, version int64, opts ...PolicyVersionOption) PolicyVersion {
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		tenantIDValue = tenant.ID(strings.TrimSpace(tenantID))
	}
	pv := PolicyVersion{
		TenantID: tenantIDValue,
		Version:  version,
	}
	for _, opt := range opts {
		opt(&pv)
	}
	return pv
}

// PolicyVersionOption 版本选项
type PolicyVersionOption func(*PolicyVersion)

func WithID(id PolicyVersionID) PolicyVersionOption { return func(pv *PolicyVersion) { pv.ID = id } }
func WithChangedBy(by string) PolicyVersionOption {
	return func(pv *PolicyVersion) { pv.ChangedBy = by }
}
func WithReason(reason string) PolicyVersionOption {
	return func(pv *PolicyVersion) { pv.Reason = reason }
}

func (pv PolicyVersion) TenantIDString() string {
	return pv.TenantID.String()
}

// PolicyVersionID 策略版本ID值对象
type PolicyVersionID meta.ID

func (id PolicyVersionID) Uint64() uint64 {
	return meta.ID(id).Uint64()
}

func (id PolicyVersionID) String() string {
	return meta.ID(id).String()
}
