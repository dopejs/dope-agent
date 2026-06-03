package bindings

import (
	"errors"
	"testing"
)

// B9/FR-016: a hidden or disabled capability decision blocks execution.
func TestEnforceExecutable(t *testing.T) {
	if err := EnforceExecutable(CapabilityDecision{Effective: EffectiveVisible, Executable: true}); err != nil {
		t.Fatalf("visible capability must be executable, got %v", err)
	}
	for _, d := range []CapabilityDecision{
		{Effective: EffectiveHidden},
		{Effective: EffectiveDisabled},
		{Effective: EffectiveBlocked},
	} {
		if err := EnforceExecutable(d); !errors.Is(err, ErrCapabilityNotExecutable) {
			t.Fatalf("effective %s must not be executable, got %v", d.Effective, err)
		}
	}
}

// B62/FR-021/SC-015: autonomous selection is bounded to policy-visible capabilities — the
// resolver never reports an executable decision for a hidden/disabled capability regardless
// of default_enabled.
func TestAutonomousSelectionBoundedToVisible(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:    "tool.fs",
		ProfilePolicy:   VisibilityDefaultEnabled,
		WorkspacePolicy: VisibilityDisabled,
	})
	if d.Offered || d.Executable {
		t.Fatalf("disabled capability must not be selectable, got %+v", d)
	}
	if err := EnforceExecutable(d); err == nil {
		t.Fatalf("expected enforcement to block disabled capability")
	}
}
