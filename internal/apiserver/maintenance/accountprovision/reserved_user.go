package accountprovision

import (
	"context"
	"fmt"

	authnuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/uow"
	user "github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

// This maintenance-only repository decorator assigns the reviewed ID before the
// normal Signup inserts it. No public signup API accepts a caller-chosen ID.
type reservedUserUnitOfWork struct {
	base authnuow.UnitOfWork
	id   meta.ID
}

func (u reservedUserUnitOfWork) WithinTx(ctx context.Context, fn func(context.Context, authnuow.TxRepositories) error) error {
	return u.base.WithinTx(ctx, func(ctx context.Context, tx authnuow.TxRepositories) error {
		tx.Users = reservedUserRepository{Repository: tx.Users, id: u.id}
		return fn(ctx, tx)
	})
}

type reservedUserRepository struct {
	user.Repository
	id meta.ID
}

func (r reservedUserRepository) Create(ctx context.Context, value *user.User) error {
	if value.ID != 0 {
		return fmt.Errorf("reserved provisioning cannot repair or reassign a user")
	}
	value.ID = r.id
	return r.Repository.Create(ctx, value)
}
