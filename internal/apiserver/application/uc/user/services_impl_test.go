package user

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	ucuow "github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	userdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type queryUserRepoStub struct{}

func (s *queryUserRepoStub) Create(context.Context, *userdomain.User) error { return nil }
func (s *queryUserRepoStub) FindByID(context.Context, meta.ID) (*userdomain.User, error) {
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *queryUserRepoStub) FindByPhone(context.Context, meta.Phone) (*userdomain.User, error) {
	return nil, perrors.WithCode(code.ErrUserNotFound, "user not found")
}
func (s *queryUserRepoStub) Update(context.Context, *userdomain.User) error { return nil }

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
