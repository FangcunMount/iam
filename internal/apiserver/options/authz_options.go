package options

import authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"

type AuthzOptions struct {
	PolicySync             authzruntime.Config `mapstructure:"policy-sync" json:"policy_sync"`
	AttributeProvidersFile string              `mapstructure:"attribute-providers-file" json:"attribute_providers_file"`
}

func NewAuthzOptions() *AuthzOptions {
	return &AuthzOptions{PolicySync: authzruntime.DefaultConfig(), AttributeProvidersFile: "configs/authz_attribute_providers.yaml"}
}
