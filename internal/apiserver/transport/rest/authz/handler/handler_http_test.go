package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type authzEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
	Total   int64           `json:"total,omitempty"`
	Offset  int             `json:"offset,omitempty"`
	Limit   int             `json:"limit,omitempty"`
}

type authzRequestOption func(*gin.Context)

func performAuthzRequest(method, target, body string, handler gin.HandlerFunc, opts ...authzRequestOption) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	for _, opt := range opts {
		opt(c)
	}

	handler(c)
	return recorder, c
}

func withTenantUser(tenantID, userID string) authzRequestOption {
	return func(c *gin.Context) {
		c.Set("tenant_id", tenantID)
		c.Set("user_id", userID)
	}
}

func withTenant(tenantID string) authzRequestOption {
	return func(c *gin.Context) {
		c.Set("tenant_id", tenantID)
	}
}

func withUser(userID string) authzRequestOption {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
	}
}

func withPathParam(key, value string) authzRequestOption {
	return func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: key, Value: value})
	}
}

func requireAuthzCode(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus, wantCode int) authzEnvelope {
	t.Helper()

	require.Equal(t, wantStatus, recorder.Code, "body=%s", recorder.Body.String())

	var envelope authzEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), "body=%s", recorder.Body.String())
	require.Equal(t, wantCode, envelope.Code, "body=%s", recorder.Body.String())
	return envelope
}

type roleCommanderFake struct {
	createFn func(context.Context, roleDomain.CreateRoleCommand) (*roleDomain.Role, error)
	updateFn func(context.Context, roleDomain.UpdateRoleCommand) (*roleDomain.Role, error)
	deleteFn func(context.Context, meta.ID) error

	createCalls []roleDomain.CreateRoleCommand
	updateCalls []roleDomain.UpdateRoleCommand
	deleteCalls []meta.ID
}

func (f *roleCommanderFake) CreateRole(ctx context.Context, cmd roleDomain.CreateRoleCommand) (*roleDomain.Role, error) {
	f.createCalls = append(f.createCalls, cmd)
	if f.createFn != nil {
		return f.createFn(ctx, cmd)
	}
	result := roleDomain.NewRole(cmd.Name, cmd.DisplayName, cmd.TenantID, roleDomain.WithID(meta.FromUint64(11)), roleDomain.WithDescription(cmd.Description))
	return &result, nil
}

func (f *roleCommanderFake) UpdateRole(ctx context.Context, cmd roleDomain.UpdateRoleCommand) (*roleDomain.Role, error) {
	f.updateCalls = append(f.updateCalls, cmd)
	if f.updateFn != nil {
		return f.updateFn(ctx, cmd)
	}
	result := roleDomain.NewRole("admin", valueOrEmpty(cmd.DisplayName), "tenant-a", roleDomain.WithID(cmd.ID), roleDomain.WithDescription(valueOrEmpty(cmd.Description)))
	return &result, nil
}

func (f *roleCommanderFake) DeleteRole(ctx context.Context, roleID meta.ID) error {
	f.deleteCalls = append(f.deleteCalls, roleID)
	if f.deleteFn != nil {
		return f.deleteFn(ctx, roleID)
	}
	return nil
}

type roleQueryerFake struct {
	getFn  func(context.Context, meta.ID) (*roleDomain.Role, error)
	listFn func(context.Context, roleDomain.ListRolesQuery) (*roleDomain.ListRolesResult, error)

	getCalls  []meta.ID
	listCalls []roleDomain.ListRolesQuery
}

func (f *roleQueryerFake) GetRoleByID(ctx context.Context, roleID meta.ID) (*roleDomain.Role, error) {
	f.getCalls = append(f.getCalls, roleID)
	if f.getFn != nil {
		return f.getFn(ctx, roleID)
	}
	result := roleDomain.NewRole("admin", "Admin", "tenant-a", roleDomain.WithID(roleID))
	return &result, nil
}

func (f *roleQueryerFake) GetRoleByName(context.Context, string, string) (*roleDomain.Role, error) {
	return nil, nil
}

