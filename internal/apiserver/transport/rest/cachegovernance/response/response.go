package response

import (
	cachegovernance "github.com/FangcunMount/iam/v3/internal/apiserver/application/cachegovernance"
)

type CatalogResponse struct {
	Total    int                        `json:"total"`
	Families []FamilyDescriptorResponse `json:"families"`
}

type OverviewResponse struct {
	RuntimeStatuses []RuntimeStatusResponse `json:"runtime_statuses"`
	Families        []FamilyViewResponse    `json:"families"`
}

type FamilyViewResponse struct {
	Descriptor FamilyDescriptorResponse `json:"descriptor"`
	Status     FamilyStatusResponse     `json:"status"`
}

type FamilyDescriptorResponse struct {
	Family          string               `json:"family"`
	Backend         string               `json:"backend"`
	RedisType       string               `json:"redis_data_type"`
	Codec           string               `json:"value_codec"`
	Role            string               `json:"data_role"`
	OwnerModule     string               `json:"owner_module"`
	KeyPattern      string               `json:"key_pattern"`
	TTLSource       string               `json:"ttl_source"`
	SelectionReason string               `json:"selection_reason"`
	Capabilities    []string             `json:"capabilities"`
	Policy          FamilyPolicyResponse `json:"policy"`
}

type FamilyPolicyResponse struct {
	TTLSource                      string `json:"ttl_source"`
	WriteMode                      string `json:"write_mode"`
	InvalidationMode               string `json:"invalidation_mode"`
	HasInternalRefreshCoordination bool   `json:"has_internal_refresh_coordination"`
}

type FamilyStatusResponse struct {
	Family          string   `json:"family"`
	Configured      bool     `json:"configured"`
	Healthy         bool     `json:"healthy"`
	EntryCountKnown bool     `json:"entry_count_known"`
	Notes           []string `json:"notes"`
}

type RuntimeStatusResponse struct {
	Backend    string   `json:"backend"`
	Configured bool     `json:"configured"`
	Healthy    bool     `json:"healthy"`
	Notes      []string `json:"notes"`
}

func FromCatalog(descriptors []cachegovernance.FamilyDescriptor) CatalogResponse {
	families := make([]FamilyDescriptorResponse, 0, len(descriptors))
	for _, descriptor := range descriptors {
		families = append(families, fromDescriptor(descriptor))
	}
	return CatalogResponse{
		Total:    len(families),
		Families: families,
	}
}

func FromOverview(overview cachegovernance.Overview) OverviewResponse {
	runtimeStatuses := make([]RuntimeStatusResponse, 0, len(overview.RuntimeStatuses))
	for _, status := range overview.RuntimeStatuses {
		runtimeStatuses = append(runtimeStatuses, fromRuntimeStatus(status))
	}

	families := make([]FamilyViewResponse, 0, len(overview.Families))
	for _, family := range overview.Families {
		families = append(families, FromFamilyView(family))
	}

	return OverviewResponse{
		RuntimeStatuses: runtimeStatuses,
		Families:        families,
	}
}

func FromFamilyView(view cachegovernance.FamilyView) FamilyViewResponse {
	return FamilyViewResponse{
		Descriptor: fromDescriptor(view.Descriptor),
		Status:     fromFamilyStatus(view.Status),
	}
}

func fromDescriptor(descriptor cachegovernance.FamilyDescriptor) FamilyDescriptorResponse {
	capabilities := make([]string, 0, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		capabilities = append(capabilities, string(capability))
	}

	return FamilyDescriptorResponse{
		Family:          string(descriptor.Family),
		Backend:         string(descriptor.Backend),
		RedisType:       string(descriptor.RedisType),
		Codec:           string(descriptor.Codec),
		Role:            string(descriptor.Role),
		OwnerModule:     descriptor.OwnerModule,
		KeyPattern:      descriptor.KeyPattern,
		TTLSource:       descriptor.TTLSource,
		SelectionReason: descriptor.SelectionReason,
		Capabilities:    capabilities,
		Policy: FamilyPolicyResponse{
			TTLSource:                      descriptor.Policy.TTLSource,
			WriteMode:                      descriptor.Policy.WriteMode,
			InvalidationMode:               descriptor.Policy.InvalidationMode,
			HasInternalRefreshCoordination: descriptor.Policy.HasInternalRefreshCoordination,
		},
	}
}

func fromFamilyStatus(status cachegovernance.FamilyStatus) FamilyStatusResponse {
	return FamilyStatusResponse{
		Family:          string(status.Family),
		Configured:      status.Configured,
		Healthy:         status.Healthy,
		EntryCountKnown: status.EntryCountKnown,
		Notes:           append([]string{}, status.Notes...),
	}
}

func fromRuntimeStatus(status cachegovernance.RuntimeStatus) RuntimeStatusResponse {
	return RuntimeStatusResponse{
		Backend:    string(status.Backend),
		Configured: status.Configured,
		Healthy:    status.Healthy,
		Notes:      append([]string{}, status.Notes...),
	}
}
