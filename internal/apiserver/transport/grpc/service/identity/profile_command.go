package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
)

// CreateProfile 创建档案并建立 User -> Profile 关系。
func (s *profileCommandServer) CreateProfile(ctx context.Context, req *identityv2.CreateProfileRequest) (*identityv2.CreateProfileResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if strings.TrimSpace(req.GetLegalName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "legal_name is required")
	}

	userID, err := parseIDArg("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := s.profileCommandSvc.Create(ctx, userID, profileApp.CreateProfileDTO{
		Name:     strings.TrimSpace(req.GetLegalName()),
		Gender:   protoGenderToUint8(req.GetGender()),
		Birthday: strings.TrimSpace(req.GetDob()),
		IDCard:   strings.TrimSpace(req.GetIdCardNumber()),
		Relation: protoRelationToString(req.GetRelation()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.CreateProfileResponse{
		Profile:     profileResultToProto(result.Profile),
		ProfileLink: profileLinkResultToProto(result.ProfileLink),
	}, nil
}

func protoGenderToUint8(gender identityv2.Gender) uint8 {
	switch gender {
	case identityv2.Gender_GENDER_MALE:
		return 1
	case identityv2.Gender_GENDER_FEMALE:
		return 2
	case identityv2.Gender_GENDER_OTHER:
		return 0
	default:
		return 0
	}
}
