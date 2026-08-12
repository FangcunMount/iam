package handler

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policylint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
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

func toPolicyLintResponse(report policylint.Report) dto.PolicyLintResponse {
	findings := make([]dto.PolicyLintFindingResponse, 0, len(report.Findings))
	for _, finding := range report.Findings {
		findings = append(findings, dto.PolicyLintFindingResponse{
			Code:        string(finding.Code),
			RoleName:    finding.RoleName,
			TenantID:    finding.TenantID,
			ResourceKey: finding.ResourceKey,
			Action:      finding.Action,
			Scope:       finding.Scope,
			Message:     finding.Message,
		})
	}
	return dto.PolicyLintResponse{Findings: findings}
}
