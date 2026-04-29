package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
)

// AddGuardian 添加监护人
func (s *guardianshipCommandServer) AddGuardian(ctx context.Context, req *identityv1.AddGuardianRequest) (*identityv1.AddGuardianResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" || strings.TrimSpace(req.GetChildId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and child_id are required")
	}

	dto := guardianshipApp.AddGuardianDTO{
		UserID:   req.GetUserId(),
		ChildID:  req.GetChildId(),
		Relation: protoRelationToString(req.GetRelation()),
	}

	result, err := s.guardianshipAccessSvc.GrantForCurrentUser(ctx, req.GetUserId(), dto)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv1.AddGuardianResponse{
		Guardianship: guardianshipResultToProto(result),
	}, nil
}

// RevokeGuardian 撤销监护关系
func (s *guardianshipCommandServer) RevokeGuardian(ctx context.Context, req *identityv1.RevokeGuardianRequest) (*identityv1.RevokeGuardianResponse, error) {
	if req == nil || req.GetTarget() == nil {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}

	var userID, childID string
	var guardianshipID string

	// 根据不同的 selector 解析
	switch target := req.GetTarget().GetSelector().(type) {
	case *identityv1.GuardianshipSelector_GuardianshipId:
		guardianshipID = target.GuardianshipId

	case *identityv1.GuardianshipSelector_Key:
		userID = target.Key.GetUserId()
		childID = target.Key.GetChildId()

	default:
		return nil, status.Error(codes.InvalidArgument, "invalid target selector")
	}

	guardianship, err := s.guardianshipAccessSvc.RevokeBySelector(ctx, guardianshipApp.RevokeGuardianBySelectorDTO{
		GuardianshipID: guardianshipID,
		UserID:         userID,
		ChildID:        childID,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv1.RevokeGuardianResponse{
		Guardianship: guardianshipResultToProto(guardianship),
	}, nil
}

// BatchRevokeGuardians 批量撤销监护关系
func (s *guardianshipCommandServer) BatchRevokeGuardians(ctx context.Context, req *identityv1.BatchRevokeGuardiansRequest) (*identityv1.BatchRevokeGuardiansResponse, error) {
	if req == nil || len(req.GetTargets()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "targets is required")
	}

	resp := &identityv1.BatchRevokeGuardiansResponse{
		Revoked:  make([]*identityv1.Guardianship, 0),
		Failures: make([]*identityv1.FailedGuardianshipFailure, 0),
	}

	for _, target := range req.GetTargets() {
		revokeReq := &identityv1.RevokeGuardianRequest{
			Target:   target,
			Reason:   req.GetReason(),
			Operator: req.GetOperator(),
		}

		revokeResp, err := s.RevokeGuardian(ctx, revokeReq)
		if err != nil {
			resp.Failures = append(resp.Failures, &identityv1.FailedGuardianshipFailure{
				Target: target,
				Error:  err.Error(),
			})
			continue
		}
		if revokeResp != nil && revokeResp.Guardianship != nil {
			resp.Revoked = append(resp.Revoked, revokeResp.Guardianship)
		}
	}

	return resp, nil
}

// ImportGuardians 批量导入监护关系
func (s *guardianshipCommandServer) ImportGuardians(ctx context.Context, req *identityv1.ImportGuardiansRequest) (*identityv1.ImportGuardiansResponse, error) {
	if req == nil || len(req.GetRecords()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "records is required")
	}

	resp := &identityv1.ImportGuardiansResponse{
		Created:  make([]*identityv1.Guardianship, 0),
		Failures: make([]*identityv1.FailedImportGuardian, 0),
	}

	for _, record := range req.GetRecords() {
		addReq := &identityv1.AddGuardianRequest{
			UserId:   record.GetUserId(),
			ChildId:  record.GetChildId(),
			Relation: record.GetRelation(),
			Operator: req.GetOperator(),
		}

		addResp, err := s.AddGuardian(ctx, addReq)
		if err != nil {
			resp.Failures = append(resp.Failures, &identityv1.FailedImportGuardian{
				Record: record,
				Error:  err.Error(),
			})
			continue
		}

		if addResp != nil && addResp.Guardianship != nil {
			resp.Created = append(resp.Created, addResp.Guardianship)
		}
	}

	return resp, nil
}

// protoRelationToString 将 proto 枚举转换为字符串
func protoRelationToString(relation identityv1.GuardianshipRelation) string {
	switch relation {
	case identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_SELF:
		return "self"
	case identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_PARENT:
		return "parent"
	case identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_GRANDPARENT:
		return guardianshipApp.NormalizeRelation("grandparent")
	case identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_OTHER:
		return "other"
	default:
		return "other"
	}
}

// stringToProtoRelation 将字符串转换为 proto 枚举
func stringToProtoRelation(relation string) identityv1.GuardianshipRelation {
	switch relation {
	case "self":
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_SELF
	case "parent":
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_PARENT
	case "grandparent":
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_GRANDPARENT
	case "other":
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_OTHER
	default:
		return identityv1.GuardianshipRelation_GUARDIANSHIP_RELATION_UNSPECIFIED
	}
}
