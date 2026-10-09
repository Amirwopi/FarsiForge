// Package detection provides the engine detection plugin registry.
package detection

import (
	"fmt"
	"sort"
	
	"farsiforge/pkg/core"
	"farsiforge/pkg/logging"
	"farsiforge/pkg/scanner"
)

var log = logging.Default().WithModule("detection")

// Registry holds all registered engine detectors.
type Registry struct {
	detectors []core.IEngineDetector
}

// NewRegistry creates a new empty detector registry.
func NewRegistry() *Registry {
	return &Registry{
		detectors: make([]core.IEngineDetector, 0),
	}
}

// Register adds a detector to the registry.
func (r *Registry) Register(detector core.IEngineDetector) {
	r.detectors = append(r.detectors, detector)
	// Sort by priority descending
	sort.Slice(r.detectors, func(i, j int) bool {
		return r.detectors[i].Priority() > r.detectors[j].Priority()
	})
	log.Debug("Registered detector", "name", detector.Name(), "priority", detector.Priority())
}

// Detect runs all detectors against a game directory and returns the best match.
func (r *Registry) Detect(gameDir string) (*core.DetectionResult, error) {
	var bestMatch *core.DetectionResult
	var highestConfidence float64

	log.Info("Starting engine detection", "dir", gameDir)
	
	if !scanner.DirExists(gameDir) {
		return nil, fmt.Errorf("directory does not exist: %s", gameDir)
	}

	for _, d := range r.detectors {
		log.Debug("Running detector", "name", d.Name())
		result, err := d.Detect(gameDir)
		if err != nil {
			log.Warn("Detector failed", "name", d.Name(), "error", err)
			continue
		}

		if result != nil {
			log.Debug("Detector matched", "name", d.Name(), "confidence", result.Confidence)
			if result.Confidence > highestConfidence {
				bestMatch = result
				highestConfidence = result.Confidence
			}
			
			// If we have a very high confidence match, short-circuit
			if highestConfidence >= 0.95 {
				break
			}
		}
	}

	if bestMatch == nil {
		return nil, core.ErrEngineNotDetected
	}

	log.Info("Engine detected", 
		"engine", bestMatch.Engine, 
		"version", bestMatch.Version, 
		"confidence", bestMatch.Confidence)

	return bestMatch, nil
}
