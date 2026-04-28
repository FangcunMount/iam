package grpc

import (
	"context"
	"strings"

	authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service 聚合 authz gRPC（PDP + snapshot/assignment facade）。
type Service struct {
	srv authorizationServer
}

// NewService 创建 authz gRPC 服务。
func NewService(
	casbin policyDomain.CasbinAdapter,
	roleRepo roleDomain.Repository,
	versionRepo policyDomain.Repository,
	assignmentCommander assignmentDomain.Commander,
) *Service {
	return &Service{
		srv: authorizationServer{
			casbin:              casbin,
			roleRepo:            roleRepo,
			versionRepo:         versionRepo,
			assignmentCommander: assignmentCommander,
		},
	}
}

// Register 注册到 gRPC Server。
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	authzv1.RegisterAuthorizationServiceServer(server, &s.srv)
}

type authorizationServer struct {
	authzv1.UnimplementedAuthorizationServiceServer
	casbin              policyDomain.CasbinAdapter
	roleRepo            roleDomain.Repository
	versionRepo         policyDomain.Repository
	assignmentCommander assignmentDomain.Commander
}

func (s *authorizationServer) Check(ctx context.Context, req *authzv1.CheckRequest) (*authzv1.CheckResponse, error) {
	if s.casbin == nil {
		return nil, status.Error(codes.Unavailable, "authorization engine not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.Object == "" || req.Action == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, object, action are required")
	}
	ok, err := s.casbin.Enforce(ctx, req.Subject, req.Domain, req.Object, req.Action)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "enforce: %v", err)
	}
	return &authzv1.CheckResponse{Allowed: ok}, nil
}

func (s *authorizationServer) GetAuthorizationSnapshot(ctx context.Context, req *authzv1.GetAuthorizationSnapshotRequest) (*authzv1.GetAuthorizationSnapshotResponse, error) {
	if s.casbin == nil {
		return nil, status.Error(codes.Unavailable, "authorization engine not available")
	}
	if s.versionRepo == nil {
		return nil, status.Error(codes.Unavailable, "authorization version repository not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.AppName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, app_name are required")
	}

	roleKeys, err := s.casbin.GetImplicitRolesForUser(ctx, req.Subject, req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get implicit roles: %v", err)
	}

	policyRules, err := s.casbin.GetImplicitPermissionsForUser(ctx, req.Subject, req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get implicit permissions: %v", err)
	}

	version, err := s.versionRepo.GetOrCreate(ctx, req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get authz version: %v", err)
	}

	roles := filterSnapshotRoles(roleKeys, req.AppName)
	permissions := filterSnapshotPermissions(policyRules, req.AppName)

	return &authzv1.GetAuthorizationSnapshotResponse{
		Roles:        roles,
		Permissions:  permissions,
		AuthzVersion: version.Version,
	}, nil
}

func (s *authorizationServer) GrantAssignment(ctx context.Context, req *authzv1.GrantAssignmentRequest) (*authzv1.GrantAssignmentResponse, error) {
	if s.assignmentCommander == nil || s.roleRepo == nil {
		return nil, status.Error(codes.Unavailable, "assignment service not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, role_name are required")
	}

	subjectType, subjectID, err := parseSubject(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	role, err := s.roleRepo.FindByName(ctx, req.Domain, req.RoleName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "role not found: %v", err)
	}

	_, err = s.assignmentCommander.Grant(ctx, assignmentDomain.GrantCommand{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      role.ID.Uint64(),
		TenantID:    req.Domain,
		GrantedBy:   req.GrantedBy,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "grant assignment: %v", err)
	}

	return &authzv1.GrantAssignmentResponse{}, nil
}

func (s *authorizationServer) RevokeAssignment(ctx context.Context, req *authzv1.RevokeAssignmentRequest) (*authzv1.RevokeAssignmentResponse, error) {
	if s.assignmentCommander == nil || s.roleRepo == nil {
		return nil, status.Error(codes.Unavailable, "assignment service not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, role_name are required")
	}

	subjectType, subjectID, err := parseSubject(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	role, err := s.roleRepo.FindByName(ctx, req.Domain, req.RoleName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "role not found: %v", err)
	}

	if err := s.assignmentCommander.Revoke(ctx, assignmentDomain.RevokeCommand{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      role.ID.Uint64(),
		TenantID:    req.Domain,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke assignment: %v", err)
	}

	return &authzv1.RevokeAssignmentResponse{}, nil
}

func parseSubject(subject string) (assignmentDomain.SubjectType, string, error) {
	parts := strings.SplitN(subject, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", status.Error(codes.InvalidArgument, "subject must be in <type>:<id> format")
	}

	switch parts[0] {
	case string(assignmentDomain.SubjectTypeUser):
		return assignmentDomain.SubjectTypeUser, parts[1], nil
	default:
		return "", "", status.Errorf(codes.InvalidArgument, "unsupported subject type for assignment writes: %s", parts[0])
	}
}

func filterSnapshotRoles(roleKeys []string, appName string) []string {
	seen := make(map[string]struct{}, len(roleKeys))
	roles := make([]string, 0, len(roleKeys))
	prefix := "role:"
	appPrefix := appName + ":"

	for _, roleKey := range roleKeys {
		roleName := strings.TrimPrefix(roleKey, prefix)
		if !strings.HasPrefix(roleName, appPrefix) {
			continue
		}
		if _, exists := seen[roleName]; exists {
			continue
		}
		seen[roleName] = struct{}{}
		roles = append(roles, roleName)
	}

	return roles
}

func filterSnapshotPermissions(policyRules []policyDomain.PolicyRule, appName string) []*authzv1.PermissionEntry {
	seen := make(map[string]struct{}, len(policyRules))
	permissions := make([]*authzv1.PermissionEntry, 0, len(policyRules))
	appPrefix := appName + ":"

	for _, rule := range policyRules {
		if !strings.HasPrefix(rule.Obj, appPrefix) {
			continue
		}

		key := rule.Obj + "\x00" + rule.Act
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		permissions = append(permissions, &authzv1.PermissionEntry{
			Resource: rule.Obj,
			Action:   rule.Act,
		})
	}

	return permissions
}

var (
	_ authzv1.AuthorizationServiceServer = (*authorizationServer)(nil)
	_ meta.ID                            = 0
)
