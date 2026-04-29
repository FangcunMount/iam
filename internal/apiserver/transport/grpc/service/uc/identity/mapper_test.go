package identity

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	childApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	userApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
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
	require.Equal(t, identityv1.UserStatus_USER_STATUS_BLOCKED, got.Status)
	require.Len(t, got.Contacts, 2)
	require.Equal(t, identityv1.ContactType_CONTACT_TYPE_PHONE, got.Contacts[0].Type)
	require.Equal(t, identityv1.ContactType_CONTACT_TYPE_EMAIL, got.Contacts[1].Type)
}

func TestChildResultToProtoFormatsGenderIdentityAndWeight(t *testing.T) {
	got := childResultToProto(&childApp.ChildResult{
		ID:       "child-1",
		Name:     "Bob",
		IDCard:   "110***********001",
		Gender:   1,
		Birthday: "2020-01-02",
		Height:   120,
		Weight:   23500,
	})

	require.Equal(t, "child-1", got.Id)
	require.Equal(t, "Bob", got.LegalName)
	require.Equal(t, identityv1.Gender_GENDER_MALE, got.Gender)
	require.Equal(t, "2020-01-02", got.Dob)
	require.Equal(t, "110***********001", got.Identity.MaskedNumber)
	require.Equal(t, int32(120), got.Stats.HeightCm)
	require.Equal(t, "23.50", got.Stats.WeightKg)
}

func TestGuardianshipResultToProtoParsesRelationAndTimestamps(t *testing.T) {
	got := guardianshipResultToProto(&guardianshipApp.GuardianshipResult{
		ID:            42,
		UserID:        "user-1",
		ChildID:       "child-1",
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-30T10:20:30Z",
	})

	require.Equal(t, "42", got.Id)
	require.Equal(t, identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_PARENT, got.Relation)
	require.NotNil(t, got.Since)
	require.NotNil(t, got.RevokedAt)
}

func TestToGRPCErrorMapsRegisteredHTTPStatus(t *testing.T) {
	err := toGRPCError(perrors.WithCode(code.ErrInvalidArgument, "invalid"))

	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
