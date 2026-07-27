package authz

import (
	"context"
	"errors"
	"strings"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	assignmentauth "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/assignmentauth"
	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	rolebindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	iamgrpc "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authorizationChecker interface {
	Check(ctx context.Context, cmd authzapp.CheckCommand) (decision.Decision, error)
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
	assignmentAuthorizers ...assignmentauth.Authorizer,
) *Service {
	var assignmentAuthorizer assignmentauth.Authorizer
	if len(assignmentAuthorizers) > 0 {
		assignmentAuthorizer = assignmentAuthorizers[0]
	}
	return &Service{
		srv: authorizationServer{
			checker:              checker,
			snapshotReader:       snapshotReader,
			roleBindings:         roleBindings,
			assignmentAuthorizer: assignmentAuthorizer,
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
	checker              authorizationChecker
	snapshotReader       authorizationSnapshotReader
	roleBindings         rolebindingApp.NamedCommands
	assignmentAuthorizer assignmentauth.Authorizer
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
	scopeValue, err := scope.Normalize(req.GetScopeType(), req.GetScopeValue())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := authzapp.NewCheckCommand(subject, req.Domain, req.Object, req.Action, scopeValue)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	decision, err := s.checker.Check(ctx, cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv2.CheckResponse{
		Allowed:       decision.Allowed,
		Reason:        string(decision.Reason),
		DenyCode:      decision.DenyCode,
		PolicyVersion: decision.PolicyVersion,
	}, nil
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
	query, err := authzapp.NewSnapshotQuery(subject, req.Domain, req.AppName)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	snapshot, err := s.snapshotReader.Read(ctx, query)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
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
	_, err := authorizeAssignmentRequest(ctx, s.assignmentAuthorizer, assignmentauth.Request{
		Operation:      assignmentauth.OperationGrant,
		Subject:        req.Subject,
		Domain:         req.Domain,
		RoleName:       req.RoleName,
		DelegatedActor: req.GrantedBy,
	})
	if err != nil {
		return nil, err
	}

	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := rolebindingApp.NewGrantByRoleNameCommand(subject, req.Domain, req.RoleName, req.GrantedBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.GrantByRoleName(ctx, cmd); err != nil {
		return nil, iamgrpc.ToStatusError(err)
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
	callerService, err := authorizeAssignmentRequest(ctx, s.assignmentAuthorizer, assignmentauth.Request{
		Operation:      assignmentauth.OperationRevoke,
		Subject:        req.Subject,
		Domain:         req.Domain,
		RoleName:       req.RoleName,
		DelegatedActor: req.GetRevokedBy(),
	})
	if err != nil {
		return nil, err
	}

	subject, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := rolebindingApp.NewRevokeByRoleNameCommand(subject, req.Domain, req.RoleName, revokeActor(req.GetRevokedBy(), callerService), req.GetReason())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.RevokeByRoleName(ctx, cmd); err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}

	return &authzv2.RevokeAssignmentResponse{}, nil
}

func revokeActor(revokedBy, callerService string) string {
	revokedBy = strings.TrimSpace(revokedBy)
	if revokedBy == "" {
		if callerService != "" {
			return "service:" + callerService
		}
		return "system"
	}
	return revokedBy
}

func authorizeAssignmentRequest(
	ctx context.Context,
	authorizer assignmentauth.Authorizer,
	request assignmentauth.Request,
) (string, error) {
	if authorizer == nil {
		recordAssignmentAuthorization("unknown", string(request.Operation), "skipped")
		return "", nil
	}
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || strings.TrimSpace(identity.ServiceName) == "" {
		recordAssignmentAuthorization("unknown", string(request.Operation), "denied")
		return "", status.Error(codes.PermissionDenied, "assignment caller identity is required")
	}
	request.CallerService = strings.TrimSpace(identity.ServiceName)
	if err := authorizer.AuthorizeAssignment(request); err != nil {
		var denied *assignmentauth.DeniedError
		if errors.As(err, &denied) {
			recordAssignmentAuthorization(request.CallerService, string(request.Operation), "denied")
			return "", status.Error(codes.PermissionDenied, "assignment request is not allowed")
		}
		recordAssignmentAuthorization(request.CallerService, string(request.Operation), "failed")
		return "", status.Error(codes.Internal, "assignment authorization failed")
	}
	recordAssignmentAuthorization(request.CallerService, string(request.Operation), "allowed")
	return request.CallerService, nil
}

func parseSubjectKey(value string) (subject.Ref, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return subject.Ref{}, status.Error(codes.InvalidArgument, "subject must be in <type>:<id> format")
	}
	id, err := meta.ParseID(parts[1])
	if err != nil || id.IsZero() {
		return subject.Ref{}, status.Error(codes.InvalidArgument, "subject id must be a valid IAM id")
	}
	return subject.NewRef(subject.Type(parts[0]), id)
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
