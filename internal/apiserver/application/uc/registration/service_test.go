package registration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	mysqluow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/uc"
)

func TestChildRegistrationService_RegisterChildWithGuardian_RollsBackChildOnGuardianshipFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := mysqluow.NewUnitOfWork(db)
	service := registration.NewChildRegistrationService(unitOfWork)

	result, err := service.RegisterChildWithGuardian(context.Background(), registration.RegisterChildWithGuardianDTO{
		UserID:   "999999999999999999",
		Name:     "回滚测试儿童",
		Gender:   1,
		Birthday: "2020-04-21",
		Relation: "parent",
	})

	require.Error(t, err)
	assert.Nil(t, result)

	var count int64
	require.NoError(t, db.Table("children").Count(&count).Error)
	assert.Zero(t, count)
}
