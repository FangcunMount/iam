package suggest

import "fmt"

// ProfileIndexOperation 表达索引变更操作。
type ProfileIndexOperation uint8

const (
	ProfileIndexUpsert ProfileIndexOperation = iota + 1
	ProfileIndexDelete
)

// ProfileIndexMutation 显式表达 Upsert 或 Delete。
type ProfileIndexMutation struct {
	Operation ProfileIndexOperation
	ProfileID int64
	Term      ProfileSearchTerm // Upsert 时使用；Delete 时忽略
}

// NewProfileIndexUpsert 构造 upsert 变更。
func NewProfileIndexUpsert(term ProfileSearchTerm) (ProfileIndexMutation, error) {
	if term.ProfileID <= 0 {
		return ProfileIndexMutation{}, fmt.Errorf("profile id required for upsert")
	}
	if term.DisplayName == "" {
		return ProfileIndexMutation{}, fmt.Errorf("display name required for upsert")
	}
	return ProfileIndexMutation{
		Operation: ProfileIndexUpsert,
		ProfileID: term.ProfileID,
		Term:      term,
	}, nil
}

// NewProfileIndexDelete 构造 delete 变更。
func NewProfileIndexDelete(profileID int64) (ProfileIndexMutation, error) {
	if profileID <= 0 {
		return ProfileIndexMutation{}, fmt.Errorf("profile id required for delete")
	}
	return ProfileIndexMutation{
		Operation: ProfileIndexDelete,
		ProfileID: profileID,
	}, nil
}
