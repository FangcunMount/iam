package authz

import (
	"context"
	"errors"
	pb "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	app "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	iamgrpc "github.com/FangcunMount/iam/v5/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

func (s *authorizationServer) ReplaceScopedAssignments(ctx context.Context, req *pb.ReplaceScopedAssignmentsRequest) (*pb.ReplaceManagedAssignmentsResponse, error) {
	if _, err := requireServiceIdentity(ctx); err != nil {
		return nil, err
	}
	if s.assignments == nil {
		return nil, status.Error(codes.Unavailable, "assignment service is unavailable")
	}
	if req == nil || strings.TrimSpace(req.ChangedBy) == "" || req.ExpectedPolicyVersion <= 0 {
		return nil, status.Error(codes.InvalidArgument, "request, changed_by and positive expected_policy_version required")
	}
	org, err := meta.ParseID(req.OrgId)
	if err != nil || org <= 0 {
		return nil, status.Error(codes.InvalidArgument, "scope company required")
	}
	sub, err := parseSubjectKey(req.Subject)
	if err != nil {
		return nil, err
	}
	targets := make([]domain.ScopedRoleGrant, 0, len(req.Roles))
	names := make([]string, 0, len(req.Roles))
	seen := map[string]bool{}
	for _, target := range req.Roles {
		if target == nil || target.Scope == nil {
			return nil, status.Error(codes.InvalidArgument, "role scope required")
		}
		name, err := role.NewName(target.RoleName)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid role")
		}
		if seen[name.String()] {
			return nil, status.Error(codes.InvalidArgument, "duplicate scoped role")
		}
		seen[name.String()] = true
		company, err := meta.ParseID(target.Scope.OrgId)
		if err != nil || company != org {
			return nil, status.Error(codes.InvalidArgument, "scope belongs to different company")
		}
		kind := scope.Kind("")
		switch target.Scope.Kind {
		case pb.DataScopeKind_ALL_STORES:
			kind = scope.AllStores
		case pb.DataScopeKind_STORES:
			kind = scope.Stores
		}
		ids := make([]meta.ID, 0, len(target.Scope.StoreIds))
		for _, raw := range target.Scope.StoreIds {
			id, err := meta.ParseID(raw)
			if err != nil || id <= 0 {
				return nil, status.Error(codes.InvalidArgument, "invalid store ID")
			}
			ids = append(ids, id)
		}
		value, err := scope.New(org, kind, ids)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid scope")
		}
		targets = append(targets, domain.ScopedRoleGrant{RoleName: name, Scope: value})
		names = append(names, name.String())
	}
	admissionRequest, err := replacementAdmissionRequest(&pb.ReplaceManagedAssignmentsRequest{Subject: req.Subject, RoleNames: names, ChangedBy: req.ChangedBy})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	managed, err := admitAssignmentReplacement(ctx, s.assignmentAdmission, admissionRequest)
	if err != nil {
		return nil, err
	}
	result, err := s.assignments.ReplaceManagedAssignments(authenticatedManagementContext(ctx), app.ReplaceManagedAssignmentsCommand{ExpectedPolicyVersion: req.ExpectedPolicyVersion, Subject: sub, OrgID: org, ScopedRoles: targets, ManagedRoleNames: managed, ChangedBy: req.ChangedBy, Reason: req.Reason})
	if err != nil {
		if errors.Is(err, policyDomain.ErrStaleVersion) {
			return nil, status.Error(codes.Aborted, err.Error())
		}
		return nil, iamgrpc.ToStatusError(err)
	}
	return &pb.ReplaceManagedAssignmentsResponse{DirectRoles: result.DirectRoles, PolicyVersion: result.PolicyVersion, Changed: result.Changed}, nil
}
