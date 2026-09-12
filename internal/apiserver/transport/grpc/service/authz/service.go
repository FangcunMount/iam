package authz

import (
	"context"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	assignmentadmission "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignmentadmission"
	authzapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	authorizationdomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	iamgrpc "github.com/FangcunMount/iam/v5/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authorizationChecker interface {
	Check(context.Context, authorizationdomain.Request) (authorizationdomain.Decision, error)
}

type authorizationSnapshotReader interface {
	Read(context.Context, subject.Ref, string) (authzapp.SubjectSnapshot, error)
}

type Service struct{ srv authorizationServer }

func NewService(
	checker authorizationChecker,
	snapshotReader authorizationSnapshotReader,
	assignments assignmentApp.NamedCommands,
	assignmentPolicy assignmentadmission.Policy,
) *Service {
	return &Service{srv: authorizationServer{
		checker: checker, snapshotReader: snapshotReader,
		assignments: assignments, assignmentAdmission: assignmentPolicy,
	}}
}

func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	authzv4.RegisterAuthorizationServiceServer(server, &s.srv)
}

type authorizationServer struct {
	authzv4.UnimplementedAuthorizationServiceServer
	checker             authorizationChecker
	snapshotReader      authorizationSnapshotReader
	assignments         assignmentApp.NamedCommands
	assignmentAdmission assignmentadmission.Policy
}

