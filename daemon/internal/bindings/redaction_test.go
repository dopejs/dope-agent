package bindings

import (
	"strings"
	"testing"
	"time"
)

func TestSafeLabel(t *testing.T) {
	if got := SafeLabel("  Personal Workspace  "); got != "Personal Workspace" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := SafeLabel(""); got != "(unnamed)" {
		t.Fatalf("empty should be placeholder, got %q", got)
	}
	if got := SafeLabel("secret=hunter2"); got != "(redacted)" {
		t.Fatalf("unsafe should redact, got %q", got)
	}
	if got := SafeLabel("line\nbreak\tand   spaces"); strings.Contains(got, "\n") || strings.Contains(got, "\t") {
		t.Fatalf("control chars not stripped: %q", got)
	}
	long := strings.Repeat("a", 500)
	if got := SafeLabel(long); len(got) > maxSafeLabelLen {
		t.Fatalf("not truncated: len=%d", len(got))
	}
}

func TestSafeReason(t *testing.T) {
	if got := SafeReason("token=abc"); got != "redacted" {
		t.Fatalf("expected redacted, got %q", got)
	}
	if got := SafeReason(""); got != "unspecified" {
		t.Fatalf("expected unspecified, got %q", got)
	}
}

// B14/B20: denial evidence must never carry secrets; the builder routes through redaction.
func TestBuildRuntimeBindingEvidence_AppliedAndRedacted(t *testing.T) {
	sel := EffectiveBindingSelection{
		Outcome:             OutcomeResolved,
		BindingScope:        RuntimeScopeChannel,
		BindingID:           "bnd_1",
		SelectedProfileID:   "prof_1",
		SelectedWorkspaceID: "ws_1",
		CapabilityVisibility: []CapabilityDecision{
			{CapabilityID: "cap.x", Effective: EffectiveHidden, Reason: "token=leak", Scope: "workspace"},
		},
	}
	ev := BuildRuntimeBindingEvidence(sel, RuntimeBindingEvidenceInput{
		ProjectionID: "brp_1", TenantID: "ten_1", ResourceKind: "thread", ResourceID: "thr_1",
		OccurredAt: time.Unix(0, 0).UTC(),
	})
	if ev.Classification != ClassificationApplied {
		t.Fatalf("expected applied classification, got %s", ev.Classification)
	}
	if ev.CapabilityVisibility[0].Reason == "token=leak" {
		t.Fatalf("denial reason leaked secret: %q", ev.CapabilityVisibility[0].Reason)
	}
	if ev.RedactionStatus != RedactionRedacted {
		t.Fatalf("expected redacted status")
	}
}

func TestBuildRuntimeBindingEvidence_DefaultAndLegacy(t *testing.T) {
	def := BuildRuntimeBindingEvidence(EffectiveBindingSelection{Outcome: OutcomeDefault}, RuntimeBindingEvidenceInput{})
	if def.Classification != ClassificationDefault {
		t.Fatalf("expected default classification, got %s", def.Classification)
	}
	legacy := BuildRuntimeBindingEvidence(EffectiveBindingSelection{Outcome: OutcomeDefault}, RuntimeBindingEvidenceInput{LegacyDefault: true})
	if legacy.Classification != ClassificationLegacy {
		t.Fatalf("expected legacy classification, got %s", legacy.Classification)
	}
}
