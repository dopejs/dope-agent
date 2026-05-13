package profiles

import (
	"path/filepath"
	"strings"
)

func RedactProfile(profile AgentProfile) AgentProfile {
	profile.RedactionStatus = RedactionRedacted
	profile.DisplayIdentity.Description = ""
	if profile.DisplayIdentity.SafeSummary == "" {
		profile.DisplayIdentity.SafeSummary = safeSummary(profile.DisplayIdentity.Name, profile.DisplayName)
	}
	profile.Persona.Instructions = ""
	if profile.Persona.SafeSummary == "" {
		profile.Persona.SafeSummary = safeSummary(profile.Persona.Tone, "structured profile")
	}
	return profile
}

func NormalizeOverlay(input OverlayReferenceInput) (OverlayReference, error) {
	uri := strings.TrimSpace(input.ReferenceURI)
	if uri == "" {
		return OverlayReference{ValidationState: OverlayMissing, FailureReasonCode: "overlay_reference_missing"}, nil
	}
	state := OverlayValid
	reason := ""
	if strings.HasPrefix(uri, "/") || strings.Contains(uri, "..") {
		state = OverlayOutOfScope
		reason = "overlay_out_of_scope"
	}
	if containsUnsafe(uri) {
		state = OverlayUnsafeContent
		reason = "overlay_unsafe_content"
	}
	label := filepath.Base(uri)
	if containsUnsafe(label) {
		label = "overlay reference suppressed"
	}
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = "profile"
	}
	return OverlayReference{
		ReferenceKind:     defaultString(strings.TrimSpace(input.ReferenceKind), "prompt_file"),
		Scope:             scope,
		ReferenceURI:      uri,
		SafeDisplayLabel:  label,
		ValidationState:   state,
		FailureReasonCode: reason,
		RedactionStatus:   RedactionRedacted,
	}, nil
}

func SafeProfileSummary(profile AgentProfile) string {
	if profile.Persona.SafeSummary != "" {
		return profile.Persona.SafeSummary
	}
	if profile.DisplayIdentity.SafeSummary != "" {
		return profile.DisplayIdentity.SafeSummary
	}
	return safeSummary(profile.DisplayName, "profile")
}

func safeSummary(preferred, fallback string) string {
	preferred = strings.TrimSpace(preferred)
	if preferred == "" {
		return fallback
	}
	if len(preferred) > 160 {
		return preferred[:160]
	}
	return preferred
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
