package user

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	ucuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/uow"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type queryUserRepoStub struct{}

func (s *queryUserRepoStub) Create(context.Context, *userdomain.User) error { return nil }
func (s *queryUserRepoStub) FindByID(context.Context, meta.ID) (*userdomain.User, error) {
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *queryUserRepoStub) FindByIDs(context.Context, []meta.ID) (map[meta.ID]*userdomain.User, error) {
	return map[meta.ID]*userdomain.User{}, nil
}
func (s *queryUserRepoStub) FindByPhone(context.Context, meta.Phone) (*userdomain.User, error) {
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *queryUserRepoStub) Update(context.Context, *userdomain.User) error { return nil }

type batchUserRepoStub struct {
	users       map[meta.ID]*userdomain.User
	findByID    int
	findByIDs   [][]meta.ID
	findByPhone int
}

func (s *batchUserRepoStub) Create(context.Context, *userdomain.User) error { return nil }
func (s *batchUserRepoStub) FindByID(context.Context, meta.ID) (*userdomain.User, error) {
	s.findByID++
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *batchUserRepoStub) FindByIDs(_ context.Context, ids []meta.ID) (map[meta.ID]*userdomain.User, error) {
	s.findByIDs = append(s.findByIDs, append([]meta.ID(nil), ids...))
	out := make(map[meta.ID]*userdomain.User, len(ids))
	for _, id := range ids {
		if u := s.users[id]; u != nil {
			out[id] = u
		}
	}
	return out, nil
}
func (s *batchUserRepoStub) FindByPhone(context.Context, meta.Phone) (*userdomain.User, error) {
	s.findByPhone++
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *batchUserRepoStub) Update(context.Context, *userdomain.User) error { return nil }

type queryUOWStub struct {
	users userdomain.Repository
}

func (s *queryUOWStub) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx ucuow.TxRepositories) error) error {
	return fn(ctx, ucuow.TxRepositories{Users: s.users})
}

func TestUserQueryGetByID_ReturnsErrUserNotFound(t *testing.T) {
	t.Parallel()

	svc := NewDirectory(&queryUOWStub{users: &queryUserRepoStub{}})
	result, err := svc.GetByID(context.Background(), "615206334492586542")

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))
}

func TestUserQueryBatchGetByIDUsesRepositoryBatchAndSkipsMissing(t *testing.T) {
	t.Parallel()

	user10 := mustBatchUser(t, 10, "alice", "+8613800000010")
	user11 := mustBatchUser(t, 11, "bob", "+8613800000011")
	repo := &batchUserRepoStub{
		users: map[meta.ID]*userdomain.User{
			user10.ID: user10,
			user11.ID: user11,
		},
	}

	svc := NewDirectory(&queryUOWStub{users: repo})
	results, err := svc.BatchGetByID(context.Background(), []string{"10", "bad", "11", "10", "404"})

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "alice", results["10"].Name)
	require.Equal(t, "bob", results["11"].Name)
	require.Nil(t, results["404"])
	require.Equal(t, 0, repo.findByID)
	require.Len(t, repo.findByIDs, 1)
	require.Equal(t, []meta.ID{meta.FromUint64(10), meta.FromUint64(11), meta.FromUint64(404)}, repo.findByIDs[0])
}

func mustBatchUser(t *testing.T, id uint64, name, phoneText string) *userdomain.User {
	t.Helper()
	phone, err := meta.NewPhone(phoneText)
	require.NoError(t, err)
	u, err := userdomain.NewUser(name, phone, userdomain.WithID(meta.FromUint64(id)))
	require.NoError(t, err)
	return u
}
