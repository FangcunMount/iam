package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
)

// HasProfileLink 判定是否为关系用户
func (s *profileLinkQueryServer) HasProfileLink(ctx context.Context, req *identityv1.HasProfileLinkRequest) (*identityv1.HasProfileLinkResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and profile_id are required")
	}

	hasProfileLink, err := s.profileLinkQuerySvc.IsLinked(ctx, req.GetUserId(), req.GetProfileId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &identityv1.HasProfileLinkResponse{HasProfileLink: hasProfileLink}

	// 如果是关系用户，返回档案关系详情
	if hasProfileLink {
		profileLink, err := s.profileLinkQuerySvc.Get(ctx, req.GetUserId(), req.GetProfileId())
		if err == nil && profileLink != nil {
			resp.ProfileLink = profileLinkResultToProto(profileLink)
		}
	}

	return resp, nil
}

// ListProfiles 列出关系的档案
func (s *profileLinkQueryServer) ListProfiles(ctx context.Context, req *identityv1.ListProfilesRequest) (*identityv1.ListProfilesResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	profileLinks, err := s.profileLinkQuerySvc.ListProfilesForUser(ctx, req.GetUserId())
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

	total := len(profileLinks)
	items := make([]*identityv1.ProfileEdge, 0)

	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		for _, g := range profileLinks[offset:end] {
			items = append(items, &identityv1.ProfileEdge{
				Profile:     profileResultToProtoFromProfileLink(g),
				ProfileLink: profileLinkResultToProto(g),
			})
		}
	}

	return &identityv1.ListProfilesResponse{
		Total: int32(total),
		Page:  req.GetPage(),
		Items: items,
	}, nil
}

// ListProfileLinks 列出档案的所有关系用户
func (s *profileLinkQueryServer) ListProfileLinks(ctx context.Context, req *identityv1.ListProfileLinksRequest) (*identityv1.ListProfileLinksResponse, error) {
	if req == nil || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}

	profileLinks, err := s.profileLinkQuerySvc.ListLinksForProfile(ctx, req.GetProfileId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*identityv1.ProfileLinkEdge, 0, len(profileLinks))
	for _, g := range profileLinks {
		// 查询关系用户详细信息
		var user *identityv1.User
		if g.UserID != "" {
			userResult, err := s.userQuerySvc.GetByID(ctx, g.UserID)
			if err == nil && userResult != nil {
				user = userResultToProto(userResult)
			}
		}

		items = append(items, &identityv1.ProfileLinkEdge{
			ProfileLink: profileLinkResultToProto(g),
			User:        user,
		})
	}

	return &identityv1.ListProfileLinksResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}
