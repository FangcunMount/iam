package loginidentity

import (
	"context"
	"testing"
	"time"

	testutil "github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/testutil"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestListActiveMiniProgramByUserIDsRestrictsProviderRealmAndStatus(t *testing.T) {
	db := testutil.OpenDBForIntegrationTest(t, &PO{})
	repo := NewRepository(db)
	user1, user2, user3 := meta.New(), meta.New(), meta.New()
	appA, appB := "wx-app-a-"+user1.String(), "wx-app-b-"+user1.String()
	var created []meta.ID
	t.Cleanup(func() {
		if len(created) > 0 {
			_ = db.Unscoped().Delete(&PO{}, "id IN ?", created).Error
		}
	})
	for _, identity := range []*domain.LoginIdentity{
		{UserID: user1, Provider: domain.ProviderWechatMinip, Realm: appA, Identifier: "open-a-" + user1.String(), Status: domain.StatusActive, LinkedAt: time.Now()},
		{UserID: user1, Provider: domain.ProviderWechatMinip, Realm: appB, Identifier: "open-b-" + user1.String(), Status: domain.StatusActive, LinkedAt: time.Now()},
		{UserID: user2, Provider: domain.ProviderWechatMinip, Realm: appA, Identifier: "open-disabled-" + user2.String(), Status: domain.StatusDisabled, LinkedAt: time.Now()},
		{UserID: user3, Provider: domain.ProviderWechatMinip, Realm: appA, Identifier: "other-user-" + user3.String(), Status: domain.StatusActive, LinkedAt: time.Now()},
		{UserID: user1, Provider: domain.ProviderWechatOpen, Realm: appA, Identifier: "wrong-provider-" + user1.String(), Status: domain.StatusActive, LinkedAt: time.Now()},
	} {
		require.NoError(t, repo.Create(context.Background(), identity))
		created = append(created, identity.ID)
	}
	deleted := &domain.LoginIdentity{UserID: user1, Provider: domain.ProviderWechatMinip, Realm: appA, Identifier: "deleted-" + user1.String(), Status: domain.StatusActive, LinkedAt: time.Now()}
	require.NoError(t, repo.Create(context.Background(), deleted))
	created = append(created, deleted.ID)
	require.NoError(t, db.Model(&PO{}).Where("id = ?", deleted.ID).Update("deleted_at", time.Now()).Error)
	items, err := repo.ListActiveMiniProgramByUserIDs(context.Background(), []meta.ID{user1, user2}, appA)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, created[0], items[0].ID)
}
