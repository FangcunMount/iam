package authz

import (
	"context"
	"errors"
	"strings"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	assignmentadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentadmission"
	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	objectattributeadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	iamgrpc "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authorizationChecker interface {
	Check(context.Context, authorizationdomain.Request) (authorizationdomain.Decision, error)
}

type authorizationSnapshotReader interface {
	Read(context.Context, subject.Ref, string, string) (authzapp.SubjectSnapshot, error)
}

type Service struct{ srv authorizationServer }

func NewService(
	checker authorizationChecker,
	snapshotReader authorizationSnapshotReader,
	assignments assignmentApp.NamedCommands,
	assignmentPolicy assignmentadmission.Policy,
	objectAttributePolicy objectattributeadmission.Policy,
) *Service {
	return &Service{srv: authorizationServer{
		checker: checker, snapshotReader: snapshotReader,
		assignments: assignments, assignmentAdmission: assignmentPolicy,
		objectAttributeAdmission: objectAttributePolicy,
	}}
}

func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	authzv3.RegisterAuthorizationServiceServer(server, &s.srv)
}

type authorizationServer struct {
	authzv3.UnimplementedAuthorizationServiceServer
	checker                  authorizationChecker
	snapshotReader           authorizationSnapshotReader
	assignments              assignmentApp.NamedCommands
	assignmentAdmission      assignmentadmission.Policy
	objectAttributeAdmission objectattributeadmission.Policy
}

