package inject

import (
	"context"
	"testing"

	"farsiforge/pkg/core"
)

func TestGodotInjectorDoesNotClaimUnsupportedWriteSupport(t *testing.T) {
	injector := &GodotInjector{}
	if injector.Capabilities().TextInjection {
		t.Fatal("Godot injector reports text injection before the writer is implemented")
	}
	if result, err := injector.Inject(context.Background(), &core.GameInfo{}, &core.Project{}, nil, core.PersianOptions{}); err == nil || result != nil {
		t.Fatalf("Inject() = (%v, %v), want an explicit unsupported error", result, err)
	}
}
