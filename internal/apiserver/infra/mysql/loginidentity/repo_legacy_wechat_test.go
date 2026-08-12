package loginidentity

import (
	"context"
	"testing"
	"time"

	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFindLegacyWechatIdentityByProviderKeyRequiresExplicitMarker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PO{}))

	repo := NewRepository(db)
	ctx := context.Background()
	for _, identity := range []*domain.LoginIdentity{
		{
			ID:         meta.FromUint64(1),
			UserID:     meta.FromUint64(101),
			Provider:   domain.ProviderWechatMinip,
			Realm:      "wx-app",
			Identifier: "ordinary-union",
			Status:     domain.StatusActive,
			LinkedAt:   time.Now(),
		},
		{
			ID:         meta.FromUint64(2),
			UserID:     meta.FromUint64(102),
			Provider:   domain.ProviderWechatMinip,
			Realm:      "wx-app",
			Identifier: "legacy-union",
			Status:     domain.StatusActive,
			LinkedAt:   time.Now(),
			Meta: map[string]string{
				domain.MetaLegacyIdentifierSemantics: domain.LegacyIdentifierOpenOrUnion,
			},
		},
	} {
		require.NoError(t, repo.Create(ctx, identity))
	}

	ordinary, err := repo.FindLegacyWechatIdentityByProviderKey(
		ctx, domain.ProviderWechatMinip, "wx-app", "ordinary-union",
	)
	require.NoError(t, err)
	require.Nil(t, ordinary)

	legacy, err := repo.FindLegacyWechatIdentityByProviderKey(
		ctx, domain.ProviderWechatMinip, "wx-app", "legacy-union",
	)
	require.NoError(t, err)
	require.NotNil(t, legacy)
	require.Equal(t, meta.FromUint64(2), legacy.LoginIdentityID)
}
