package testhelpers

import (
	"context"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"sync"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	assignment "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	role "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	profile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	user "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	wechatapp "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// AssignmentRepoStub is a simple stub for assignment.Repository used in tests.
type AssignmentRepoStub struct {
	Assignments []*assignment.Assignment
	Err         error
}

func (s *AssignmentRepoStub) Create(ctx context.Context, a *assignment.Assignment) error {
	return nil
}
func (s *AssignmentRepoStub) Delete(ctx context.Context, id assignment.AssignmentID) error {
	return nil
}
func (s *AssignmentRepoStub) DeleteBySubjectAndRole(ctx context.Context, subjectType assignment.SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string) error {
	return nil
}
func (s *AssignmentRepoStub) FindByID(ctx context.Context, id assignment.AssignmentID) (*assignment.Assignment, error) {
	return nil, nil
}
func (s *AssignmentRepoStub) ListBySubject(ctx context.Context, subjectType assignment.SubjectType, subjectID meta.ID, tenantID string) ([]*assignment.Assignment, error) {
	return s.Assignments, s.Err
}
func (s *AssignmentRepoStub) ListBySubjectForUpdate(ctx context.Context, subjectType assignment.SubjectType, subjectID meta.ID, tenantID string) ([]*assignment.Assignment, error) {
	return s.Assignments, s.Err
}
func (s *AssignmentRepoStub) ListByRole(ctx context.Context, roleID meta.ID, tenantID string) ([]*assignment.Assignment, error) {
	return nil, s.Err
}

// RoleRepoStub is a minimal stub for role.Repository used in tests.
type RoleRepoStub struct {
	R   *role.Role
	Err error
}

func (s *RoleRepoStub) Create(ctx context.Context, r *role.Role) error { return nil }
func (s *RoleRepoStub) Update(ctx context.Context, r *role.Role) error { return nil }
func (s *RoleRepoStub) Delete(ctx context.Context, id meta.ID) error   { return nil }
func (s *RoleRepoStub) FindByID(ctx context.Context, id meta.ID) (*role.Role, error) {
	return s.R, s.Err
}
func (s *RoleRepoStub) FindByIDForUpdate(ctx context.Context, id meta.ID) (*role.Role, error) {
	return s.R, s.Err
}
func (s *RoleRepoStub) FindByName(ctx context.Context, tenantID, name string) (*role.Role, error) {
	return nil, nil
}
func (s *RoleRepoStub) List(ctx context.Context, tenantID string, offset, limit int) ([]*role.Role, int64, error) {
	return nil, 0, nil
}

// WechatRepoStub is a stub for wechatapp.Repository used in tests.
type WechatRepoStub struct {
	Existing *wechatapp.WechatApp
	Err      error
}

func (s *WechatRepoStub) Create(ctx context.Context, app *wechatapp.WechatApp) error { return nil }
func (s *WechatRepoStub) GetByID(ctx context.Context, id idutil.ID) (*wechatapp.WechatApp, error) {
	return nil, nil
}
func (s *WechatRepoStub) GetByAppID(ctx context.Context, appID string) (*wechatapp.WechatApp, error) {
	return s.Existing, s.Err
}
func (s *WechatRepoStub) List(ctx context.Context, filter wechatapp.ListFilter) ([]*wechatapp.WechatApp, error) {
	if s.Existing == nil {
		return nil, s.Err
	}
	return []*wechatapp.WechatApp{s.Existing}, s.Err
}
func (s *WechatRepoStub) Update(ctx context.Context, app *wechatapp.WechatApp) error { return nil }

// ProfileRepoStub is a stub for profile.Repository used in tests.
type ProfileRepoStub struct {
	Profile          *profile.Profile
	ProfilesByIDCard map[string]*profile.Profile
	FindErr          error
	FindCalls        int
	FindIDCardCalls  int
	UpdateArgs       []*profile.Profile
	mu               sync.Mutex
}

func (s *ProfileRepoStub) Create(ctx context.Context, c *profile.Profile) error { return nil }
func (s *ProfileRepoStub) FindByID(ctx context.Context, id meta.ID) (*profile.Profile, error) {
	s.mu.Lock()
	s.FindCalls++
	findErr := s.FindErr
	ch := s.Profile
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	if ch == nil {
		return nil, nil
	}
	return ch, nil
}
func (s *ProfileRepoStub) FindByIDs(ctx context.Context, ids []meta.ID) (map[meta.ID]*profile.Profile, error) {
	s.mu.Lock()
	s.FindCalls++
	findErr := s.FindErr
	ch := s.Profile
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	out := make(map[meta.ID]*profile.Profile, len(ids))
	if ch == nil {
		return out, nil
	}
	for _, id := range ids {
		out[id] = ch
	}
	return out, nil
}
func (s *ProfileRepoStub) FindByName(ctx context.Context, name string) (*profile.Profile, error) {
	return nil, s.FindErr
}
func (s *ProfileRepoStub) FindByIDCard(ctx context.Context, idCard meta.IDCard) (*profile.Profile, error) {
	s.mu.Lock()
	s.FindIDCardCalls++
	findErr := s.FindErr
	profilesByIDCard := s.ProfilesByIDCard
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	if profilesByIDCard != nil {
		if p := profilesByIDCard[idCard.String()]; p != nil {
			return p, nil
		}
	}
	return nil, perrors.WithCode(code.ErrIdentityProfileNotFound, "profile with id card(%s) not found", idCard.String())
}
func (s *ProfileRepoStub) FindListByName(ctx context.Context, name string) ([]*profile.Profile, error) {
	return nil, s.FindErr
}
func (s *ProfileRepoStub) FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) ([]*profile.Profile, error) {
	return nil, s.FindErr
}
func (s *ProfileRepoStub) Update(ctx context.Context, ch *profile.Profile) error {
	s.mu.Lock()
	s.UpdateArgs = append(s.UpdateArgs, ch)
	findErr := s.FindErr
	s.mu.Unlock()
	return findErr
}

