package authz

import (
	"context"
	"strings"

	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	rolebindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	iamgrpc "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authorizationChecker interface {
	Check(ctx context.Context, cmd authzapp.CheckCommand) (authzDomain.AuthorizationDecision, error)
}

type authorizationSnapshotReader interface {
	Read(ctx context.Context, query authzapp.SnapshotQuery) (*authzapp.Snapshot, error)
}

// Service 聚合 authz gRPC（PDP + snapshot/assignment facade）。
type Service struct {
	srv authorizationServer
}

// NewService 创建 authz gRPC 服务。
func NewService(
	checker authorizationChecker,
	snapshotReader authorizationSnapshotReader,
	roleBindings rolebindingApp.NamedCommands,
) *Service {
	return &Service{
		srv: authorizationServer{
			checker:        checker,
			snapshotReader: snapshotReader,
			roleBindings:   roleBindings,
		},
	}
}

// Register 注册到 gRPC Server。
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	authzv2.RegisterAuthorizationServiceServer(server, &s.srv)
}

type authorizationServer struct {
	authzv2.UnimplementedAuthorizationServiceServer
	checker        authorizationChecker
	snapshotReader authorizationSnapshotReader
	roleBindings   rolebindingApp.NamedCommands
}

func (s *authorizationServer) Check(ctx context.Context, req *authzv2.CheckRequest) (*authzv2.CheckResponse, error) {
	if s.checker == nil {
		return nil, status.Error(codes.Unavailable, "authorization engine not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.Object == "" || req.Action == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, object, action are required")
	}
	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	scope, err := authzDomain.NormalizeScope(req.GetScopeType(), req.GetScopeValue())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	decision, err := s.checker.Check(ctx, authzapp.CheckCommand{
		Subject:     subject,
		TenantID:    req.Domain,
		ResourceKey: req.Object,
		Action:      req.Action,
		ObjectScope: scope,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "enforce: %v", err)
	}
	return &authzv2.CheckResponse{Allowed: decision.Allowed}, nil
}

func (s *authorizationServer) GetAuthorizationSnapshot(ctx context.Context, req *authzv2.GetAuthorizationSnapshotRequest) (*authzv2.GetAuthorizationSnapshotResponse, error) {
	if s.snapshotReader == nil {
		return nil, status.Error(codes.Unavailable, "authorization snapshot service not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.AppName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, app_name are required")
	}

	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	snapshot, err := s.snapshotReader.Read(ctx, authzapp.SnapshotQuery{
		Subject:  subject,
		TenantID: req.Domain,
		AppName:  req.AppName,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get authorization snapshot: %v", err)
	}

	return &authzv2.GetAuthorizationSnapshotResponse{
		Roles:        snapshot.Roles,
		Permissions:  toProtoPermissions(snapshot.Permissions),
		AuthzVersion: snapshot.AuthzVersion,
	}, nil
}

func (s *authorizationServer) GrantAssignment(ctx context.Context, req *authzv2.GrantAssignmentRequest) (*authzv2.GrantAssignmentResponse, error) {
	if s.roleBindings == nil {
		return nil, status.Error(codes.Unavailable, "assignment service not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, role_name are required")
	}

	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.GrantByRoleName(ctx, rolebindingApp.GrantByRoleNameCommand{
		Subject: subject, TenantID: req.Domain, RoleName: req.RoleName, GrantedBy: req.GrantedBy,
	}); err != nil {
		return nil, authzGRPCError(codes.Internal, "grant assignment", err)
	}

	return &authzv2.GrantAssignmentResponse{}, nil
}

func (s *authorizationServer) RevokeAssignment(ctx context.Context, req *authzv2.RevokeAssignmentRequest) (*authzv2.RevokeAssignmentResponse, error) {
	if s.roleBindings == nil {
		return nil, status.Error(codes.Unavailable, "assignment service not available")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, role_name are required")
	}

	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.RevokeByRoleName(ctx, rolebindingApp.RevokeByRoleNameCommand{
		Subject: subject, TenantID: req.Domain, RoleName: req.RoleName, ChangedBy: revokeActor(req.GetRevokedBy()), Reason: req.GetReason(),
	}); err != nil {
		return nil, authzGRPCError(codes.Internal, "revoke assignment", err)
	}

	return &authzv2.RevokeAssignmentResponse{}, nil
}

func revokeActor(revokedBy string) string {
	revokedBy = strings.TrimSpace(revokedBy)
	if revokedBy == "" {
		return "system"
	}
	return revokedBy
}

func parseSubjectKey(subject string) (authzDomain.Subject, error) {
	parts := strings.SplitN(subject, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return authzDomain.Subject{}, status.Error(codes.InvalidArgument, "subject must be in <type>:<id> format")
	}
	id, err := meta.ParseID(parts[1])
	if err != nil || id.IsZero() {
		return authzDomain.Subject{}, status.Error(codes.InvalidArgument, "subject id must be a valid IAM id")
	}
	return authzDomain.NewSubject(authzDomain.SubjectType(parts[0]), id)
}

func authzGRPCError(defaultCode codes.Code, operation string, err error) error {
	if coded, ok := iamgrpc.CodedStatusError(err); ok {
		return coded
	}
	return status.Errorf(defaultCode, "%s: %v", operation, err)
}

func toProtoPermissions(entries []authzapp.PermissionEntry) []*authzv2.PermissionEntry {
	permissions := make([]*authzv2.PermissionEntry, 0, len(entries))
	for _, entry := range entries {
		scope := entry.Scope.Normalized()
		permissions = append(permissions, &authzv2.PermissionEntry{
			Resource:   entry.ResourceKey,
			Action:     entry.Action,
			ScopeType:  string(scope.Kind),
			ScopeValue: scope.Value,
		})
	}
	return permissions
}

var (
	_ authzv2.AuthorizationServiceServer = (*authorizationServer)(nil)
)
