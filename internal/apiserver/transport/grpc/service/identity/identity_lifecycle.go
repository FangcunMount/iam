package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	userApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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
	userID, err := parseIDArg("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}

	patch := userApp.PatchUserProfileDTO{UserID: userID}
	if req.GetNickname() != "" {
		value := req.GetNickname()
		patch.Nickname = &value
	}
	if req.GetPhone() != "" {
		value := req.GetPhone()
		patch.Phone = &value
	}
	if req.GetEmail() != "" {
		value := req.GetEmail()
		patch.Email = &value
	}
	result, err := s.userProfileSvc.PatchProfile(ctx, patch)
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
	userID, err := parseIDArg("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}

	err = s.userStatusSvc.Deactivate(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return s.buildUserOperationResponse(ctx, userID)
}

// BlockUser 封禁用户
func (s *identityLifecycleServer) BlockUser(ctx context.Context, req *identityv2.ChangeUserStatusRequest) (*identityv2.UserOperationResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	userID, err := parseIDArg("user_id", req.GetUserId())
	if err != nil {
		return nil, err
	}

	err = s.userStatusSvc.Block(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return s.buildUserOperationResponse(ctx, userID)
}

func (s *identityLifecycleServer) buildUserOperationResponse(ctx context.Context, userID meta.ID) (*identityv2.UserOperationResponse, error) {
	result, err := s.userQuerySvc.GetByID(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &identityv2.UserOperationResponse{
		User: userResultToProto(result),
	}, nil
}
