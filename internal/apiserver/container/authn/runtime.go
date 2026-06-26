package authn

// CollectRuntime copies authn background collaborators into runtime deps.
func CollectRuntime(mod *AuthnModule, rotationScheduler *KeyRotationScheduler) {
	if mod == nil || rotationScheduler == nil {
		return
	}
	*rotationScheduler = mod.RuntimeCapabilities().RotationScheduler
}