func (f *roleQueryerFake) ListRoles(ctx context.Context, query roleDomain.ListRolesQuery) (*roleDomain.ListRolesResult, error) {
	f.listCalls = append(f.listCalls, query)
	if f.listFn != nil {
		return f.listFn(ctx, query)
	}
	result := roleDomain.NewRole("admin", "Admin", query.TenantID, roleDomain.WithID(meta.FromUint64(11)))
	return &roleDomain.ListRolesResult{Roles: []*roleDomain.Role{&result}, Total: 1}, nil
}

func (f *roleQueryerFake) ListRolesByTenant(context.Context, string) ([]*roleDomain.Role, error) {
	return nil, nil
}

type resourceCommanderFake struct {
	createFn func(context.Context, resourceDomain.CreateResourceCommand) (*resourceDomain.Resource, error)
	updateFn func(context.Context, resourceDomain.UpdateResourceCommand) (*resourceDomain.Resource, error)
	deleteFn func(context.Context, resourceDomain.ResourceID) error

	createCalls []resourceDomain.CreateResourceCommand
	updateCalls []resourceDomain.UpdateResourceCommand
	deleteCalls []resourceDomain.ResourceID
}

func (f *resourceCommanderFake) CreateResource(ctx context.Context, cmd resourceDomain.CreateResourceCommand) (*resourceDomain.Resource, error) {
	f.createCalls = append(f.createCalls, cmd)
	if f.createFn != nil {
		return f.createFn(ctx, cmd)
	}
	result := resourceDomain.NewResource(cmd.Key, cmd.Actions,
		resourceDomain.WithID(resourceDomain.NewResourceID(21)),
		resourceDomain.WithDisplayName(cmd.DisplayName),
		resourceDomain.WithAppName(cmd.AppName),
		resourceDomain.WithDomain(cmd.Domain),
		resourceDomain.WithType(cmd.Type),
		resourceDomain.WithDescription(cmd.Description),
	)
	return &result, nil
}

func (f *resourceCommanderFake) UpdateResource(ctx context.Context, cmd resourceDomain.UpdateResourceCommand) (*resourceDomain.Resource, error) {
	f.updateCalls = append(f.updateCalls, cmd)
	if f.updateFn != nil {
		return f.updateFn(ctx, cmd)
	}
	result := resourceDomain.NewResource("scale:form:*", cmd.Actions,
		resourceDomain.WithID(cmd.ID),
		resourceDomain.WithDisplayName(valueOrEmpty(cmd.DisplayName)),
		resourceDomain.WithDescription(valueOrEmpty(cmd.Description)),
	)
	return &result, nil
}

func (f *resourceCommanderFake) DeleteResource(ctx context.Context, resourceID resourceDomain.ResourceID) error {
	f.deleteCalls = append(f.deleteCalls, resourceID)
	if f.deleteFn != nil {
		return f.deleteFn(ctx, resourceID)
	}
	return nil
}

type resourceQueryerFake struct {
	getByIDFn  func(context.Context, resourceDomain.ResourceID) (*resourceDomain.Resource, error)
	getByKeyFn func(context.Context, string) (*resourceDomain.Resource, error)
	listFn     func(context.Context, resourceDomain.ListResourcesQuery) (*resourceDomain.ListResourcesResult, error)
	validateFn func(context.Context, string, string) (bool, error)

	getByIDCalls  []resourceDomain.ResourceID
	getByKeyCalls []string
	listCalls     []resourceDomain.ListResourcesQuery
	validateCalls []struct {
		resourceKey string
		action      string
	}
}

func (f *resourceQueryerFake) GetResourceByID(ctx context.Context, resourceID resourceDomain.ResourceID) (*resourceDomain.Resource, error) {
	f.getByIDCalls = append(f.getByIDCalls, resourceID)
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, resourceID)
	}
	result := resourceDomain.NewResource("scale:form:*", []string{"read"}, resourceDomain.WithID(resourceID))
	return &result, nil
}

func (f *resourceQueryerFake) GetResourceByKey(ctx context.Context, key string) (*resourceDomain.Resource, error) {
	f.getByKeyCalls = append(f.getByKeyCalls, key)
	if f.getByKeyFn != nil {
		return f.getByKeyFn(ctx, key)
	}
	result := resourceDomain.NewResource(key, []string{"read"}, resourceDomain.WithID(resourceDomain.NewResourceID(21)))
	return &result, nil
}

