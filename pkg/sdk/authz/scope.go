package authz

import (
	"context"
	"fmt"
	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	"strconv"
)

// GetScopedAuthorizationSnapshot refuses older producers and malformed scopes.
// Business callers must still match the action and enforce its returned ranges.
func (c *Client) GetScopedAuthorizationSnapshot(ctx context.Context, request *authzv4.GetAuthorizationSnapshotRequest) (*authzv4.GetAuthorizationSnapshotResponse, error) {
	response, err := c.GetAuthorizationSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}
	if err = ValidateScopedSnapshot(response); err != nil {
		return nil, err
	}
	if request != nil && request.IncludeAssignmentFacts {
		if err := ValidateAssignmentScopes(response); err != nil {
			return nil, err
		}
	}
	return response, nil
}

// ValidateScopedSnapshot never treats an absent scope as company-wide access.
// A valid snapshot can contain zero ranges; those entries grant no data range.
func ValidateScopedSnapshot(response *authzv4.GetAuthorizationSnapshotResponse) error {
	if response == nil || response.ScopeContractVersion != 1 || response.PolicyVersion <= 0 {
		return fmt.Errorf("scope-aware authorization snapshot unavailable")
	}

	for _, entry := range response.Permissions {
		if entry == nil || entry.Mode != authzv4.AuthorizationMode_UNCONDITIONAL || entry.Resource == "" || entry.Action == "" {
			return fmt.Errorf("invalid scoped permission")
		}
		for _, value := range entry.Scopes {
			if err := validateDataScope(value); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateAssignmentScopes refuses incomplete management facts, including an
// older producer that supplies role names but omits per-assignment ranges.
// Nil Scope remains a valid, explicitly unconfigured assignment.
func ValidateAssignmentScopes(response *authzv4.GetAuthorizationSnapshotResponse) error {
	if err := ValidateScopedSnapshot(response); err != nil {
		return err
	}
	if !response.AssignmentFactsComplete {
		return fmt.Errorf("complete assignment facts unavailable")
	}
	roles := map[string]*authzv4.AssignmentRoleFact{}
	validID := func(value string) bool {
		id, err := strconv.ParseInt(value, 10, 64)
		return err == nil && id > 0 && strconv.FormatInt(id, 10) == value
	}
	for _, role := range response.AssignmentFacts {
		if role == nil || !validID(role.RoleId) || role.RoleName == "" {
			return fmt.Errorf("invalid assignment role fact")
		}
		if _, ok := roles[role.RoleId]; ok {
			return fmt.Errorf("duplicate assignment role fact")
		}
		roles[role.RoleId] = role
	}
	ids := map[string]bool{}
	covered := map[string]bool{}
	for _, fact := range response.AssignmentScopes {
		if fact == nil || !validID(fact.AssignmentId) || ids[fact.AssignmentId] || fact.Role == nil {
			return fmt.Errorf("invalid or duplicate assignment fact")
		}
		role, ok := roles[fact.Role.RoleId]
		if !ok || role.RoleName != fact.Role.RoleName || role.ManagementProtection != fact.Role.ManagementProtection {
			return fmt.Errorf("assignment role facts disagree")
		}
		ids[fact.AssignmentId] = true
		covered[role.RoleId] = true
		if fact.Scope != nil {
			if err := validateDataScope(fact.Scope); err != nil {
				return err
			}
		}
	}
	if len(covered) != len(roles) {
		return fmt.Errorf("per-assignment scope facts incomplete")
	}
	return nil
}

func positiveScopeID(value string) bool {
	id, err := strconv.ParseInt(value, 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == value
}
func validateDataScope(value *authzv4.DataScope) error {
	if value == nil || !positiveScopeID(value.OrgId) {
		return fmt.Errorf("invalid scope company")
	}
	switch value.Kind {
	case authzv4.DataScopeKind_ALL_STORES:
		if len(value.StoreIds) != 0 {
			return fmt.Errorf("all-store scope cannot select stores")
		}
	case authzv4.DataScopeKind_STORES:
		if len(value.StoreIds) == 0 {
			return fmt.Errorf("selected-store scope is empty")
		}
		for _, id := range value.StoreIds {
			if !positiveScopeID(id) {
				return fmt.Errorf("invalid scope store")
			}
		}
	default:
		return fmt.Errorf("unsupported data scope kind")
	}
	return nil
}
