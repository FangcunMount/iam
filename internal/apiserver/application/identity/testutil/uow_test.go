package testutil

import (
	"testing"

	identityuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/stretchr/testify/require"
)

func TestNewUnitOfWorkReturnsApplicationPort(t *testing.T) {
	db := SetupTestDB(t)

	unitOfWork := NewUnitOfWork(db)

	require.NotNil(t, unitOfWork)
	var _ identityuow.UnitOfWork = unitOfWork
}
