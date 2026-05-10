package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/FangcunMount/iam/v2/internal/pkg/requestctx"
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
		requestctx.SetTenantID(c, tenantID)
		id, err := meta.ParseID(userID)
		if err != nil {
			panic(err)
		}
		requestctx.SetUserID(c, id)
	}
}

func withTenant(tenantID string) authzRequestOption {
	return func(c *gin.Context) {
		requestctx.SetTenantID(c, tenantID)
	}
}

func withUser(userID string) authzRequestOption {
	return func(c *gin.Context) {
		id, err := meta.ParseID(userID)
		if err != nil {
			panic(err)
		}
		requestctx.SetUserID(c, id)
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
	createFn func(context.Context, roleApp.CreateRoleCommand) (*roleDomain.Role, error)
	updateFn func(context.Context, roleApp.UpdateRoleCommand) (*roleDomain.Role, error)
	deleteFn func(context.Context, meta.ID) error

	createCalls []roleApp.CreateRoleCommand
	updateCalls []roleApp.UpdateRoleCommand
	deleteCalls []meta.ID
}

func (f *roleCommanderFake) CreateRole(ctx context.Context, cmd roleApp.CreateRoleCommand) (*roleDomain.Role, error) {
	f.createCalls = append(f.createCalls, cmd)
	if f.createFn != nil {
		return f.createFn(ctx, cmd)
	}
	result, _ := roleDomain.NewRole(cmd.Name, cmd.DisplayName, cmd.TenantID, roleDomain.WithID(meta.FromUint64(11)), roleDomain.WithDescription(cmd.Description))
	return &result, nil
}

func (f *roleCommanderFake) UpdateRole(ctx context.Context, cmd roleApp.UpdateRoleCommand) (*roleDomain.Role, error) {
	f.updateCalls = append(f.updateCalls, cmd)
	if f.updateFn != nil {
		return f.updateFn(ctx, cmd)
	}
	result, _ := roleDomain.NewRole("admin", valueOrEmpty(cmd.DisplayName), "tenant-a", roleDomain.WithID(cmd.ID), roleDomain.WithDescription(valueOrEmpty(cmd.Description)))
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
	listFn func(context.Context, roleApp.ListRolesQuery) (*roleApp.ListRolesResult, error)

	getCalls  []meta.ID
	listCalls []roleApp.ListRolesQuery
}

func (f *roleQueryerFake) GetRoleByID(ctx context.Context, roleID meta.ID) (*roleDomain.Role, error) {
	f.getCalls = append(f.getCalls, roleID)
	if f.getFn != nil {
		return f.getFn(ctx, roleID)
	}
	result, _ := roleDomain.NewRole("admin", "Admin", "tenant-a", roleDomain.WithID(roleID))
	return &result, nil
}

func (f *roleQueryerFake) GetRoleByName(context.Context, string, string) (*roleDomain.Role, error) {
	return nil, nil
}

func (f *roleQueryerFake) ListRoles(ctx context.Context, query roleApp.ListRolesQuery) (*roleApp.ListRolesResult, error) {
	f.listCalls = append(f.listCalls, query)
	if f.listFn != nil {
		return f.listFn(ctx, query)
	}
	result, _ := roleDomain.NewRole("admin", "Admin", query.TenantID, roleDomain.WithID(meta.FromUint64(11)))
	return &roleApp.ListRolesResult{Roles: []*roleDomain.Role{&result}, Total: 1}, nil
}

func (f *roleQueryerFake) ListRolesByTenant(context.Context, string) ([]*roleDomain.Role, error) {
	return nil, nil
}

type resourceCommanderFake struct {
	createFn func(context.Context, resourceApp.CreateResourceCommand) (*resourceDomain.Resource, error)
	updateFn func(context.Context, resourceApp.UpdateResourceCommand) (*resourceDomain.Resource, error)
	deleteFn func(context.Context, resourceDomain.ResourceID) error

	createCalls []resourceApp.CreateResourceCommand
	updateCalls []resourceApp.UpdateResourceCommand
	deleteCalls []resourceDomain.ResourceID
}

func (f *resourceCommanderFake) CreateResource(ctx context.Context, cmd resourceApp.CreateResourceCommand) (*resourceDomain.Resource, error) {
	f.createCalls = append(f.createCalls, cmd)
	if f.createFn != nil {
		return f.createFn(ctx, cmd)
	}
	result, _ := resourceDomain.NewResource(cmd.Key, cmd.Actions,
		resourceDomain.WithID(resourceDomain.NewResourceID(21)),
		resourceDomain.WithDisplayName(cmd.DisplayName),
		resourceDomain.WithAppName(cmd.AppName),
		resourceDomain.WithDomain(cmd.Domain),
		resourceDomain.WithType(cmd.Type),
		resourceDomain.WithDescription(cmd.Description),
	)
	return &result, nil
}

func (f *resourceCommanderFake) UpdateResource(ctx context.Context, cmd resourceApp.UpdateResourceCommand) (*resourceDomain.Resource, error) {
	f.updateCalls = append(f.updateCalls, cmd)
	if f.updateFn != nil {
		return f.updateFn(ctx, cmd)
	}
	result, _ := resourceDomain.NewResource("scale:form:template:*", cmd.Actions,
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
	listFn     func(context.Context, resourceApp.ListResourcesQuery) (*resourceApp.ListResourcesResult, error)
	validateFn func(context.Context, string, string) (bool, error)

	getByIDCalls  []resourceDomain.ResourceID
	getByKeyCalls []string
	listCalls     []resourceApp.ListResourcesQuery
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
	result, _ := resourceDomain.NewResource("scale:form:template:*", []string{"read"}, resourceDomain.WithID(resourceID))
	return &result, nil
}

func (f *resourceQueryerFake) GetResourceByKey(ctx context.Context, key string) (*resourceDomain.Resource, error) {
	f.getByKeyCalls = append(f.getByKeyCalls, key)
	if f.getByKeyFn != nil {
		return f.getByKeyFn(ctx, key)
	}
	result, _ := resourceDomain.NewResource(key, []string{"read"}, resourceDomain.WithID(resourceDomain.NewResourceID(21)))
	return &result, nil
}

func (f *resourceQueryerFake) ListResources(ctx context.Context, query resourceApp.ListResourcesQuery) (*resourceApp.ListResourcesResult, error) {
	f.listCalls = append(f.listCalls, query)
	if f.listFn != nil {
		return f.listFn(ctx, query)
	}
	result, _ := resourceDomain.NewResource("scale:form:template:*", []string{"read"}, resourceDomain.WithID(resourceDomain.NewResourceID(21)))
	return &resourceApp.ListResourcesResult{Resources: []*resourceDomain.Resource{&result}, Total: 1}, nil
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

type bindingCommanderFake struct {
	grantFn      func(context.Context, bindingApp.GrantCommand) (*bindingDomain.Binding, error)
	revokeFn     func(context.Context, bindingApp.RevokeCommand) error
	revokeByIDFn func(context.Context, bindingApp.RevokeByIDCommand) error

	grantCalls      []bindingApp.GrantCommand
	revokeCalls     []bindingApp.RevokeCommand
	revokeByIDCalls []bindingApp.RevokeByIDCommand
}

func (f *bindingCommanderFake) Grant(ctx context.Context, cmd bindingApp.GrantCommand) (*bindingDomain.Binding, error) {
	f.grantCalls = append(f.grantCalls, cmd)
	if f.grantFn != nil {
		return f.grantFn(ctx, cmd)
	}
	result := bindingDomain.NewBinding(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID,
		bindingDomain.WithID(bindingDomain.NewBindingID(31)),
		bindingDomain.WithGrantedBy(cmd.GrantedBy),
	)
	return &result, nil
}

func (f *bindingCommanderFake) Revoke(ctx context.Context, cmd bindingApp.RevokeCommand) error {
	f.revokeCalls = append(f.revokeCalls, cmd)
	if f.revokeFn != nil {
		return f.revokeFn(ctx, cmd)
	}
	return nil
}

func (f *bindingCommanderFake) RevokeByID(ctx context.Context, cmd bindingApp.RevokeByIDCommand) error {
	f.revokeByIDCalls = append(f.revokeByIDCalls, cmd)
	if f.revokeByIDFn != nil {
		return f.revokeByIDFn(ctx, cmd)
	}
	return nil
}

type bindingQueryerFake struct {
	listBySubjectFn func(context.Context, bindingApp.ListBySubjectQuery) ([]*bindingDomain.Binding, error)
	listByRoleFn    func(context.Context, bindingApp.ListByRoleQuery) ([]*bindingDomain.Binding, error)

	listBySubjectCalls []bindingApp.ListBySubjectQuery
	listByRoleCalls    []bindingApp.ListByRoleQuery
}

func (f *bindingQueryerFake) ListBySubject(ctx context.Context, query bindingApp.ListBySubjectQuery) ([]*bindingDomain.Binding, error) {
	f.listBySubjectCalls = append(f.listBySubjectCalls, query)
	if f.listBySubjectFn != nil {
		return f.listBySubjectFn(ctx, query)
	}
	result := bindingDomain.NewBinding(query.SubjectType, query.SubjectID, meta.FromUint64(11), query.TenantID, bindingDomain.WithID(bindingDomain.NewBindingID(31)))
	return []*bindingDomain.Binding{&result}, nil
}

func (f *bindingQueryerFake) ListByRole(ctx context.Context, query bindingApp.ListByRoleQuery) ([]*bindingDomain.Binding, error) {
	f.listByRoleCalls = append(f.listByRoleCalls, query)
	if f.listByRoleFn != nil {
		return f.listByRoleFn(ctx, query)
	}
	result := bindingDomain.NewBinding(bindingDomain.SubjectTypeUser, meta.FromUint64(1), query.RoleID, query.TenantID, bindingDomain.WithID(bindingDomain.NewBindingID(31)))
	return []*bindingDomain.Binding{&result}, nil
}

type policyCommanderFake struct {
	addFn    func(context.Context, policyApp.AddPermissionCommand) error
	removeFn func(context.Context, policyApp.RemovePermissionCommand) error

	addCalls    []policyApp.AddPermissionCommand
	removeCalls []policyApp.RemovePermissionCommand
}

func (f *policyCommanderFake) AddPermission(ctx context.Context, cmd policyApp.AddPermissionCommand) error {
	f.addCalls = append(f.addCalls, cmd)
	if f.addFn != nil {
		return f.addFn(ctx, cmd)
	}
	return nil
}

func (f *policyCommanderFake) RemovePermission(ctx context.Context, cmd policyApp.RemovePermissionCommand) error {
	f.removeCalls = append(f.removeCalls, cmd)
	if f.removeFn != nil {
		return f.removeFn(ctx, cmd)
	}
	return nil
}

type policyQueryerFake struct {
	getPoliciesFn       func(context.Context, policyApp.RolePermissionsQuery) ([]authzDomain.Permission, error)
	getCurrentVersionFn func(context.Context, policyApp.CurrentVersionQuery) (*policyDomain.PolicyVersion, error)

	getPoliciesCalls       []policyApp.RolePermissionsQuery
	getCurrentVersionCalls []policyApp.CurrentVersionQuery
}

func (f *policyQueryerFake) GetPermissionsForRole(ctx context.Context, query policyApp.RolePermissionsQuery) ([]authzDomain.Permission, error) {
	f.getPoliciesCalls = append(f.getPoliciesCalls, query)
	if f.getPoliciesFn != nil {
		return f.getPoliciesFn(ctx, query)
	}
	permission, err := authzDomain.NewPermission("admin", query.TenantID, "scale:form:template:*", "read")
	if err != nil {
		return nil, err
	}
	return []authzDomain.Permission{permission}, nil
}

func (f *policyQueryerFake) GetCurrentVersion(ctx context.Context, query policyApp.CurrentVersionQuery) (*policyDomain.PolicyVersion, error) {
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
		sub   string
		dom   string
		obj   string
		act   string
		scope authzDomain.Scope
	}
}

func (f *casbinFake) AuthorizeRoute(ctx context.Context, sub, dom, obj, act string) (bool, error) {
	f.enforceCalls = append(f.enforceCalls, struct {
		sub   string
		dom   string
		obj   string
		act   string
		scope authzDomain.Scope
	}{sub: sub, dom: dom, obj: obj, act: act})
	if f.enforceFn != nil {
		return f.enforceFn(ctx, sub, dom, obj, act)
	}
	return true, nil
}

func (f *casbinFake) Check(ctx context.Context, cmd authzapp.CheckCommand) (authzDomain.AuthorizationDecision, error) {
	sub := string(cmd.Subject.Type) + ":" + cmd.Subject.ID.String()
	allowed, err := f.AuthorizeRoute(ctx, sub, cmd.TenantID, cmd.ResourceKey, cmd.Action)
	if err != nil {
		return authzDomain.AuthorizationDecision{}, err
	}
	if len(f.enforceCalls) > 0 {
		f.enforceCalls[len(f.enforceCalls)-1].scope = cmd.ObjectScope
	}
	return authzDomain.AuthorizationDecision{Allowed: allowed}, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
