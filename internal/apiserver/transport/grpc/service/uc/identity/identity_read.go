package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
)

// GetUser 查询用户
func (s *identityReadServer) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	result, err := s.userQuerySvc.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv1.GetUserResponse{User: userResultToProto(result)}, nil
}

// BatchGetUsers 批量查询用户
func (s *identityReadServer) BatchGetUsers(ctx context.Context, req *identityv1.BatchGetUsersRequest) (*identityv1.BatchGetUsersResponse, error) {
	if req == nil || len(req.GetUserIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_ids is required")
	}

	resp := &identityv1.BatchGetUsersResponse{
		Users:       make([]*identityv1.User, 0, len(req.GetUserIds())),
		NotFoundIds: make([]string, 0),
	}

	for _, userID := range req.GetUserIds() {
		result, err := s.userQuerySvc.GetByID(ctx, userID)
		if err != nil {
			// 如果是未找到错误，添加到 not_found 列表
			resp.NotFoundIds = append(resp.NotFoundIds, userID)
			continue
		}
		resp.Users = append(resp.Users, userResultToProto(result))
	}

	return resp, nil
}

// SearchUsers 搜索用户
func (s *identityReadServer) SearchUsers(ctx context.Context, req *identityv1.SearchUsersRequest) (*identityv1.SearchUsersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	// 目前只支持通过手机号精确查询
	if len(req.GetPhones()) > 0 {
		return s.searchUsersByPhones(ctx, req)
	}

	// 其他搜索条件暂不支持
	return &identityv1.SearchUsersResponse{
		Total: 0,
		Page:  req.GetPage(),
		Users: []*identityv1.User{},
	}, nil
}

// searchUsersByPhones 通过手机号列表搜索用户
func (s *identityReadServer) searchUsersByPhones(ctx context.Context, req *identityv1.SearchUsersRequest) (*identityv1.SearchUsersResponse, error) {
	users := make([]*identityv1.User, 0)

	for _, phone := range req.GetPhones() {
		result, err := s.userQuerySvc.GetByPhone(ctx, phone)
		if err != nil {
			// 忽略未找到的错误
			continue
		}
		users = append(users, userResultToProto(result))
	}

	return &identityv1.SearchUsersResponse{
		Total: int32(len(users)),
		Page:  req.GetPage(),
		Users: users,
	}, nil
}

// GetChild 查询儿童档案
func (s *identityReadServer) GetChild(ctx context.Context, req *identityv1.GetChildRequest) (*identityv1.GetChildResponse, error) {
	if req == nil || strings.TrimSpace(req.GetChildId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "child_id is required")
	}

	result, err := s.childQuerySvc.GetByID(ctx, req.GetChildId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv1.GetChildResponse{Child: childResultToProto(result)}, nil
}

// BatchGetChildren 批量查询儿童档案
func (s *identityReadServer) BatchGetChildren(ctx context.Context, req *identityv1.BatchGetChildrenRequest) (*identityv1.BatchGetChildrenResponse, error) {
	if req == nil || len(req.GetChildIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "child_ids is required")
	}

	resp := &identityv1.BatchGetChildrenResponse{
		Children:    make([]*identityv1.Child, 0, len(req.GetChildIds())),
		NotFoundIds: make([]string, 0),
	}

	for _, childID := range req.GetChildIds() {
		result, err := s.childQuerySvc.GetByID(ctx, childID)
		if err != nil {
			// 如果是未找到错误，添加到 not_found 列表
			resp.NotFoundIds = append(resp.NotFoundIds, childID)
			continue
		}
		resp.Children = append(resp.Children, childResultToProto(result))
	}

	return resp, nil
}
