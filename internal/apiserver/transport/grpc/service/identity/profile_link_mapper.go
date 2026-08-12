package identity

import (
	"strconv"
	"time"

	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	profileLinkApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profilelink"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func profileLinkResultToProto(result *profileLinkApp.ProfileLinkResult) *identityv2.ProfileLink {
	if result == nil {
		return nil
	}

	return &identityv2.ProfileLink{
		Id:        strconv.FormatUint(result.ID, 10),
		UserId:    result.UserID,
		ProfileId: result.ProfileID,
		Relation:  stringToProtoRelation(result.Relation),
		Since:     parseTimestamp(result.EstablishedAt),
		RevokedAt: parseTimestamp(result.RevokedAt),
	}
}

func parseTimestamp(value string) *timestamppb.Timestamp {
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}
