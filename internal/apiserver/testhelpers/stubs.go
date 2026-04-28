package testhelpers

import (
	"context"
	"sync"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	assignment "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	role "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	wechatapp "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	child "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	user "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
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
func (s *AssignmentRepoStub) DeleteBySubjectAndRole(ctx context.Context, subjectType assignment.SubjectType, subjectID string, roleID uint64, tenantID string) error {
	return nil
}
func (s *AssignmentRepoStub) FindByID(ctx context.Context, id assignment.AssignmentID) (*assignment.Assignment, error) {
	return nil, nil
}
func (s *AssignmentRepoStub) ListBySubject(ctx context.Context, subjectType assignment.SubjectType, subjectID, tenantID string) ([]*assignment.Assignment, error) {
	return s.Assignments, s.Err
}
func (s *AssignmentRepoStub) ListByRole(ctx context.Context, roleID uint64, tenantID string) ([]*assignment.Assignment, error) {
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

// ChildRepoStub is a stub for child.Repository used in tests.
type ChildRepoStub struct {
	Child      *child.Child
	FindErr    error
	FindCalls  int
	UpdateArgs []*child.Child
	mu         sync.Mutex
}

func (s *ChildRepoStub) Create(ctx context.Context, c *child.Child) error { return nil }
func (s *ChildRepoStub) FindByID(ctx context.Context, id meta.ID) (*child.Child, error) {
	s.mu.Lock()
	s.FindCalls++
	findErr := s.FindErr
	ch := s.Child
	s.mu.Unlock()

	if findErr != nil {
		return nil, findErr
	}
	if ch == nil {
		return nil, nil
	}
	return ch, nil
}
func (s *ChildRepoStub) FindByName(ctx context.Context, name string) (*child.Child, error) {
	return nil, s.FindErr
}
func (s *ChildRepoStub) FindByIDCard(ctx context.Context, idCard meta.IDCard) (*child.Child, error) {
	return nil, s.FindErr
}
func (s *ChildRepoStub) FindListByName(ctx context.Context, name string) ([]*child.Child, error) {
	return nil, s.FindErr
}
func (s *ChildRepoStub) FindListByNameAndBirthday(ctx context.Context, name string, birthday meta.Birthday) ([]*child.Child, error) {
	return nil, s.FindErr
}
func (s *ChildRepoStub) FindSimilar(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) ([]*child.Child, error) {
	return nil, s.FindErr
}
func (s *ChildRepoStub) Update(ctx context.Context, ch *child.Child) error {
	s.mu.Lock()
	s.UpdateArgs = append(s.UpdateArgs, ch)
	findErr := s.FindErr
	s.mu.Unlock()
	return findErr
}

// ChildValidatorStub is a stub for child.Validator used in tests.
type ChildValidatorStub struct {
	RenameErr        error
	UpdateProfileErr error
}

func (s *ChildValidatorStub) ValidateRegister(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) error {
	return nil
}
func (s *ChildValidatorStub) ValidateRename(name string) error { return s.RenameErr }
func (s *ChildValidatorStub) ValidateUpdateProfile(gender meta.Gender, birthday meta.Birthday) error {
	return s.UpdateProfileErr
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
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}

func (s *UserRepoStub) Update(ctx context.Context, u *user.User) error {
	s.mu.Lock()
	s.UpdateArgs = append(s.UpdateArgs, u)
	findErr := s.FindErr
	s.mu.Unlock()
	return findErr
}

// UserValidatorStub is a stub for user.Validator used in tests.
type UserValidatorStub struct {
	RenameErr          error
	UpdateContactErr   error
	CheckPhoneErr      error
	RenameCalls        int
	UpdateContactCalls int
	CheckCalls         int
}

func (s *UserValidatorStub) ValidateRegister(ctx context.Context, name string, phone meta.Phone) error {
	return nil
}
func (s *UserValidatorStub) ValidateRename(name string) error {
	s.RenameCalls++
	return s.RenameErr
}
func (s *UserValidatorStub) ValidateUpdateContact(ctx context.Context, u *user.User, phone meta.Phone, email meta.Email) error {
	s.UpdateContactCalls++
	return s.UpdateContactErr
}
func (s *UserValidatorStub) CheckPhoneUnique(ctx context.Context, phone meta.Phone) error {
	s.CheckCalls++
	return s.CheckPhoneErr
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
