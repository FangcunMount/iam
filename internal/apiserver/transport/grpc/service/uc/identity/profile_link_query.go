package identity

import (
	"context"
	"strings"

	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/profilelink"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
)

// HasProfileLink 判定是否为关系用户
func (s *profileLinkQueryServer) HasProfileLink(ctx context.Context, req *identityv2.HasProfileLinkRequest) (*identityv2.HasProfileLinkResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and profile_id are required")
	}

	hasProfileLink, err := s.profileLinkQuerySvc.IsLinked(ctx, req.GetUserId(), req.GetProfileId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &identityv2.HasProfileLinkResponse{HasProfileLink: hasProfileLink}

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
func (s *profileLinkQueryServer) ListProfiles(ctx context.Context, req *identityv2.ListProfilesRequest) (*identityv2.ListProfilesResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	var (
		profileLinks []*profileLinkApp.ProfileLinkResult
		err          error
	)
	if req.GetIncludeRevoked() {
		profileLinks, err = s.profileLinkQuerySvc.ListProfilesForUserIncludingRevoked(ctx, req.GetUserId())
	} else {
		profileLinks, err = s.profileLinkQuerySvc.ListProfilesForUser(ctx, req.GetUserId())
	}
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
	items := make([]*identityv2.ProfileEdge, 0)

	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		for _, g := range profileLinks[offset:end] {
			items = append(items, &identityv2.ProfileEdge{
				Profile:     profileResultToProtoFromProfileLink(g),
				ProfileLink: profileLinkResultToProto(g),
			})
		}
	}

	return &identityv2.ListProfilesResponse{
		Total: int32(total),
		Page:  req.GetPage(),
		Items: items,
	}, nil
}

// ListProfileLinks 列出档案的所有关系用户
func (s *profileLinkQueryServer) ListProfileLinks(ctx context.Context, req *identityv2.ListProfileLinksRequest) (*identityv2.ListProfileLinksResponse, error) {
	if req == nil || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}

	var (
		profileLinks []*profileLinkApp.ProfileLinkResult
		err          error
	)
	if req.GetIncludeRevoked() {
		profileLinks, err = s.profileLinkQuerySvc.ListLinksForProfileIncludingRevoked(ctx, req.GetProfileId())
	} else {
		profileLinks, err = s.profileLinkQuerySvc.ListLinksForProfile(ctx, req.GetProfileId())
	}
	if err != nil {
		return nil, toGRPCError(err)
	}

	usersByID, err := s.userQuerySvc.BatchGetByID(ctx, userIDsFromProfileLinks(profileLinks))
	if err != nil {
		return nil, toGRPCError(err)
	}
	items := make([]*identityv2.ProfileLinkEdge, 0, len(profileLinks))
	for _, g := range profileLinks {
		var user *identityv2.User
		if userResult := usersByID[g.UserID]; userResult != nil {
			user = userResultToProto(userResult)
		}

		items = append(items, &identityv2.ProfileLinkEdge{
			ProfileLink: profileLinkResultToProto(g),
			User:        user,
		})
	}

	return &identityv2.ListProfileLinksResponse{
		Total: int32(len(items)),
		Items: items,
	}, nil
}

func userIDsFromProfileLinks(profileLinks []*profileLinkApp.ProfileLinkResult) []string {
	ids := make([]string, 0, len(profileLinks))
	seen := make(map[string]struct{}, len(profileLinks))
	for _, link := range profileLinks {
		if link == nil || link.UserID == "" {
			continue
		}
		if _, ok := seen[link.UserID]; ok {
			continue
		}
		seen[link.UserID] = struct{}{}
		ids = append(ids, link.UserID)
	}
	return ids
}
