package refreshindex

import (
	"fmt"

	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

// ChangeKind 表达索引投影变更种类。
type ChangeKind uint8

const (
	ChangeUpsert ChangeKind = iota + 1
	ChangeDelete
)

// ProjectionChange 显式表达 Upsert 或 Delete。
type ProjectionChange struct {
	kind      ChangeKind
	profileID int64
	profile   domainprofile.SuggestibleProfile
}

// Upsert 构造 upsert 变更。
func Upsert(p domainprofile.SuggestibleProfile) (ProjectionChange, error) {
	if _, err := domainprofile.New(
		p.ID(), p.DisplayName(), p.Mobiles(), p.Weight(), p.OrgID(), p.OwnerOperatorIDs(),
	); err != nil {
		return ProjectionChange{}, err
	}
	return ProjectionChange{kind: ChangeUpsert, profileID: p.ID(), profile: p}, nil
}

// Delete 构造 delete 变更。
func Delete(profileID int64) (ProjectionChange, error) {
	if profileID <= 0 {
		return ProjectionChange{}, fmt.Errorf("profile id required for delete")
	}
	return ProjectionChange{kind: ChangeDelete, profileID: profileID}, nil
}

func (c ProjectionChange) Kind() ChangeKind                      { return c.kind }
func (c ProjectionChange) ProfileID() int64                      { return c.profileID }
func (c ProjectionChange) Profile() domainprofile.SuggestibleProfile { return c.profile }
