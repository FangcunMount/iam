package identity

import (
	"context"
	"testing"

	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	userApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/user"
	"github.com/stretchr/testify/require"
)

func TestSearchUsersKeywordFindsExactNickname(t *testing.T) {
	query := &userQueryStub{nicknameUsers: []*userApp.UserResult{
		{ID: "10", Nickname: "__matrix__"},
	}}
	server := &identityReadServer{userQuerySvc: query}

	response, err := server.SearchUsers(context.Background(), &identityv2.SearchUsersRequest{
		Keyword: "  __matrix__  ",
		Page:    &identityv2.OffsetPagination{Limit: 100},
	})

	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetTotal())
	require.Equal(t, "__matrix__", response.GetUsers()[0].GetNickname())
}
