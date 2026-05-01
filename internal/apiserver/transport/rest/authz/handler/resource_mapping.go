package handler

import (
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// toResourceResponse 转换为响应对象
func (h *ResourceHandler) toResourceResponse(r *resourceDomain.Resource) dto.ResourceResponse {
	return dto.ResourceResponse{
		ID:          meta.FromUint64(r.ID.Uint64()),
		Key:         r.Key,
		DisplayName: r.DisplayName,
		AppName:     r.AppName,
		Domain:      r.Domain,
		Type:        r.Type,
		Actions:     r.Actions,
		ScopeKinds:  fromDomainScopeKinds(r.ScopeKinds),
		Description: r.Description,
	}
}

func fromDomainScopeKinds(kinds []authzDomain.ScopeKind) []string {
	normalized := resourceDomain.NormalizeScopeKinds(kinds)
	values := make([]string, 0, len(normalized))
	for _, kind := range normalized {
		values = append(values, string(kind))
	}
	return values
}
