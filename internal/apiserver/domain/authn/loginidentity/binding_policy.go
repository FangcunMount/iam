package loginidentity

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// BindingDecision 描述 ProviderKey 绑定或复用决策。
type BindingDecision int

const (
	BindingCreate BindingDecision = iota
	BindingReuse
	BindingConflictOtherUser
	BindingInactiveSameUser
)

// BindingRequest 是 ProviderKey 绑定策略的确定性输入。
type BindingRequest struct {
	RequestUserID meta.ID
	Existing      *LoginIdentity
}

// AssessBinding 根据已存在的登录身份判断绑定动作。
func AssessBinding(req BindingRequest) BindingDecision {
	if req.Existing == nil {
		return BindingCreate
	}
	if req.Existing.UserID != req.RequestUserID {
		return BindingConflictOtherUser
	}
	if !req.Existing.IsActive() {
		return BindingInactiveSameUser
	}
	return BindingReuse
}

// GlobalIdentifierDecision 描述全局标识符预检结果。
type GlobalIdentifierDecision int

const (
	GlobalIdentifierAvailable GlobalIdentifierDecision = iota
	GlobalIdentifierOwnedByOtherUser
)

// AssessGlobalIdentifierAvailability 判断全局标识符是否可被当前用户占用。
func AssessGlobalIdentifierAvailability(requestUserID meta.ID, existing *LoginIdentity) GlobalIdentifierDecision {
	if existing == nil || existing.UserID == requestUserID {
		return GlobalIdentifierAvailable
	}
	return GlobalIdentifierOwnedByOtherUser
}

// CanonicalClaimDecision 描述创建时 canonical global identifier 的处理方式。
type CanonicalClaimDecision int

const (
	CanonicalClaimStoreOnNewRow CanonicalClaimDecision = iota
	CanonicalClaimKeepExistingAnchor
	CanonicalClaimTransferFromInactiveAnchor
	CanonicalClaimConflictOtherUser
)

// AssessCanonicalClaim 根据已锁定的 canonical 行判断创建策略。
func AssessCanonicalClaim(requestUserID meta.ID, existing *LoginIdentity) CanonicalClaimDecision {
	if existing == nil {
		return CanonicalClaimStoreOnNewRow
	}
	if existing.UserID != requestUserID {
		return CanonicalClaimConflictOtherUser
	}
	if existing.IsActive() {
		return CanonicalClaimKeepExistingAnchor
	}
	return CanonicalClaimTransferFromInactiveAnchor
}

// CanonicalReplacement 描述解绑时 canonical 全局标识符的转移计划。
type CanonicalReplacement struct {
	TargetID         meta.ID
	ReplacementID    meta.ID
	GlobalIdentifier string
}

// SelectCanonicalReplacement 从同一用户已加锁身份中选择 canonical 接替者。
// identities 必须已按稳定顺序排列（与仓储锁定顺序一致）。
func SelectCanonicalReplacement(target *LoginIdentity, identities []*LoginIdentity) *CanonicalReplacement {
	if target == nil || target.GlobalIdentifier == "" {
		return nil
	}
	for _, candidate := range identities {
		if candidate == nil || candidate.ID == target.ID {
			continue
		}
		if candidate.Provider == target.Provider && candidate.IsActive() {
			return &CanonicalReplacement{
				TargetID:         target.ID,
				ReplacementID:    candidate.ID,
				GlobalIdentifier: target.GlobalIdentifier,
			}
		}
	}
	return nil
}
