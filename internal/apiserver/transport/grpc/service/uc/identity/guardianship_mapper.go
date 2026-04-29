package identity

import (
	"strconv"
	"time"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func guardianshipResultToProto(result *guardianshipApp.GuardianshipResult) *identityv1.Guardianship {
	if result == nil {
		return nil
	}

	return &identityv1.Guardianship{
		Id:        strconv.FormatUint(result.ID, 10),
		UserId:    result.UserID,
		ChildId:   result.ChildID,
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
