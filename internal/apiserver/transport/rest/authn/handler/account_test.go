package handler

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	appAccount "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/account"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestParseAccountIDTrimsAndRejectsInvalidValues(t *testing.T) {
	id, err := parseAccountID(" 42 ")
	require.NoError(t, err)
	require.Equal(t, uint64(42), id.Uint64())

	_, err = parseAccountID("")
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())

	_, err = parseAccountID("abc")
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestToAccountResponseKeepsCurrentWireShape(t *testing.T) {
	accountID := meta.FromUint64(7)
	userID := meta.FromUint64(11)
	got := toAccountResponse(&appAccount.AccountResult{
		AccountID:  accountID,
		UserID:     userID,
		Type:       domain.TypeWcMinip,
		ExternalID: domain.ExternalID("openid-1"),
		AppID:      domain.AppId("wx-app"),
		Status:     domain.StatusActive,
	})

	require.Equal(t, "7", got.ID)
	require.Equal(t, "11", got.UserID)
	require.Equal(t, string(domain.TypeWcMinip), got.Provider)
	require.Equal(t, "openid-1", got.ExternalID)
	require.NotNil(t, got.AppID)
	require.Equal(t, "wx-app", *got.AppID)
	require.Equal(t, "active", got.Status)
}
