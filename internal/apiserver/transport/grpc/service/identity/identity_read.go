package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
)

// GetUser 查询用户
func (s *identityReadServer) GetUser(ctx context.Context, req *identityv2.GetUserRequest) (*identityv2.GetUserResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	userID, err := parseIDArg("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}

	result, err := s.userQuerySvc.GetByID(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.GetUserResponse{User: userResultToProto(result)}, nil
}

// BatchGetUsers 批量查询用户
func (s *identityReadServer) BatchGetUsers(ctx context.Context, req *identityv2.BatchGetUsersRequest) (*identityv2.BatchGetUsersResponse, error) {
	if req == nil || len(req.GetUserIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_ids is required")
	}

	resp := &identityv2.BatchGetUsersResponse{
		Users:       make([]*identityv2.User, 0, len(req.GetUserIds())),
		NotFoundIds: make([]string, 0),
	}

	userIDs, err := parseIDArgs("user_id", req.GetUserIds())
	if err != nil {
		return nil, err
	}
	usersByID, err := s.userQuerySvc.BatchGetByID(ctx, userIDs)
	if err != nil {
		return nil, toGRPCError(err)
	}
	for _, userID := range req.GetUserIds() {
		result := usersByID[userID]
		if result == nil {
			resp.NotFoundIds = append(resp.NotFoundIds, userID)
			continue
		}
		resp.Users = append(resp.Users, userResultToProto(result))
	}

	return resp, nil
}

// SearchUsers 搜索用户
func (s *identityReadServer) SearchUsers(ctx context.Context, req *identityv2.SearchUsersRequest) (*identityv2.SearchUsersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	// 目前只支持通过手机号精确查询
	if len(req.GetPhones()) > 0 {
		return s.searchUsersByPhones(ctx, req)
	}

	// 其他搜索条件暂不支持
	return &identityv2.SearchUsersResponse{
		Total: 0,
		Page:  req.GetPage(),
		Users: []*identityv2.User{},
	}, nil
}

// searchUsersByPhones 通过手机号列表搜索用户
func (s *identityReadServer) searchUsersByPhones(ctx context.Context, req *identityv2.SearchUsersRequest) (*identityv2.SearchUsersResponse, error) {
	users := make([]*identityv2.User, 0)

	for _, phone := range req.GetPhones() {
		result, err := s.userQuerySvc.GetByPhone(ctx, phone)
		if err != nil {
			// 忽略未找到的错误
			continue
		}
		users = append(users, userResultToProto(result))
	}

	return &identityv2.SearchUsersResponse{
		Total: int32(len(users)),
		Page:  req.GetPage(),
		Users: users,
	}, nil
}

// GetProfile 查询档案
func (s *identityReadServer) GetProfile(ctx context.Context, req *identityv2.GetProfileRequest) (*identityv2.GetProfileResponse, error) {
	if req == nil || strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := parseIDArg("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}

	result, err := s.profileQuerySvc.GetByID(ctx, profileID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.GetProfileResponse{Profile: profileResultToProto(result)}, nil
}

// BatchGetProfiles 批量查询档案
func (s *identityReadServer) BatchGetProfiles(ctx context.Context, req *identityv2.BatchGetProfilesRequest) (*identityv2.BatchGetProfilesResponse, error) {
	if req == nil || len(req.GetProfileIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "profile_ids is required")
	}

	resp := &identityv2.BatchGetProfilesResponse{
		Profiles:    make([]*identityv2.Profile, 0, len(req.GetProfileIds())),
		NotFoundIds: make([]string, 0),
	}

	for _, profileID := range req.GetProfileIds() {
		parsedProfileID, err := parseIDArg("profile_id", profileID)
		if err != nil {
			return nil, err
		}
		result, err := s.profileQuerySvc.GetByID(ctx, parsedProfileID)
		if err != nil {
			// 如果是未找到错误，添加到 not_found 列表
			resp.NotFoundIds = append(resp.NotFoundIds, profileID)
			continue
		}
		resp.Profiles = append(resp.Profiles, profileResultToProto(result))
	}

	return resp, nil
}
