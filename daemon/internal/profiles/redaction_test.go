package profiles

import (
	"errors"
	"testing"
)

func TestProfileValidationRejectsUnsafeAndDeferredOverlayInputs(t *testing.T) {
	if err := ValidateMutation(MutationInput{DisplayName: "bad", Persona: Persona{Instructions: "token=secret"}}); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("expected unsafe persona rejection, got %v", err)
	}
	if err := ValidateMutation(MutationInput{DisplayName: "bad", OverlayReferences: []OverlayReferenceInput{{ReferenceURI: "profiles/base.md", Scope: "thread"}}}); !errors.Is(err, ErrScopedBindingDeferred) {
		t.Fatalf("expected deferred scoped binding rejection, got %v", err)
	}
}

func TestRedactProfileSuppressesRawPersonaAndKeepsSafeSummary(t *testing.T) {
	profile := RedactProfile(AgentProfile{
		DisplayName:     "Support",
		DisplayIdentity: DisplayIdentity{Name: "Support", Description: "raw description"},
		Persona:         Persona{Tone: "direct", Instructions: "raw instructions"},
	})
	if profile.DisplayIdentity.Description != "" || profile.Persona.Instructions != "" {
		t.Fatalf("raw profile fields were not redacted: %+v", profile)
	}
	if profile.DisplayIdentity.SafeSummary == "" || profile.Persona.SafeSummary == "" {
		t.Fatalf("safe summaries were not populated: %+v", profile)
	}
}

func TestNormalizeOverlayRecordsUnsafeOutOfScopeEvidence(t *testing.T) {
	overlay, err := NormalizeOverlay(OverlayReferenceInput{ReferenceURI: "../secret-token=abc", ReferenceKind: "prompt_file"})
	if err != nil {
		t.Fatalf("NormalizeOverlay returned error: %v", err)
	}
	if overlay.ValidationState != OverlayUnsafeContent || overlay.RedactionStatus != RedactionRedacted || overlay.SafeDisplayLabel == "" {
		t.Fatalf("unexpected overlay evidence: %+v", overlay)
	}
}

func TestProfileAndOverlaySummariesDoNotExposeSecrets(t *testing.T) {
	profile := RedactProfile(AgentProfile{
		DisplayName:     "Support",
		DisplayIdentity: DisplayIdentity{Name: "Support", Description: "api_key=hidden"},
		Persona:         Persona{Tone: "direct", Instructions: "token=hidden"},
	})
	if profile.DisplayIdentity.Description != "" || profile.Persona.Instructions != "" {
		t.Fatalf("profile retained unsafe raw text: %+v", profile)
	}
	if containsUnsafe(SafeProfileSummary(profile)) {
		t.Fatalf("safe profile summary contains unsafe content: %q", SafeProfileSummary(profile))
	}
	overlay, err := NormalizeOverlay(OverlayReferenceInput{ReferenceKind: "prompt", ReferenceURI: "prompt://token=hidden"})
	if err != nil {
		t.Fatalf("NormalizeOverlay returned error: %v", err)
	}
	if overlay.ValidationState != OverlayUnsafeContent || containsUnsafe(overlay.SafeDisplayLabel) {
		t.Fatalf("overlay did not preserve redacted unsafe evidence: %+v", overlay)
	}
}