// ----------------- User stubs -----------------

// UserRepoStub is a stub implementation of user.Repository for tests.
type UserRepoStub struct {
	UsersByID      map[uint64]*user.User
	UsersByPhone   map[string]*user.User
	FindErr        error
	PhoneErr       error
	UpdateArgs     []*user.User
	CreateArgs     []*user.User
	FindIDCalls    int
	FindPhoneCalls int
	mu             sync.Mutex
}

func NewUserRepoStub() *UserRepoStub {
	return &UserRepoStub{
		UsersByID:    make(map[uint64]*user.User),
		UsersByPhone: make(map[string]*user.User),
	}
}

func (s *UserRepoStub) Create(ctx context.Context, u *user.User) error {
	s.mu.Lock()
	s.CreateArgs = append(s.CreateArgs, u)
	findErr := s.FindErr
	if u != nil {
		s.UsersByID[u.ID.Uint64()] = u
		s.UsersByPhone[u.Phone.String()] = u
	}
	s.mu.Unlock()
	return findErr
}

func (s *UserRepoStub) FindByID(ctx context.Context, id meta.ID) (*user.User, error) {
	s.mu.Lock()
	s.FindIDCalls++
	findErr := s.FindErr
	u, ok := s.UsersByID[id.Uint64()]
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	if ok {
		if u != nil {
			return u, nil
		}
		return nil, nil
	}
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}

func (s *UserRepoStub) FindByIDs(ctx context.Context, ids []meta.ID) (map[meta.ID]*user.User, error) {
	s.mu.Lock()
	s.FindIDCalls++
	findErr := s.FindErr
	out := make(map[meta.ID]*user.User, len(ids))
	for _, id := range ids {
		if u := s.UsersByID[id.Uint64()]; u != nil {
			out[id] = u
		}
	}
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	return out, nil
}

func (s *UserRepoStub) FindByNickname(_ context.Context, nickname string) ([]*user.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FindErr != nil {
		return nil, s.FindErr
	}
	var matches []*user.User
	for _, candidate := range s.UsersByID {
		if candidate != nil && candidate.Nickname == nickname {
			matches = append(matches, candidate)
		}
	}
	return matches, nil
}

func (s *UserRepoStub) FindByPhone(ctx context.Context, phone meta.Phone) (*user.User, error) {
	s.mu.Lock()
	s.FindPhoneCalls++
	phoneErr := s.PhoneErr
	u, ok := s.UsersByPhone[phone.String()]
	s.mu.Unlock()

	if phoneErr != nil {
		return nil, phoneErr
	}
	if ok {
		if u != nil {
			return u, nil
		}
		return nil, nil
	}
	return nil, nil
}

func (s *UserRepoStub) Update(ctx context.Context, u *user.User) error {
	s.mu.Lock()
	s.UpdateArgs = append(s.UpdateArgs, u)
	findErr := s.FindErr
	s.mu.Unlock()
	return findErr
}

// VaultStub is a simple SecretVault implementation for tests.
type VaultStub struct {
	Err error
}

func (s *VaultStub) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	out := make([]byte, len(plaintext)+7)
	copy(out, []byte("cipher:"))
	copy(out[7:], plaintext)
	return out, nil
}
func (s *VaultStub) Decrypt(ctx context.Context, cipher []byte) ([]byte, error) { return nil, nil }
func (s *VaultStub) Sign(ctx context.Context, keyRef string, data []byte) ([]byte, error) {
	return nil, nil
}

func (r *RoleRepoStub) FindByTenantAndID(ctx context.Context, tenantID tenant.ID, id meta.ID) (*role.Role, error) {
	item, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil || !item.BelongsToTenant(tenantID.String()) {
		return nil, perrors.WithCode(code.ErrRoleNotFound, "role not found")
	}
	return item, nil
}
