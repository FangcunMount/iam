package testutil

import (
	"testing"

	ucuow "github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/stretchr/testify/require"
)

func TestNewUnitOfWorkReturnsApplicationPort(t *testing.T) {
	db := SetupTestDB(t)

	unitOfWork := NewUnitOfWork(db)

	require.NotNil(t, unitOfWork)
	var _ ucuow.UnitOfWork = unitOfWork
}
