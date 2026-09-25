package visibility

// AuthorizationFacts 是 Suggest 所需的粗粒度授权事实。
type AuthorizationFacts struct {
	AllProfilesAllowed             bool
	AllProfilesMobileSearchAllowed bool
	ScopedMobileSearchAllowed      bool
}
