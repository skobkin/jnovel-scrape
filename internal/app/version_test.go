package app

import "testing"

// TestVersionVariableIsSettable exercises the build-time injection contract.
// The release pipeline passes -ldflags "-X ... internal/app.Version=vX.Y.Z"
// to set the variable; this test verifies the variable is a plain `var`
// (not a function or constant) and that both values round-trip.
func TestVersionVariableIsSettable(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "0.1.0-test"
	if Version != "0.1.0-test" {
		t.Fatalf("Version variable not settable, got %q", Version)
	}

	Version = "dev"
	if Version != "dev" {
		t.Fatalf("Version variable not resettable, got %q", Version)
	}
}
