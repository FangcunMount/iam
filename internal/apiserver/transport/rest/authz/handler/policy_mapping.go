package handler

import (
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
)

func toPolicyRuleResponses(rules []policyDomain.PolicyRule) []dto.PolicyRuleResponse {
	policyRules := make([]dto.PolicyRuleResponse, 0, len(rules))
	for _, rule := range rules {
		policyRules = append(policyRules, dto.PolicyRuleResponse{
			Subject: rule.Sub,
			Domain:  rule.Dom,
			Object:  rule.Obj,
			Action:  rule.Act,
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