func (f *resourceQueryerFake) ListResources(ctx context.Context, query resourceDomain.ListResourcesQuery) (*resourceDomain.ListResourcesResult, error) {
	f.listCalls = append(f.listCalls, query)
	if f.listFn != nil {
		return f.listFn(ctx, query)
	}
	result := resourceDomain.NewResource("scale:form:*", []string{"read"}, resourceDomain.WithID(resourceDomain.NewResourceID(21)))
	return &resourceDomain.ListResourcesResult{Resources: []*resourceDomain.Resource{&result}, Total: 1}, nil
}

func (f *resourceQueryerFake) ValidateAction(ctx context.Context, resourceKey, action string) (bool, error) {
	f.validateCalls = append(f.validateCalls, struct {
		resourceKey string
		action      string
	}{resourceKey: resourceKey, action: action})
	if f.validateFn != nil {
		return f.validateFn(ctx, resourceKey, action)
	}
	return true, nil
}

type assignmentCommanderFake struct {
	grantFn      func(context.Context, assignmentDomain.GrantCommand) (*assignmentDomain.Assignment, error)
	revokeFn     func(context.Context, assignmentDomain.RevokeCommand) error
	revokeByIDFn func(context.Context, assignmentDomain.RevokeByIDCommand) error

	grantCalls      []assignmentDomain.GrantCommand
	revokeCalls     []assignmentDomain.RevokeCommand
	revokeByIDCalls []assignmentDomain.RevokeByIDCommand
}

func (f *assignmentCommanderFake) Grant(ctx context.Context, cmd assignmentDomain.GrantCommand) (*assignmentDomain.Assignment, error) {
	f.grantCalls = append(f.grantCalls, cmd)
	if f.grantFn != nil {
		return f.grantFn(ctx, cmd)
	}
	result := assignmentDomain.NewAssignment(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID,
		assignmentDomain.WithID(assignmentDomain.NewAssignmentID(31)),
		assignmentDomain.WithGrantedBy(cmd.GrantedBy),
	)
	return &result, nil
}

func (f *assignmentCommanderFake) Revoke(ctx context.Context, cmd assignmentDomain.RevokeCommand) error {
	f.revokeCalls = append(f.revokeCalls, cmd)
	if f.revokeFn != nil {
		return f.revokeFn(ctx, cmd)
	}
	return nil
}

func (f *assignmentCommanderFake) RevokeByID(ctx context.Context, cmd assignmentDomain.RevokeByIDCommand) error {
	f.revokeByIDCalls = append(f.revokeByIDCalls, cmd)
	if f.revokeByIDFn != nil {
		return f.revokeByIDFn(ctx, cmd)
	}
	return nil
}

type assignmentQueryerFake struct {
	listBySubjectFn func(context.Context, assignmentDomain.ListBySubjectQuery) ([]*assignmentDomain.Assignment, error)
	listByRoleFn    func(context.Context, assignmentDomain.ListByRoleQuery) ([]*assignmentDomain.Assignment, error)

	listBySubjectCalls []assignmentDomain.ListBySubjectQuery
	listByRoleCalls    []assignmentDomain.ListByRoleQuery
}

func (f *assignmentQueryerFake) ListBySubject(ctx context.Context, query assignmentDomain.ListBySubjectQuery) ([]*assignmentDomain.Assignment, error) {
	f.listBySubjectCalls = append(f.listBySubjectCalls, query)
	if f.listBySubjectFn != nil {
		return f.listBySubjectFn(ctx, query)
	}
	result := assignmentDomain.NewAssignment(query.SubjectType, query.SubjectID, 11, query.TenantID, assignmentDomain.WithID(assignmentDomain.NewAssignmentID(31)))
	return []*assignmentDomain.Assignment{&result}, nil
}