func (s *authorizationServer) Check(ctx context.Context, req *authzv3.CheckRequest) (*authzv3.CheckResponse, error) {
	callerService, err := requireServiceIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if s.checker == nil {
		return nil, status.Error(codes.Unavailable, "authorization runtime is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Domain) == "" || strings.TrimSpace(req.Resource) == "" || strings.TrimSpace(req.Action) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, resource, and action are required")
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	object, err := parseObjectContext(callerService, req.Resource, req.ObjectContext, s.objectAttributeAdmission)
	if err != nil {
		return nil, err
	}
	request, err := authorizationdomain.NewRequest(sub, req.Domain, req.Resource, req.Action, object)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	decision, err := s.checker.Check(ctx, request)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.CheckResponse{
		Allowed: decision.Allowed, Reason: toProtoReason(decision.Reason), DenyCode: decision.DenyCode,
		MatchedGrantId: decision.MatchedGrantID.String(), MatchedRole: decision.MatchedRole,
		PolicyVersion: decision.PolicyVersion, MissingAttributeKeys: decision.MissingAttributeKeys,
	}, nil
}

func (s *authorizationServer) GetAuthorizationSnapshot(ctx context.Context, req *authzv3.GetAuthorizationSnapshotRequest) (*authzv3.GetAuthorizationSnapshotResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.snapshotReader == nil {
		return nil, status.Error(codes.Unavailable, "authorization snapshot service is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Domain) == "" || strings.TrimSpace(req.AppName) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and app_name are required")
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	snapshot, err := s.snapshotReader.Read(ctx, sub, req.Domain, req.AppName)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.GetAuthorizationSnapshotResponse{
		Roles: snapshot.EffectiveRoles, DirectRoles: snapshot.DirectRoles,
		Permissions:   toProtoPermissions(snapshot.Permissions),
		PolicyVersion: snapshot.PolicyVersion,
	}, nil
}

func (s *authorizationServer) GrantAssignment(ctx context.Context, req *authzv3.GrantAssignmentRequest) (*authzv3.GrantAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and role_name are required")
	}
	admissionRequest, err := newAssignmentAdmissionRequest(
		assignmentadmission.OperationGrant, req.Subject, req.Domain, req.RoleName, req.GrantedBy,
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
	cmd, err := assignmentApp.NewGrantByRoleNameCommand(sub, req.Domain, req.RoleName, req.GrantedBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	policyVersion, err := s.assignments.GrantByRoleName(ctx, cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.GrantAssignmentResponse{PolicyVersion: policyVersion}, nil
}

func (s *authorizationServer) RevokeAssignment(ctx context.Context, req *authzv3.RevokeAssignmentRequest) (*authzv3.RevokeAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and role_name are required")
	}
	admissionRequest, err := newAssignmentAdmissionRequest(
		assignmentadmission.OperationRevoke, req.Subject, req.Domain, req.RoleName, req.RevokedBy,
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
	cmd, err := assignmentApp.NewRevokeByRoleNameCommand(sub, req.Domain, req.RoleName, revokeActor(req.RevokedBy, callerService), req.Reason)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	policyVersion, err := s.assignments.RevokeByRoleName(ctx, cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.RevokeAssignmentResponse{PolicyVersion: policyVersion}, nil
}

func (s *authorizationServer) ReplaceManagedAssignments(ctx context.Context, req *authzv3.ReplaceManagedAssignmentsRequest) (*authzv3.ReplaceManagedAssignmentsResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Domain) == "" || strings.TrimSpace(req.ChangedBy) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and changed_by are required")
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
		sub, req.Domain, req.RoleNames, managedRoles, req.ChangedBy, req.Reason,
	)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	result, err := s.assignments.ReplaceManagedAssignments(ctx, cmd)
	if err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.ReplaceManagedAssignmentsResponse{
		DirectRoles: result.DirectRoles, PolicyVersion: result.PolicyVersion, Changed: result.Changed,
	}, nil
}

func parseObjectContext(callerService, resourceKey string, input *authzv3.ObjectContext, admission objectattributeadmission.Policy) (authorizationdomain.ObjectContext, error) {
	if input == nil {
		return authorizationdomain.NewObjectContext("", nil)
	}
	if admission == nil {
		return authorizationdomain.ObjectContext{}, status.Error(codes.Internal, "object attribute admission policy is unavailable")
	}
	attributes := make(constraint.Attributes, len(input.Attributes))
	for _, item := range input.Attributes {
		if item == nil || strings.TrimSpace(item.Key) == "" || item.Value == nil {
			return authorizationdomain.ObjectContext{}, status.Error(codes.InvalidArgument, "each object attribute requires a key and typed value")
		}
		key := strings.TrimSpace(item.Key)
		if _, exists := attributes[key]; exists {
			return authorizationdomain.ObjectContext{}, status.Errorf(codes.InvalidArgument, "duplicate object attribute: %s", key)
		}
		if err := admission.AuthorizeAttribute(objectattributeadmission.Request{
			CallerService: callerService,
			ResourceKey:   resourceKey,
			AttributeKey:  key,
		}); err != nil {
			switch err.(type) {
			case objectattributeadmission.ErrUnsupportedAttribute:
				return authorizationdomain.ObjectContext{}, status.Errorf(codes.InvalidArgument, "%s", err.Error())
			case objectattributeadmission.ErrUntrustedCaller:
				return authorizationdomain.ObjectContext{}, status.Error(codes.PermissionDenied, err.Error())
			default:
				return authorizationdomain.ObjectContext{}, status.Error(codes.Internal, "object attribute authorization failed")
			}
		}
		switch value := item.Value.(type) {
		case *authzv3.ObjectAttribute_StringValue:
			attributes[key] = constraint.StringValue(value.StringValue)
		case *authzv3.ObjectAttribute_Int64Value:
			attributes[key] = constraint.Int64Value(value.Int64Value)
		case *authzv3.ObjectAttribute_BoolValue:
			attributes[key] = constraint.BoolValue(value.BoolValue)
		default:
			return authorizationdomain.ObjectContext{}, status.Error(codes.InvalidArgument, "unsupported object attribute value")
		}
	}
	object, err := authorizationdomain.NewObjectContext(input.ObjectId, attributes)
	if err != nil {
		return authorizationdomain.ObjectContext{}, iamgrpc.ToStatusError(err)
	}
	return object, nil
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
	subjectValue, domainValue, roleNameValue, delegatedActor string,
) (assignmentadmission.Request, error) {
	sub, err := subject.ParseRef(subjectValue)
	if err != nil {
		return assignmentadmission.Request{}, err
	}
	domain, err := tenant.NewID(domainValue)
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
		Domain:         domain,
		RoleName:       roleName,
		DelegatedActor: delegatedActor,
	}, nil
}

func replacementAdmissionRequest(req *authzv3.ReplaceManagedAssignmentsRequest) (assignmentadmission.ReplacementRequest, error) {
	sub, err := subject.ParseRef(req.Subject)
	if err != nil {
		return assignmentadmission.ReplacementRequest{}, err
	}
	domain, err := tenant.NewID(req.Domain)
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
		Domain:         domain,
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

func toProtoPermissions(entries []authzapp.PermissionEntry) []*authzv3.PermissionEntry {
	permissions := make([]*authzv3.PermissionEntry, 0, len(entries))
	for _, entry := range entries {
		mode := authzv3.AuthorizationMode_OBJECT_CHECK_REQUIRED
		if entry.Mode == authzapp.ModeUnconditional {
			mode = authzv3.AuthorizationMode_UNCONDITIONAL
		}
		permissions = append(permissions, &authzv3.PermissionEntry{Resource: entry.Resource, Action: entry.Action, Mode: mode})
	}
	return permissions
}

func toProtoReason(reason authorizationdomain.Reason) authzv3.DecisionReason {
	switch reason {
	case authorizationdomain.ReasonAllowed:
		return authzv3.DecisionReason_ALLOWED
	case authorizationdomain.ReasonAttributeMissing:
		return authzv3.DecisionReason_ATTRIBUTE_MISSING
	case authorizationdomain.ReasonNotMatched:
		return authzv3.DecisionReason_NOT_MATCHED
	default:
		return authzv3.DecisionReason_DECISION_REASON_UNSPECIFIED
	}
}

var _ authzv3.AuthorizationServiceServer = (*authorizationServer)(nil)
var _ authorizationChecker = (*authzapp.DecisionService)(nil)
