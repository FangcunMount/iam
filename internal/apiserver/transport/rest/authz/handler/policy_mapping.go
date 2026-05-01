package handler

import (
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
)

func toPermissionResponses(permissions []authzDomain.Permission) []dto.PermissionResponse {
	policyRules := make([]dto.PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		policyRules = append(policyRules, dto.PermissionResponse{
			Subject:    "role:" + permission.RoleName,
			Domain:     permission.TenantID,
			Object:     permission.ResourceKey,
			Action:     permission.Action,
			ScopeType:  string(permission.Scope.Normalized().Kind),
			ScopeValue: permission.Scope.Normalized().Value,
		})
	}
	return policyRules
}

func emptyPolicyVersionResponse(tenantID string) dto.PolicyVersionResponse {
	return dto.PolicyVersionResponse{
		TenantID:  tenantID,
		Version:   0,
		ChangedBy: "",
		Reason:    "",
	}
}

func toPolicyVersionResponse(version *policyDomain.PolicyVersion) dto.PolicyVersionResponse {
	return dto.PolicyVersionResponse{
		TenantID:  version.TenantID,
		Version:   version.Version,
		ChangedBy: version.ChangedBy,
		Reason:    version.Reason,
	}
}
