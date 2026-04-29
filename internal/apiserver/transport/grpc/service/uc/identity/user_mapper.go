package identity

import (
	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	userApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

func userResultToProto(result *userApp.UserResult) *identityv1.User {
	if result == nil {
		return nil
	}

	contacts := make([]*identityv1.VerifiedContact, 0)
	if result.Phone != "" {
		contacts = append(contacts, &identityv1.VerifiedContact{
			Type:  identityv1.ContactType_CONTACT_TYPE_PHONE,
			Value: result.Phone,
		})
	}
	if result.Email != "" {
		contacts = append(contacts, &identityv1.VerifiedContact{
			Type:  identityv1.ContactType_CONTACT_TYPE_EMAIL,
			Value: result.Email,
		})
	}

	return &identityv1.User{
		Id:                 result.ID,
		Status:             userStatusToProto(result.Status),
		Nickname:           result.Name,
		AvatarUrl:          "",
		Contacts:           contacts,
		ExternalIdentities: []*identityv1.ExternalIdentity{},
		CreatedAt:          nil,
		UpdatedAt:          nil,
	}
}

func userStatusToProto(status userDomain.UserStatus) identityv1.UserStatus {
	switch status {
	case userDomain.UserActive:
		return identityv1.UserStatus_USER_STATUS_ACTIVE
	case userDomain.UserInactive:
		return identityv1.UserStatus_USER_STATUS_INACTIVE
	case userDomain.UserBlocked:
		return identityv1.UserStatus_USER_STATUS_BLOCKED
	default:
		return identityv1.UserStatus_USER_STATUS_UNSPECIFIED
	}
}
