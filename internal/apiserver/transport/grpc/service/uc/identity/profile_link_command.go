package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/profilelink"
)

// EstablishProfileLink 添加关系用户
func (s *profileLinkCommandServer) EstablishProfileLink(ctx context.Context, req *identityv2.EstablishProfileLinkRequest) (*identityv2.EstablishProfileLinkResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and profile_id are required")
	}

	dto := profileLinkApp.CreateProfileLinkDTO{
		UserID:    req.GetUserId(),
		ProfileID: req.GetProfileId(),
		Relation:  protoRelationToString(req.GetRelation()),
	}

	result, err := s.profileLinkSvc.Establish(ctx, dto)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.EstablishProfileLinkResponse{
		ProfileLink: profileLinkResultToProto(result),
	}, nil
}

// RevokeProfileLink 撤销档案关系
func (s *profileLinkCommandServer) RevokeProfileLink(ctx context.Context, req *identityv2.RevokeProfileLinkRequest) (*identityv2.RevokeProfileLinkResponse, error) {
	if req == nil || req.GetTarget() == nil {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}

	var userID, profileID string
	var profileLinkID string

	// 根据不同的 selector 解析
	switch target := req.GetTarget().GetSelector().(type) {
	case *identityv2.ProfileLinkSelector_ProfileLinkId:
		profileLinkID = target.ProfileLinkId

	case *identityv2.ProfileLinkSelector_Key:
		userID = target.Key.GetUserId()
		profileID = target.Key.GetProfileId()

	default:
		return nil, status.Error(codes.InvalidArgument, "invalid target selector")
	}

	profileLink, err := s.profileLinkSvc.RevokeBySelector(ctx, profileLinkApp.RevokeProfileLinkBySelectorDTO{
		ProfileLinkID: profileLinkID,
		UserID:        userID,
		ProfileID:     profileID,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.RevokeProfileLinkResponse{
		ProfileLink: profileLinkResultToProto(profileLink),
	}, nil
}

// BatchRevokeProfileLinks 批量撤销档案关系
func (s *profileLinkCommandServer) BatchRevokeProfileLinks(ctx context.Context, req *identityv2.BatchRevokeProfileLinksRequest) (*identityv2.BatchRevokeProfileLinksResponse, error) {
	if req == nil || len(req.GetTargets()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "targets is required")
	}

	resp := &identityv2.BatchRevokeProfileLinksResponse{
		Revoked:  make([]*identityv2.ProfileLink, 0),
		Failures: make([]*identityv2.FailedProfileLinkFailure, 0),
	}

	for _, target := range req.GetTargets() {
		revokeReq := &identityv2.RevokeProfileLinkRequest{
			Target:   target,
			Reason:   req.GetReason(),
			Operator: req.GetOperator(),
		}

		revokeResp, err := s.RevokeProfileLink(ctx, revokeReq)
		if err != nil {
			resp.Failures = append(resp.Failures, &identityv2.FailedProfileLinkFailure{
				Target: target,
				Error:  err.Error(),
			})
			continue
		}
		if revokeResp != nil && revokeResp.ProfileLink != nil {
			resp.Revoked = append(resp.Revoked, revokeResp.ProfileLink)
		}
	}

	return resp, nil
}

// ImportProfileLinks 批量导入档案关系
func (s *profileLinkCommandServer) ImportProfileLinks(ctx context.Context, req *identityv2.ImportProfileLinksRequest) (*identityv2.ImportProfileLinksResponse, error) {
	if req == nil || len(req.GetRecords()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "records is required")
	}

	resp := &identityv2.ImportProfileLinksResponse{
		Created:  make([]*identityv2.ProfileLink, 0),
		Failures: make([]*identityv2.FailedImportProfileLink, 0),
	}

	for _, record := range req.GetRecords() {
		addReq := &identityv2.EstablishProfileLinkRequest{
			UserId:    record.GetUserId(),
			ProfileId: record.GetProfileId(),
			Relation:  record.GetRelation(),
			Operator:  req.GetOperator(),
		}

		addResp, err := s.EstablishProfileLink(ctx, addReq)
		if err != nil {
			resp.Failures = append(resp.Failures, &identityv2.FailedImportProfileLink{
				Record: record,
				Error:  err.Error(),
			})
			continue
		}

		if addResp != nil && addResp.ProfileLink != nil {
			resp.Created = append(resp.Created, addResp.ProfileLink)
		}
	}

	return resp, nil
}

// protoRelationToString 将 proto 枚举转换为字符串
func protoRelationToString(relation identityv2.ProfileLinkRelation) string {
	switch relation {
	case identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_SELF:
		return "self"
	case identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT:
		return "parent"
	case identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_GRANDPARENT:
		return profileLinkApp.NormalizeRelation("grandparent")
	case identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_OTHER:
		return "other"
	default:
		return "other"
	}
}

// stringToProtoRelation 将字符串转换为 proto 枚举
func stringToProtoRelation(relation string) identityv2.ProfileLinkRelation {
	switch relation {
	case "self":
		return identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_SELF
	case "parent":
		return identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT
	case "grandparent":
		return identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_GRANDPARENT
	case "other":
		return identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_OTHER
	default:
		return identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_UNSPECIFIED
	}
}
