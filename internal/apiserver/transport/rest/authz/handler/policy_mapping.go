package handler

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
)

func toPermissionResponses(permissions []permission.Permission) []dto.PermissionResponse {
	policyRules := make([]dto.PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		policyRules = append(policyRules, dto.PermissionResponse{
			Subject:    "role:" + permission.RoleNameString(),
			Domain:     permission.TenantIDString(),
			Object:     permission.ResourceKeyString(),
			Action:     permission.ActionString(),
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
		TenantID:  version.TenantIDString(),
		Version:   version.Version,
		ChangedBy: version.ChangedBy,
		Reason:    version.Reason,
	}
}