func (s *authorizationServer) Check(ctx context.Context, req *authzv4.CheckRequest) (*authzv4.CheckResponse, error) {
	_, err := requireServiceIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if s.checker == nil {
		return nil, status.Error(codes.Unavailable, "authorization runtime is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Resource) == "" || strings.TrimSpace(req.Action) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, resource, and action are required")
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.ObjectContext != nil && (req.ObjectContext.ObjectId != "" || len(req.ObjectContext.Attributes) != 0 || len(req.ObjectContext.ProtoReflect().GetUnknown()) != 0) {
		return nil, status.Error(codes.InvalidArgument, "conditional authorization has been retired")
	}
	request, err := authorizationdomain.NewRequest(sub, req.Resource, req.Action)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	decision, err := s.checker.Check(ctx, request)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv4.CheckResponse{
		Allowed: decision.Allowed, Reason: toProtoReason(decision.Reason), DenyCode: decision.DenyCode,
		MatchedGrantId: decision.MatchedGrantID.String(), MatchedRole: decision.MatchedRole,
		PolicyVersion: decision.PolicyVersion,
	}, nil
}

func (s *authorizationServer) GetAuthorizationSnapshot(ctx context.Context, req *authzv4.GetAuthorizationSnapshotRequest) (*authzv4.GetAuthorizationSnapshotResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.snapshotReader == nil {
		return nil, status.Error(codes.Unavailable, "authorization snapshot service is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.AppName) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and app_name are required")
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.IncludeAssignmentFacts {
		caller, _ := requireServiceIdentity(ctx)
		_, err := admitAssignmentReplacement(ctx, s.assignmentAdmission, assignmentadmission.ReplacementRequest{Subject: sub, DelegatedActor: "service:" + caller})
		if err != nil {
			return nil, err
		}
	}
	snapshot, err := s.snapshotReader.Read(ctx, sub, req.AppName)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	// Field numbers stay stable: roles and direct_roles both expose the app-
	// filtered direct assignment set after inheritance retirement.
	response := &authzv4.GetAuthorizationSnapshotResponse{
		Roles: snapshot.DirectRoles, DirectRoles: snapshot.DirectRoles, ScopeContractVersion: 1,
		Permissions:   toProtoPermissions(snapshot.Permissions),
		PolicyVersion: snapshot.PolicyVersion,
	}
	if req.IncludeAssignmentFacts {
		if !snapshot.AssignmentFactsComplete {
			return nil, status.Error(codes.Unavailable, "complete assignment facts unavailable")
		}
		for _, f := range snapshot.AssignmentFacts {
			response.AssignmentFacts = append(response.AssignmentFacts, &authzv4.AssignmentRoleFact{RoleId: f.RoleID, RoleName: f.RoleName, ManagementProtection: f.ManagementProtection})
		}
		for _, f := range snapshot.AssignmentScopes {
			item := &authzv4.AssignmentScopeFact{AssignmentId: f.AssignmentID, Role: &authzv4.AssignmentRoleFact{RoleId: f.Role.RoleID, RoleName: f.Role.RoleName, ManagementProtection: f.Role.ManagementProtection}}
			if f.Scope != nil {
				item.Scope = toProtoDataScope(*f.Scope)
			}
			response.AssignmentScopes = append(response.AssignmentScopes, item)
		}
		response.AssignmentFactsComplete = true
	}
	return response, nil
}

func (s *authorizationServer) GrantAssignment(ctx context.Context, req *authzv4.GrantAssignmentRequest) (*authzv4.GrantAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and role_name are required")
	}
	admissionRequest, err := newAssignmentAdmissionRequest(
		assignmentadmission.OperationGrant, req.Subject, req.RoleName, req.GrantedBy,
	)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := admitAssignmentRequest(ctx, s.assignmentAdmission, admissionRequest); err != nil {
		return nil, err
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := assignmentApp.NewGrantByRoleNameCommand(sub, req.RoleName, req.GrantedBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	policyVersion, err := s.assignments.GrantByRoleName(authenticatedManagementContext(ctx), cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv4.GrantAssignmentResponse{PolicyVersion: policyVersion}, nil
}

func (s *authorizationServer) RevokeAssignment(ctx context.Context, req *authzv4.RevokeAssignmentRequest) (*authzv4.RevokeAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and role_name are required")
	}
	admissionRequest, err := newAssignmentAdmissionRequest(
		assignmentadmission.OperationRevoke, req.Subject, req.RoleName, req.RevokedBy,
	)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	callerService, err := admitAssignmentRequest(ctx, s.assignmentAdmission, admissionRequest)
	if err != nil {
		return nil, err
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := assignmentApp.NewRevokeByRoleNameCommand(sub, req.RoleName, revokeActor(req.RevokedBy, callerService), req.Reason)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	policyVersion, err := s.assignments.RevokeByRoleName(authenticatedManagementContext(ctx), cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv4.RevokeAssignmentResponse{PolicyVersion: policyVersion}, nil
}

func (s *authorizationServer) ReplaceManagedAssignments(ctx context.Context, req *authzv4.ReplaceManagedAssignmentsRequest) (*authzv4.ReplaceManagedAssignmentsResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.ChangedBy) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and changed_by are required")
	}
	replacementRequest, err := replacementAdmissionRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	managedRoles, err := admitAssignmentReplacement(ctx, s.assignmentAdmission, replacementRequest)
	if err != nil {
		return nil, err
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := assignmentApp.NewReplaceManagedAssignmentsCommand(
		sub, req.RoleNames, managedRoles, req.ChangedBy, req.Reason,
	)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	result, err := s.assignments.ReplaceManagedAssignments(authenticatedManagementContext(ctx), cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv4.ReplaceManagedAssignmentsResponse{
		DirectRoles: result.DirectRoles, PolicyVersion: result.PolicyVersion, Changed: result.Changed,
	}, nil
}

func requireServiceIdentity(ctx context.Context) (string, error) {
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || strings.TrimSpace(identity.ServiceName) == "" {
		return "", status.Error(codes.PermissionDenied, "service identity is required")
	}
	return strings.TrimSpace(identity.ServiceName), nil
}

func revokeActor(revokedBy, callerService string) string {
	if value := strings.TrimSpace(revokedBy); value != "" {
		return value
	}
	return "service:" + callerService
}

func admitAssignmentRequest(ctx context.Context, policy assignmentadmission.Policy, request assignmentadmission.Request) (string, error) {
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || strings.TrimSpace(identity.ServiceName) == "" {
		recordAssignmentAuthorization("unknown", string(request.Operation), "denied")
		return "", status.Error(codes.PermissionDenied, "assignment caller identity is required")
	}
	request.CallerService = strings.TrimSpace(identity.ServiceName)
	if policy == nil {
		recordAssignmentAuthorization(request.CallerService, string(request.Operation), "failed")
		return "", status.Error(codes.Internal, "managed assignment constraints are unavailable")
	}
	if err := policy.AuthorizeAssignment(request); err != nil {
		var denied *assignmentadmission.DeniedError
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

func admitAssignmentReplacement(ctx context.Context, policy assignmentadmission.Policy, request assignmentadmission.ReplacementRequest) ([]string, error) {
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || strings.TrimSpace(identity.ServiceName) == "" {
		recordAssignmentAuthorization("unknown", string(assignmentadmission.OperationReplace), "denied")
		return nil, status.Error(codes.PermissionDenied, "assignment caller identity is required")
	}
	request.CallerService = strings.TrimSpace(identity.ServiceName)
	if policy == nil {
		recordAssignmentAuthorization(request.CallerService, string(assignmentadmission.OperationReplace), "failed")
		return nil, status.Error(codes.Internal, "managed assignment constraints are unavailable")
	}
	managedRoles, err := policy.AuthorizeReplacement(request)
	if err != nil {
		var denied *assignmentadmission.DeniedError
		if errors.As(err, &denied) {
			recordAssignmentAuthorization(request.CallerService, string(assignmentadmission.OperationReplace), "denied")
			return nil, status.Error(codes.PermissionDenied, "assignment request is not allowed")
		}
		recordAssignmentAuthorization(request.CallerService, string(assignmentadmission.OperationReplace), "failed")
		return nil, status.Error(codes.Internal, "assignment authorization failed")
	}
	recordAssignmentAuthorization(request.CallerService, string(assignmentadmission.OperationReplace), "allowed")
	return managedRoles, nil
}

func newAssignmentAdmissionRequest(
	operation assignmentadmission.Operation,
	subjectValue, roleNameValue, delegatedActor string,
) (assignmentadmission.Request, error) {
	sub, err := subject.ParseRef(subjectValue)
	if err != nil {
		return assignmentadmission.Request{}, err
	}
	roleName, err := role.NewName(roleNameValue)
	if err != nil {
		return assignmentadmission.Request{}, err
	}
	return assignmentadmission.Request{
		Operation:      operation,
		Subject:        sub,
		RoleName:       roleName,
		DelegatedActor: delegatedActor,
	}, nil
}

func replacementAdmissionRequest(req *authzv4.ReplaceManagedAssignmentsRequest) (assignmentadmission.ReplacementRequest, error) {
	sub, err := subject.ParseRef(req.Subject)
	if err != nil {
		return assignmentadmission.ReplacementRequest{}, err
	}
	roleNames := make([]role.Name, 0, len(req.RoleNames))
	for _, value := range req.RoleNames {
		roleName, err := role.NewName(value)
		if err != nil {
			return assignmentadmission.ReplacementRequest{}, err
		}
		roleNames = append(roleNames, roleName)
	}
	return assignmentadmission.ReplacementRequest{
		Subject:        sub,
		RoleNames:      roleNames,
		DelegatedActor: req.ChangedBy,
	}, nil
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

func toProtoPermissions(entries []authzapp.PermissionEntry) []*authzv4.PermissionEntry {
	permissions := make([]*authzv4.PermissionEntry, 0, len(entries))
	for _, entry := range entries {
		mode := authzv4.AuthorizationMode_UNCONDITIONAL
		item := &authzv4.PermissionEntry{Resource: entry.Resource, Action: entry.Action, Mode: mode}
		for _, value := range entry.Scopes {
			item.Scopes = append(item.Scopes, toProtoDataScope(value))
		}
		permissions = append(permissions, item)
	}
	return permissions
}

func toProtoReason(reason authorizationdomain.Reason) authzv4.DecisionReason {
	switch reason {
	case authorizationdomain.ReasonAllowed:
		return authzv4.DecisionReason_ALLOWED
	case authorizationdomain.ReasonNotMatched:
		return authzv4.DecisionReason_NOT_MATCHED
	default:
		return authzv4.DecisionReason_DECISION_REASON_UNSPECIFIED
	}
}

var _ authzv4.AuthorizationServiceServer = (*authorizationServer)(nil)
var _ authorizationChecker = (*authzapp.DecisionService)(nil)

func authenticatedManagementContext(ctx context.Context) context.Context {
	service, err := requireServiceIdentity(ctx)
	if err != nil {
		return ctx
	}
	return management.WithAuthenticatedService(ctx, service)
}

func toProtoDataScope(value scope.Scope) *authzv4.DataScope {
	kind := authzv4.DataScopeKind_DATA_SCOPE_KIND_UNSPECIFIED
	switch value.Kind() {
	case scope.AllStores:
		kind = authzv4.DataScopeKind_ALL_STORES
	case scope.Stores:
		kind = authzv4.DataScopeKind_STORES
	}
	ids := make([]string, 0, len(value.StoreIDs()))
	for _, id := range value.StoreIDs() {
		ids = append(ids, id.String())
	}
	return &authzv4.DataScope{OrgId: value.OrgID().String(), Kind: kind, StoreIds: ids}
}
