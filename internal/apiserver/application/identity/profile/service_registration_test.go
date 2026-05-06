package profile_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/testutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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

func TestMyProfiles_CreateSelfRejectsDuplicateActiveSelfProfile(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userResult, err := user.NewCreator(unitOfWork).Create(ctx, user.CreateUserDTO{
		Name:  "当前用户",
		Phone: "13800139100",
	})
	require.NoError(t, err)

	service := profile.NewMyProfiles(unitOfWork)
	first, err := service.Create(ctx, userResult.ID, profile.CreateMyProfileDTO{
		Name:     "本人档案",
		Gender:   1,
		Birthday: "2000-01-01",
		Relation: "self",
	})
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, first.ProfileLink)
	assert.Equal(t, "self", first.ProfileLink.Relation)

	second, err := service.Create(ctx, userResult.ID, profile.CreateMyProfileDTO{
		Name:     "重复本人档案",
		Gender:   1,
		Birthday: "2001-01-01",
		Relation: "self",
	})
	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrIdentityProfileLinkExists))
	assert.Nil(t, second)

	var selfLinkCount int64
	require.NoError(t, db.Table("profile_links").
		Where("user_id = ? AND type = ? AND relation = ? AND revoked_at IS NULL", userResult.ID, "self", "self").
		Count(&selfLinkCount).Error)
	assert.Equal(t, int64(1), selfLinkCount)

	var profileCount int64
	require.NoError(t, db.Table("profiles").Count(&profileCount).Error)
	assert.Equal(t, int64(1), profileCount)
}

func TestMyProfiles_CreateRelationAllowsMultipleProfiles(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userResult, err := user.NewCreator(unitOfWork).Create(ctx, user.CreateUserDTO{
		Name:  "关系用户",
		Phone: "13800139101",
	})
	require.NoError(t, err)

	service := profile.NewMyProfiles(unitOfWork)
	for _, name := range []string{"关系档案一", "关系档案二"} {
		result, err := service.Create(ctx, userResult.ID, profile.CreateMyProfileDTO{
			Name:     name,
			Gender:   1,
			Birthday: "2020-01-01",
			Relation: "parent",
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "parent", result.ProfileLink.Relation)
	}

	links, err := profilelink.NewDirectory(unitOfWork).ListProfilesForUser(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, links, 2)
	for _, link := range links {
		assert.Equal(t, "parent", link.Relation)
	}
}
