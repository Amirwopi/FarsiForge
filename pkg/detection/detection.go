package detection

// DefaultRegistry creates and returns a registry populated with all
// built-in engine detectors.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Register all built-in detectors
	r.Register(&UnityDetector{})
	r.Register(&UE3Detector{})
	r.Register(&UnrealDetector{})
	r.Register(&GodotDetector{})

	// Classic / custom-engine detectors
	r.Register(&SAGEDetector{})
	r.Register(&GoldSrcDetector{})
	r.Register(&FromSoftwareDetector{})
	r.Register(&FactorioDetector{})
	r.Register(&ZomboidDetector{})
	r.Register(&SourceDetector{})
	r.Register(&Source2Detector{})
	r.Register(&RAGEDetector{})

	// Add other detectors as they are implemented...
	// r.Register(&RPGMakerDetector{})
	// r.Register(&GameMakerDetector{})
	// r.Register(&RenPyDetector{})
	// r.Register(&AdobeAIRDetector{})

	// Fallback must be registered last (or with lowest priority)
	r.Register(&GenericDetector{})

	return r
}