func (f *assignmentQueryerFake) ListByRole(ctx context.Context, query assignmentDomain.ListByRoleQuery) ([]*assignmentDomain.Assignment, error) {
	f.listByRoleCalls = append(f.listByRoleCalls, query)
	if f.listByRoleFn != nil {
		return f.listByRoleFn(ctx, query)
	}
	result := assignmentDomain.NewAssignment(assignmentDomain.SubjectTypeUser, "user-1", query.RoleID, query.TenantID, assignmentDomain.WithID(assignmentDomain.NewAssignmentID(31)))
	return []*assignmentDomain.Assignment{&result}, nil
}

type policyCommanderFake struct {
	addFn    func(context.Context, policyDomain.AddPolicyRuleCommand) error
	removeFn func(context.Context, policyDomain.RemovePolicyRuleCommand) error

	addCalls    []policyDomain.AddPolicyRuleCommand
	removeCalls []policyDomain.RemovePolicyRuleCommand
}

func (f *policyCommanderFake) AddPolicyRule(ctx context.Context, cmd policyDomain.AddPolicyRuleCommand) error {
	f.addCalls = append(f.addCalls, cmd)
	if f.addFn != nil {
		return f.addFn(ctx, cmd)
	}
	return nil
}

func (f *policyCommanderFake) RemovePolicyRule(ctx context.Context, cmd policyDomain.RemovePolicyRuleCommand) error {
	f.removeCalls = append(f.removeCalls, cmd)
	if f.removeFn != nil {
		return f.removeFn(ctx, cmd)
	}
	return nil
}

type policyQueryerFake struct {
	getPoliciesFn       func(context.Context, policyDomain.GetPoliciesByRoleQuery) ([]policyDomain.PolicyRule, error)
	getCurrentVersionFn func(context.Context, policyDomain.GetCurrentVersionQuery) (*policyDomain.PolicyVersion, error)

	getPoliciesCalls       []policyDomain.GetPoliciesByRoleQuery
	getCurrentVersionCalls []policyDomain.GetCurrentVersionQuery
}

func (f *policyQueryerFake) GetPoliciesByRole(ctx context.Context, query policyDomain.GetPoliciesByRoleQuery) ([]policyDomain.PolicyRule, error) {
	f.getPoliciesCalls = append(f.getPoliciesCalls, query)
	if f.getPoliciesFn != nil {
		return f.getPoliciesFn(ctx, query)
	}
	return []policyDomain.PolicyRule{policyDomain.NewPolicyRule("role:admin", query.TenantID, "scale:form:*", "read")}, nil
}

func (f *policyQueryerFake) GetCurrentVersion(ctx context.Context, query policyDomain.GetCurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
	f.getCurrentVersionCalls = append(f.getCurrentVersionCalls, query)
	if f.getCurrentVersionFn != nil {
		return f.getCurrentVersionFn(ctx, query)
	}
	result := policyDomain.NewPolicyVersion(query.TenantID, 3, policyDomain.WithChangedBy("operator"), policyDomain.WithReason("test"))
	return &result, nil
}

type casbinFake struct {
	enforceFn func(context.Context, string, string, string, string) (bool, error)

	enforceCalls []struct {
		sub string
		dom string
		obj string
		act string
	}
}

func (f *casbinFake) AddPolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}

func (f *casbinFake) RemovePolicy(context.Context, ...policyDomain.PolicyRule) error {
	return nil
}

func (f *casbinFake) AddGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}

func (f *casbinFake) RemoveGroupingPolicy(context.Context, ...policyDomain.GroupingRule) error {
	return nil
}

func (f *casbinFake) GetPoliciesByRole(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}

func (f *casbinFake) GetGroupingsBySubject(context.Context, string, string) ([]policyDomain.GroupingRule, error) {
	return nil, nil
}

func (f *casbinFake) LoadPolicy(context.Context) error {
	return nil
}

func (f *casbinFake) Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error) {
	f.enforceCalls = append(f.enforceCalls, struct {
		sub string
		dom string
		obj string
		act string
	}{sub: sub, dom: dom, obj: obj, act: act})
	if f.enforceFn != nil {
		return f.enforceFn(ctx, sub, dom, obj, act)
	}
	return true, nil
}

func (f *casbinFake) GetRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *casbinFake) GetImplicitRolesForUser(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *casbinFake) GetImplicitPermissionsForUser(context.Context, string, string) ([]policyDomain.PolicyRule, error) {
	return nil, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
