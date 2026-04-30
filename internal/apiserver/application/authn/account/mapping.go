package account

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

func toAccountResult(account *domain.Account) *AccountResult {
	return &AccountResult{
		AccountID:  account.ID,
		UserID:     account.UserID,
		Type:       account.Type,
		AppID:      account.AppID,
		ExternalID: account.ExternalID,
		UniqueID:   account.UniqueID,
		Profile:    account.Profile,
		Meta:       account.Meta,
		Status:     account.Status,
	}
}

func isAccountNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrCredentialNotFound) ||
		perrors.IsCode(err, code.ErrNotFoundAccount)
}

func accountNotFound(l *logger.RequestLogger, accountID meta.ID) error {
	l.Warnw("账户不存在",
		"action", logger.ActionRead,
		"resource", "account",
		"account_id", accountID.String(),
		"result", logger.ResultFailed,
	)
	return perrors.WithCode(code.ErrCredentialNotFound, "account not found")
}
