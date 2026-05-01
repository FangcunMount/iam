package account

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestDisableAccountUpdatesStatusBeforeRevokingSessions(t *testing.T) {
	t.Parallel()

	accountID := meta.FromUint64(1001)
	events := []string{}
	repo := &accountRepoStub{
		account: domain.NewAccount(
			meta.FromUint64(2002),
			domain.TypeWcMinip,
			domain.ExternalID("openid@app"),
			domain.WithID(accountID),
		),
		events: &events,
	}
	sessions := &sessionManagerStub{events: &events}
	svc := NewAccountApplicationService(&accountUOWStub{repo: repo, events: &events}, sessions)

	err := svc.DisableAccount(context.Background(), accountID)

	require.NoError(t, err)
	require.Equal(t, domain.StatusDisabled, repo.updatedStatus)
	require.Equal(t, []string{
		"tx:start",
		"account:update_status",
		"tx:end",
		"session:revoke_account",
	}, events)
	require.Equal(t, accountID, sessions.revokedAccountID)
	require.Equal(t, "account_disabled", sessions.reason)
	require.Equal(t, accountID.String(), sessions.revokedBy)
}

type accountUOWStub struct {
	repo   domain.Repository
	events *[]string
}

func (s *accountUOWStub) WithinTx(ctx context.Context, fn func(context.Context, uow.TxRepositories) error) error {
	*s.events = append(*s.events, "tx:start")
	err := fn(ctx, uow.TxRepositories{Accounts: s.repo})
	*s.events = append(*s.events, "tx:end")
	return err
}

type accountRepoStub struct {
	account       *domain.Account
	updatedStatus domain.AccountStatus
	events        *[]string
}

func (s *accountRepoStub) Create(context.Context, *domain.Account) error { return nil }

func (s *accountRepoStub) UpdateUniqueID(context.Context, meta.ID, domain.UnionID) error {
	return nil
}

func (s *accountRepoStub) UpdateStatus(_ context.Context, _ meta.ID, status domain.AccountStatus) error {
	s.updatedStatus = status
	*s.events = append(*s.events, "account:update_status")
	return nil
}

func (s *accountRepoStub) UpdateProfile(context.Context, meta.ID, map[string]string) error {
	return nil
}

func (s *accountRepoStub) UpdateMeta(context.Context, meta.ID, map[string]string) error {
	return nil
}

func (s *accountRepoStub) GetByID(context.Context, meta.ID) (*domain.Account, error) {
	return s.account, nil
}

func (s *accountRepoStub) GetByUniqueID(context.Context, domain.UnionID) (*domain.Account, error) {
	return nil, nil
}

func (s *accountRepoStub) GetByExternalIDAppId(context.Context, domain.ExternalID, domain.AppId) (*domain.Account, error) {
	return nil, nil
}

type sessionManagerStub struct {
	events           *[]string
	revokedAccountID meta.ID
	reason           string
	revokedBy        string
}

func (s *sessionManagerStub) Create(context.Context, *authentication.Principal, time.Time) (*sessiondomain.Session, error) {
	return nil, nil
}

func (s *sessionManagerStub) Get(context.Context, string) (*sessiondomain.Session, error) {
	return nil, nil
}

func (s *sessionManagerStub) Revoke(context.Context, string, string, string) error {
	return nil
}

func (s *sessionManagerStub) RevokeByUser(context.Context, meta.ID, string, string) error {
	return nil
}

func (s *sessionManagerStub) RevokeByAccount(_ context.Context, accountID meta.ID, reason string, revokedBy string) error {
	*s.events = append(*s.events, "session:revoke_account")
	s.revokedAccountID = accountID
	s.reason = reason
	s.revokedBy = revokedBy
	return nil
}

func (s *sessionManagerStub) Extend(context.Context, string, time.Time) error {
	return nil
}
