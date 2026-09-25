package loginidentity

import (
	"context"
	"testing"
	"time"

	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListActiveMiniProgramByUserIDsRestrictsProviderRealmAndStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PO{}))
	repo := NewRepository(db)
	for _, identity := range []*domain.LoginIdentity{
		{ID: 11, UserID: 1, Provider: domain.ProviderWechatMinip, Realm: "wx-app-a", Identifier: "open-a", Status: domain.StatusActive, LinkedAt: time.Now()},
		{ID: 12, UserID: 1, Provider: domain.ProviderWechatMinip, Realm: "wx-app-b", Identifier: "open-b", Status: domain.StatusActive, LinkedAt: time.Now()},
		{ID: 13, UserID: 2, Provider: domain.ProviderWechatMinip, Realm: "wx-app-a", Identifier: "open-disabled", Status: domain.StatusDisabled, LinkedAt: time.Now()},
		{ID: 14, UserID: 3, Provider: domain.ProviderWechatMinip, Realm: "wx-app-a", Identifier: "other-user", Status: domain.StatusActive, LinkedAt: time.Now()},
		{ID: 15, UserID: 1, Provider: domain.ProviderWechatOpen, Realm: "wx-app-a", Identifier: "wrong-provider", Status: domain.StatusActive, LinkedAt: time.Now()},
	} {
		require.NoError(t, repo.Create(context.Background(), identity))
	}
	items, err := repo.ListActiveMiniProgramByUserIDs(context.Background(), []meta.ID{1, 2}, "wx-app-a")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, meta.ID(11), items[0].ID)
}
