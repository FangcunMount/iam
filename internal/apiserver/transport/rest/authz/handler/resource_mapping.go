package handler

import (
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// toResourceResponse 转换为响应对象
func (h *ResourceHandler) toResourceResponse(r *resourceDomain.Resource) dto.ResourceResponse {
	return dto.ResourceResponse{
		ID:              meta.FromUint64(r.ID.Uint64()),
		Key:             r.KeyString(),
		DisplayName:     r.DisplayName,
		AppName:         r.AppName,
		Domain:          r.Domain,
		Type:            r.Type,
		Actions:         r.ActionStrings(),
		AttributeSchema: r.AttributeSchema,
		Description:     r.Description,
	}
}
