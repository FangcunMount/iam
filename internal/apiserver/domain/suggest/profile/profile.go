package profile

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// SuggestibleProfile 是受约束的 Suggest 读模型。
type SuggestibleProfile struct {
	id               int64
	displayName      string
	mobiles          []string
	weight           int
	orgID            int64
	ownerOperatorIDs []int64
}

// New 构建并规范化 SuggestibleProfile；非正 ID 或空名称返回错误。
func New(
	profileID int64,
	displayName string,
	mobiles []string,
	weight int,
	orgID int64,
	ownerOperatorIDs []int64,
) (SuggestibleProfile, error) {
	if profileID <= 0 {
		return SuggestibleProfile{}, fmt.Errorf("profile id must be positive")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return SuggestibleProfile{}, fmt.Errorf("display name required")
	}
	return rawProjection(profileID, displayName, mobiles, weight, orgID, ownerOperatorIDs), nil
}

// MustNew 同 New，校验失败时 panic；仅供测试与静态 fixture。
func MustNew(
	profileID int64,
	displayName string,
	mobiles []string,
	weight int,
	orgID int64,
	ownerOperatorIDs []int64,
) SuggestibleProfile {
	p, err := New(profileID, displayName, mobiles, weight, orgID, ownerOperatorIDs)
	if err != nil {
		panic(err)
	}
	return p
}

// RawProjection 在 loader/refresh 边界保留原始行语义（含 tombstone 空名称），不做校验。
func RawProjection(
	profileID int64,
	displayName string,
	mobiles []string,
	weight int,
	orgID int64,
	ownerOperatorIDs []int64,
) SuggestibleProfile {
	return rawProjection(profileID, displayName, mobiles, weight, orgID, ownerOperatorIDs)
}

func rawProjection(
	profileID int64,
	displayName string,
	mobiles []string,
	weight int,
	orgID int64,
	ownerOperatorIDs []int64,
) SuggestibleProfile {
	cleanMobiles := make([]string, 0, len(mobiles))
	for _, mobile := range mobiles {
		mobile = strings.TrimSpace(mobile)
		if mobile == "" {
			continue
		}
		cleanMobiles = append(cleanMobiles, mobile)
	}
	return SuggestibleProfile{
		id:               profileID,
		displayName:      strings.TrimSpace(displayName),
		mobiles:          cleanMobiles,
		weight:           weight,
		orgID:            orgID,
		ownerOperatorIDs: uniqueInt64(ownerOperatorIDs),
	}
}

func uniqueInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (p SuggestibleProfile) ID() int64               { return p.id }
func (p SuggestibleProfile) DisplayName() string       { return p.displayName }
func (p SuggestibleProfile) Weight() int               { return p.weight }
func (p SuggestibleProfile) OrgID() int64              { return p.orgID }
func (p SuggestibleProfile) Mobiles() []string         { return append([]string(nil), p.mobiles...) }
func (p SuggestibleProfile) OwnerOperatorIDs() []int64 { return append([]int64(nil), p.ownerOperatorIDs...) }

// PrimaryMobile 返回第一个手机号；无则空串。
func (p SuggestibleProfile) PrimaryMobile() string {
	if len(p.mobiles) == 0 {
		return ""
	}
	return p.mobiles[0]
}

// VisibilityResource 供 scope 过滤使用。
func (p SuggestibleProfile) VisibilityResource() visibility.Resource {
	return visibility.Resource{
		ProfileID:        p.id,
		OrgID:            p.orgID,
		OwnerOperatorIDs: append([]int64(nil), p.ownerOperatorIDs...),
	}
}
