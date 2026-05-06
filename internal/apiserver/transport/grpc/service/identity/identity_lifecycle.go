package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	userApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
)

// CreateUser 创建用户
func (s *identityLifecycleServer) CreateUser(ctx context.Context, req *identityv2.CreateUserRequest) (*identityv2.CreateUserResponse, error) {
	if req == nil || strings.TrimSpace(req.GetNickname()) == "" {
		return nil, status.Error(codes.InvalidArgument, "nickname is required")
	}

	dto := userApp.CreateUserDTO{
		Name:  req.GetNickname(),
		Phone: req.GetPhone(),
		Email: req.GetEmail(),
	}

	result, err := s.userSvc.Create(ctx, dto)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.CreateUserResponse{
		User: userResultToProto(result),
	}, nil
}

// UpdateUser 更新用户
func (s *identityLifecycleServer) UpdateUser(ctx context.Context, req *identityv2.UpdateUserRequest) (*identityv2.UpdateUserResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// 更新昵称
	if req.GetNickname() != "" {
		err := s.userProfileSvc.Rename(ctx, req.GetUserId(), req.GetNickname())
		if err != nil {
			return nil, toGRPCError(err)
		}
	}

	// 更新联系方式
	if req.GetPhone() != "" || req.GetEmail() != "" {
		dto := userApp.UpdateContactDTO{
			UserID: req.GetUserId(),
			Phone:  req.GetPhone(),
			Email:  req.GetEmail(),
		}
		err := s.userProfileSvc.UpdateContact(ctx, dto)
		if err != nil {
			return nil, toGRPCError(err)
		}
	}

	// 查询更新后的用户
	result, err := s.userQuerySvc.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &identityv2.UpdateUserResponse{
		User: userResultToProto(result),
	}, nil
}

// DeactivateUser 停用用户
func (s *identityLifecycleServer) DeactivateUser(ctx context.Context, req *identityv2.ChangeUserStatusRequest) (*identityv2.UserOperationResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	err := s.userStatusSvc.Deactivate(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return s.buildUserOperationResponse(ctx, req.GetUserId())
}

// BlockUser 封禁用户
func (s *identityLifecycleServer) BlockUser(ctx context.Context, req *identityv2.ChangeUserStatusRequest) (*identityv2.UserOperationResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	err := s.userStatusSvc.Block(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return s.buildUserOperationResponse(ctx, req.GetUserId())
}

func (s *identityLifecycleServer) buildUserOperationResponse(ctx context.Context, userID string) (*identityv2.UserOperationResponse, error) {
	result, err := s.userQuerySvc.GetByID(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &identityv2.UserOperationResponse{
		User: userResultToProto(result),
	}, nil
}
