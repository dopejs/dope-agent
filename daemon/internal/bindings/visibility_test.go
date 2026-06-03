package bindings

import "testing"

func TestVisibility_NoPolicyDefaultsVisible(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{CapabilityID: "cap.read"})
	if d.Effective != EffectiveVisible || !d.Offered || !d.Executable {
		t.Fatalf("expected visible/offered/executable, got %+v", d)
	}
	if d.DefaultEnabled {
		t.Fatalf("no policy should not be default-enabled")
	}
}

func TestVisibility_HiddenWinsOverVisible(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:    "cap.x",
		ProfilePolicy:   VisibilityVisible,
		WorkspacePolicy: VisibilityHidden,
	})
	if d.Effective != EffectiveHidden || d.Offered || d.Executable {
		t.Fatalf("hidden must win, got %+v", d)
	}
	if d.Scope != "workspace" {
		t.Fatalf("expected workspace scope as winner, got %s", d.Scope)
	}
}

func TestVisibility_DisabledWinsOverHidden(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:    "cap.x",
		ProfilePolicy:   VisibilityDisabled,
		WorkspacePolicy: VisibilityHidden,
	})
	if d.Effective != EffectiveDisabled || d.Executable {
		t.Fatalf("disabled must win, got %+v", d)
	}
}

// FR-017: a higher-level blocked limit wins over everything, including default_enabled.
func TestVisibility_BlockedLimitWins(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:    "cap.x",
		Limits:          []ScopeVisibility{{Scope: "connector", Blocked: true}},
		ProfilePolicy:   VisibilityDefaultEnabled,
		WorkspacePolicy: VisibilityVisible,
	})
	if d.Effective != EffectiveBlocked || d.Offered || d.Executable {
		t.Fatalf("blocked must win, got %+v", d)
	}
	if d.Scope != "connector" {
		t.Fatalf("expected connector scope, got %s", d.Scope)
	}
}

// FR-019: default_enabled must not override a stricter hidden/disabled policy.
func TestVisibility_DefaultEnabledDoesNotOverrideHidden(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:    "cap.x",
		ProfilePolicy:   VisibilityDefaultEnabled,
		WorkspacePolicy: VisibilityHidden,
	})
	if d.Effective != EffectiveHidden || d.DefaultEnabled {
		t.Fatalf("default_enabled must not override hidden, got %+v", d)
	}
}

func TestVisibility_DefaultEnabledWhenAllowed(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:  "cap.x",
		ProfilePolicy: VisibilityDefaultEnabled,
	})
	if d.Effective != EffectiveVisible || !d.DefaultEnabled || !d.Offered {
		t.Fatalf("expected default-enabled visible, got %+v", d)
	}
}

func TestVisibility_TenantLimitHiddenWins(t *testing.T) {
	d := ResolveCapabilityVisibility(VisibilityInput{
		CapabilityID:  "cap.x",
		Limits:        []ScopeVisibility{{Scope: "tenant", Visibility: VisibilityHidden}},
		ProfilePolicy: VisibilityVisible,
	})
	if d.Effective != EffectiveHidden || d.Scope != "tenant" {
		t.Fatalf("tenant limit should win, got %+v", d)
	}
}

func TestResolveVisibilitySet_PreservesOrder(t *testing.T) {
	out := ResolveVisibilitySet([]VisibilityInput{
		{CapabilityID: "a"},
		{CapabilityID: "b", ProfilePolicy: VisibilityHidden},
	})
	if len(out) != 2 || out[0].CapabilityID != "a" || out[1].CapabilityID != "b" {
		t.Fatalf("order not preserved: %+v", out)
	}
	if out[1].Effective != EffectiveHidden {
		t.Fatalf("second decision wrong: %+v", out[1])
	}
}
