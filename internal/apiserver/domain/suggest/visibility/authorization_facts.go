package visibility

// AuthorizationFacts 是 Suggest 所需的粗粒度授权事实。
type AuthorizationFacts struct {
	PlatformListAllowed         bool
	PlatformMobileSearchAllowed bool
	TenantMobileSearchAllowed   bool
}
