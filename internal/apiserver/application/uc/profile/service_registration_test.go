package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
)

func TestProfileRegistrationService_RegisterProfileWithProfileLink_RollsBackProfileOnLinkFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	service := profile.NewProfileRegistrationService(unitOfWork)

	result, err := service.RegisterProfileWithProfileLink(context.Background(), profile.RegisterProfileWithProfileLinkDTO{
		UserID:   "999999999999999999",
		Name:     "回滚测试档案",
		Gender:   1,
		Birthday: "2020-04-21",
		Relation: "parent",
	})

	require.Error(t, err)
	assert.Nil(t, result)

	var count int64
	require.NoError(t, db.Table("profiles").Count(&count).Error)
	assert.Zero(t, count)
}
