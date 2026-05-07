package identity

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	userApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserResultToProtoKeepsContactsAndStatus(t *testing.T) {
	got := userResultToProto(&userApp.UserResult{
		ID:     "user-1",
		Name:   "Alice",
		Phone:  "+8613800138000",
		Email:  "alice@example.com",
		Status: userDomain.UserBlocked,
	})

	require.Equal(t, "user-1", got.Id)
	require.Equal(t, "Alice", got.Nickname)
	require.Equal(t, identityv2.UserStatus_USER_STATUS_BLOCKED, got.Status)
	require.Len(t, got.Contacts, 2)
	require.Equal(t, identityv2.ContactType_CONTACT_TYPE_PHONE, got.Contacts[0].Type)
	require.Equal(t, identityv2.ContactType_CONTACT_TYPE_EMAIL, got.Contacts[1].Type)
}

func TestProfileResultToProtoFormatsGenderAndIdentity(t *testing.T) {
	got := profileResultToProto(&profileApp.ProfileResult{
		ID:       "profile-1",
		Name:     "Bob",
		IDCard:   "110***********001",
		Gender:   1,
		Birthday: "2020-01-02",
	})

	require.Equal(t, "profile-1", got.Id)
	require.Equal(t, "Bob", got.LegalName)
	require.Equal(t, identityv2.Gender_GENDER_MALE, got.Gender)
	require.Equal(t, "2020-01-02", got.Dob)
	require.Equal(t, "110***********001", got.Identity.MaskedNumber)
}

func TestProfileLinkResultToProtoParsesRelationAndTimestamps(t *testing.T) {
	got := profileLinkResultToProto(&profileLinkApp.ProfileLinkResult{
		ID:            42,
		UserID:        "user-1",
		ProfileID:     "profile-1",
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-30T10:20:30Z",
	})

	require.Equal(t, "42", got.Id)
	require.Equal(t, identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT, got.Relation)
	require.NotNil(t, got.Since)
	require.NotNil(t, got.RevokedAt)
}

func TestToGRPCErrorMapsRegisteredHTTPStatus(t *testing.T) {
	err := toGRPCError(perrors.WithCode(code.ErrInvalidArgument, "invalid"))

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
