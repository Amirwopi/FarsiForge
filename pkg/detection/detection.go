package detection

// DefaultRegistry creates and returns a registry populated with all
// built-in engine detectors.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	
	// Register all built-in detectors
	r.Register(&UnityDetector{})
	r.Register(&UnrealDetector{})
	r.Register(&GodotDetector{})
	
	// Add other detectors as they are implemented...
	// r.Register(&RPGMakerDetector{})
	// r.Register(&GameMakerDetector{})
	// r.Register(&RenPyDetector{})
	// r.Register(&SourceDetector{})
	// r.Register(&GoldSrcDetector{})
	// r.Register(&AdobeAIRDetector{})
	
	// Fallback must be registered last (or with lowest priority)
	r.Register(&GenericDetector{})
	
	return r
}
