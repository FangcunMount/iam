package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
)

// IsGuardian 判定是否为监护人
func (s *guardianshipQueryServer) IsGuardian(ctx context.Context, req *identityv1.IsGuardianRequest) (*identityv1.IsGuardianResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" || strings.TrimSpace(req.GetChildId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and child_id are required")
	}

	isGuardian, err := s.guardianshipQuerySvc.IsGuardian(ctx, req.GetUserId(), req.GetChildId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &identityv1.IsGuardianResponse{IsGuardian: isGuardian}

	// 如果是监护人，返回监护关系详情
	if isGuardian {
		guardianship, err := s.guardianshipQuerySvc.GetByUserIDAndChildID(ctx, req.GetUserId(), req.GetChildId())
		if err == nil && guardianship != nil {
			resp.Guardianship = guardianshipResultToProto(guardianship)
		}
	}

	return resp, nil
}

// ListChildren 列出监护的儿童
func (s *guardianshipQueryServer) ListChildren(ctx context.Context, req *identityv1.ListChildrenRequest) (*identityv1.ListChildrenResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	guardianships, err := s.guardianshipQuerySvc.ListChildrenByUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	// 应用分页
	limit := int(req.GetPage().GetLimit())
	offset := int(req.GetPage().GetOffset())
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	total := len(guardianships)
	items := make([]*identityv1.ChildEdge, 0)

	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		for _, g := range guardianships[offset:end] {
			items = append(items, &identityv1.ChildEdge{
				Child:        childResultToProtoFromGuardianship(g),
				Guardianship: guardianshipResultToProto(g),
			})
		}
	}

	return &identityv1.ListChildrenResponse{
		Total: int32(total),
		Page:  req.GetPage(),
		Items: items,
	}, nil
}

// ListGuardians 列出儿童的所有监护人
func (s *guardianshipQueryServer) ListGuardians(ctx context.Context, req *identityv1.ListGuardiansRequest) (*identityv1.ListGuardiansResponse, error) {
	if req == nil || strings.TrimSpace(req.GetChildId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "child_id is required")
	}

	guardianships, err := s.guardianshipQuerySvc.ListGuardiansByChildID(ctx, req.GetChildId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*identityv1.GuardianshipEdge, 0, len(guardianships))
	for _, g := range guardianships {
		// 查询监护人详细信息
		var guardian *identityv1.User
		if g.UserID != "" {
			userResult, err := s.userQuerySvc.GetByID(ctx, g.UserID)
			if err == nil && userResult != nil {
				guardian = userResultToProto(userResult)
			}
		}

		items = append(items, &identityv1.GuardianshipEdge{
			Guardianship: guardianshipResultToProto(g),
			Guardian:     guardian,
		})
	}

	return &identityv1.ListGuardiansResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}
