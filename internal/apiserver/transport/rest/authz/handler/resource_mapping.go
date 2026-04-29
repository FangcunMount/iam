package handler

import (
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/internal/pkg/meta"
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
		Description: r.Description,
	}
}
