package authz

import (
	"context"
	"errors"
	"strings"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	assignmentauth "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentauth"
	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	rolebindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	iamgrpc "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	trustedAssessmentAttributeService = "qs-apiserver.svc"
	assessmentResource                = "qs:evaluation:collection:assessments"
)

type authorizationChecker interface {
	Check(context.Context, authzruntime.Request) (authzruntime.Decision, error)
}

type authorizationSnapshotReader interface {
	Read(context.Context, subject.Ref, string, string) (authzruntime.SubjectSnapshot, error)
}

type Service struct{ srv authorizationServer }

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
	return &Service{srv: authorizationServer{
		checker: checker, snapshotReader: snapshotReader,
		roleBindings: roleBindings, assignmentAuthorizer: assignmentAuthorizer,
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
	checker              authorizationChecker
	snapshotReader       authorizationSnapshotReader
	roleBindings         rolebindingApp.NamedCommands
	assignmentAuthorizer assignmentauth.Authorizer
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
	object, err := parseObjectContext(callerService, req.Resource, req.ObjectContext)
	if err != nil {
		return nil, err
	}
	request, err := authzruntime.NewRequest(sub, req.Domain, req.Resource, req.Action, object)
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
		Roles: snapshot.Roles, Permissions: toProtoPermissions(snapshot.Permissions),
		PolicyVersion: snapshot.PolicyVersion,
	}, nil
}

func (s *authorizationServer) GrantAssignment(ctx context.Context, req *authzv3.GrantAssignmentRequest) (*authzv3.GrantAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.roleBindings == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and role_name are required")
	}
	if _, err := authorizeAssignmentRequest(ctx, s.assignmentAuthorizer, assignmentauth.Request{
		Operation: assignmentauth.OperationGrant, Subject: req.Subject, Domain: req.Domain,
		RoleName: req.RoleName, DelegatedActor: req.GrantedBy,
	}); err != nil {
		return nil, err
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := rolebindingApp.NewGrantByRoleNameCommand(sub, req.Domain, req.RoleName, req.GrantedBy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.GrantByRoleName(ctx, cmd); err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.GrantAssignmentResponse{}, nil
}

func (s *authorizationServer) RevokeAssignment(ctx context.Context, req *authzv3.RevokeAssignmentRequest) (*authzv3.RevokeAssignmentResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.roleBindings == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || req.Subject == "" || req.Domain == "" || req.RoleName == "" {
		return nil, status.Error(codes.InvalidArgument, "subject, domain, and role_name are required")
	}
	callerService, err := authorizeAssignmentRequest(ctx, s.assignmentAuthorizer, assignmentauth.Request{
		Operation: assignmentauth.OperationRevoke, Subject: req.Subject, Domain: req.Domain,
		RoleName: req.RoleName, DelegatedActor: req.RevokedBy,
	})
	if err != nil {
		return nil, err
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	cmd, err := rolebindingApp.NewRevokeByRoleNameCommand(sub, req.Domain, req.RoleName, revokeActor(req.RevokedBy, callerService), req.Reason)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.roleBindings.RevokeByRoleName(ctx, cmd); err != nil {
		return nil, iamgrpc.ToStatusError(err)
	}
	return &authzv3.RevokeAssignmentResponse{}, nil
}

func parseObjectContext(callerService, resourceKey string, input *authzv3.ObjectContext) (authzruntime.ObjectContext, error) {
	if input == nil {
		return authzruntime.NewObjectContext("", nil)
	}
	attributes := make(constraint.Attributes, len(input.Attributes))
	for _, item := range input.Attributes {
		if item == nil || strings.TrimSpace(item.Key) == "" || item.Value == nil {
			return authzruntime.ObjectContext{}, status.Error(codes.InvalidArgument, "each object attribute requires a key and typed value")
		}
		key := strings.TrimSpace(item.Key)
		if _, exists := attributes[key]; exists {
			return authzruntime.ObjectContext{}, status.Errorf(codes.InvalidArgument, "duplicate object attribute: %s", key)
		}
		if key != attribute.ObjectOriginType || resourceKey != assessmentResource {
			return authzruntime.ObjectContext{}, status.Errorf(codes.InvalidArgument, "unsupported object attribute: %s", key)
		}
		if callerService != trustedAssessmentAttributeService {
			return authzruntime.ObjectContext{}, status.Error(codes.PermissionDenied, "caller is not trusted to provide this object attribute")
		}
		switch value := item.Value.(type) {
		case *authzv3.ObjectAttribute_StringValue:
			attributes[key] = constraint.StringValue(value.StringValue)
		case *authzv3.ObjectAttribute_Int64Value:
			attributes[key] = constraint.Int64Value(value.Int64Value)
		case *authzv3.ObjectAttribute_BoolValue:
			attributes[key] = constraint.BoolValue(value.BoolValue)
		default:
			return authzruntime.ObjectContext{}, status.Error(codes.InvalidArgument, "unsupported object attribute value")
		}
	}
	object, err := authzruntime.NewObjectContext(input.ObjectId, attributes)
	if err != nil {
		return authzruntime.ObjectContext{}, iamgrpc.ToStatusError(err)
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

func authorizeAssignmentRequest(ctx context.Context, authorizer assignmentauth.Authorizer, request assignmentauth.Request) (string, error) {
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || strings.TrimSpace(identity.ServiceName) == "" {
		recordAssignmentAuthorization("unknown", string(request.Operation), "denied")
		return "", status.Error(codes.PermissionDenied, "assignment caller identity is required")
	}
	request.CallerService = strings.TrimSpace(identity.ServiceName)
	if authorizer == nil {
		recordAssignmentAuthorization(request.CallerService, string(request.Operation), "allowed")
		return request.CallerService, nil
	}
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

func toProtoPermissions(entries []authzruntime.PermissionEntry) []*authzv3.PermissionEntry {
	permissions := make([]*authzv3.PermissionEntry, 0, len(entries))
	for _, entry := range entries {
		mode := authzv3.AuthorizationMode_OBJECT_CHECK_REQUIRED
		if entry.Mode == authzruntime.ModeUnconditional {
			mode = authzv3.AuthorizationMode_UNCONDITIONAL
		}
		permissions = append(permissions, &authzv3.PermissionEntry{Resource: entry.Resource, Action: entry.Action, Mode: mode})
	}
	return permissions
}

func toProtoReason(reason authzruntime.Reason) authzv3.DecisionReason {
	switch reason {
	case authzruntime.ReasonAllowed:
		return authzv3.DecisionReason_ALLOWED
	case authzruntime.ReasonAttributeMissing:
		return authzv3.DecisionReason_ATTRIBUTE_MISSING
	case authzruntime.ReasonNotMatched:
		return authzv3.DecisionReason_NOT_MATCHED
	default:
		return authzv3.DecisionReason_DECISION_REASON_UNSPECIFIED
	}
}

var _ authzv3.AuthorizationServiceServer = (*authorizationServer)(nil)
var _ authorizationChecker = (*authzapp.NativeChecker)(nil)
