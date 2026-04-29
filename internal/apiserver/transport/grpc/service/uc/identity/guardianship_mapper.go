package identity

import (
	"strconv"
	"time"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	guardianshipDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
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

func guardianshipDomainToProto(g *guardianshipDomain.Guardianship) *identityv1.Guardianship {
	if g == nil {
		return nil
	}

	guardianship := &identityv1.Guardianship{
		Id:       g.ID.String(),
		UserId:   g.User.String(),
		ChildId:  g.Child.String(),
		Relation: relationToProto(g.Rel),
		Since:    timestamppb.New(g.EstablishedAt),
	}

	if g.RevokedAt != nil && !g.RevokedAt.IsZero() {
		guardianship.RevokedAt = timestamppb.New(*g.RevokedAt)
	}

	return guardianship
}

func relationToProto(relation guardianshipDomain.Relation) identityv1.GuardianshipRelation {
	switch relation {
	case guardianshipDomain.RelSelf:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_SELF
	case guardianshipDomain.RelParent:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_PARENT
	case guardianshipDomain.RelGrandparent:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_GRANDPARENT
	case guardianshipDomain.RelOther:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_OTHER
	default:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_UNSPECIFIED
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
