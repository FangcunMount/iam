package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
)

func TestMyProfiles_Create_RollsBackProfileOnLinkFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	service := profile.NewMyProfiles(unitOfWork)

	result, err := service.Create(context.Background(), "999999999999999999", profile.CreateMyProfileDTO{
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
