package suggest

// CollectRuntime copies suggest background collaborators into runtime deps.
func CollectRuntime(mod *SuggestModule, cleanup *func() error) {
	if mod == nil || cleanup == nil {
		return
	}
	*cleanup = mod.RuntimeCapabilities().Cleanup
}
