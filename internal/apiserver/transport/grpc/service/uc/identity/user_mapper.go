package identity

import (
	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	userApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/user"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
)

func userResultToProto(result *userApp.UserResult) *identityv2.User {
	if result == nil {
		return nil
	}

	contacts := make([]*identityv2.VerifiedContact, 0)
	if result.Phone != "" {
		contacts = append(contacts, &identityv2.VerifiedContact{
			Type:  identityv2.ContactType_CONTACT_TYPE_PHONE,
			Value: result.Phone,
		})
	}
	if result.Email != "" {
		contacts = append(contacts, &identityv2.VerifiedContact{
			Type:  identityv2.ContactType_CONTACT_TYPE_EMAIL,
			Value: result.Email,
		})
	}

	return &identityv2.User{
		Id:                 result.ID,
		Status:             userStatusToProto(result.Status),
		Nickname:           result.Name,
		AvatarUrl:          "",
		Contacts:           contacts,
		ExternalIdentities: []*identityv2.ExternalIdentity{},
		CreatedAt:          nil,
		UpdatedAt:          nil,
	}
}

func userStatusToProto(status userDomain.UserStatus) identityv2.UserStatus {
	switch status {
	case userDomain.UserActive:
		return identityv2.UserStatus_USER_STATUS_ACTIVE
	case userDomain.UserInactive:
		return identityv2.UserStatus_USER_STATUS_INACTIVE
	case userDomain.UserBlocked:
		return identityv2.UserStatus_USER_STATUS_BLOCKED
	default:
		return identityv2.UserStatus_USER_STATUS_UNSPECIFIED
	}
}
