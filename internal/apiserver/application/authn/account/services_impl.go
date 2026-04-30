package account

import (
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
)

type accountApplicationService struct {
	uow            uow.UnitOfWork
	sessionManager sessiondomain.Manager
}

var _ AccountApplicationService = (*accountApplicationService)(nil)

func NewAccountApplicationService(uow uow.UnitOfWork, sessionManager sessiondomain.Manager) AccountApplicationService {
	return &accountApplicationService{uow: uow, sessionManager: sessionManager}
}
