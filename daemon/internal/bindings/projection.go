package bindings

import (
	"strings"
	"time"
)

// RuntimeBindingEvidenceInput carries the per-run facts needed to build durable runtime
// binding evidence in the same work-start pass as the profile runtime projection.
type RuntimeBindingEvidenceInput struct {
	ProjectionID string
	TenantID     string
	ResourceKind string
	ResourceID   string
	OccurredAt   time.Time
	// LegacyDefault marks runs that predate explicit binding support so evidence is
	// labeled legacy_default rather than presented as user-configured (FR-026).
	LegacyDefault bool
}

// BuildRuntimeBindingEvidence materializes an EffectiveBindingSelection into durable,
// redacted runtime evidence. It flips the planted profile-projection deferral marker by
// recording ClassificationApplied when an explicit binding influenced the run (FR-013,
// FR-026, B22).
func BuildRuntimeBindingEvidence(sel EffectiveBindingSelection, input RuntimeBindingEvidenceInput) RuntimeBindingEvidence {
	classification := ClassificationDefault
	switch {
	case input.LegacyDefault:
		classification = ClassificationLegacy
	case sel.Outcome == OutcomeResolved:
		classification = ClassificationApplied
	}

	reason := "tenant_default_selection"
	switch sel.Outcome {
	case OutcomeResolved:
		reason = "explicit_binding_selection"
	case OutcomeRepairRequired:
		reason = SafeReason(sel.RepairReason)
	}

	decisions := make([]CapabilityDecision, 0, len(sel.CapabilityVisibility))
	for _, d := range sel.CapabilityVisibility {
		decisions = append(decisions, CapabilityDecision{
			CapabilityID:   strings.TrimSpace(d.CapabilityID),
			Effective:      d.Effective,
			DefaultEnabled: d.DefaultEnabled,
			Offered:        d.Offered,
			Executable:     d.Executable,
			Reason:         SafeReason(d.Reason),
			Scope:          safeScopeLabel(d.Scope),
		})
	}

	return RuntimeBindingEvidence{
		ProjectionID:             strings.TrimSpace(input.ProjectionID),
		TenantID:                 strings.TrimSpace(input.TenantID),
		ResourceKind:             strings.TrimSpace(input.ResourceKind),
		ResourceID:               strings.TrimSpace(input.ResourceID),
		SelectedProfileID:        strings.TrimSpace(sel.SelectedProfileID),
		SelectedProfileVersionID: strings.TrimSpace(sel.SelectedProfileVersionID),
		SelectedWorkspaceID:      strings.TrimSpace(sel.SelectedWorkspaceID),
		BindingScope:             sel.BindingScope,
		BindingID:                strings.TrimSpace(sel.BindingID),
		Classification:           classification,
		SelectionReason:          reason,
		CapabilityVisibility:     decisions,
		OccurredAt:               input.OccurredAt,
		RedactionStatus:          RedactionRedacted,
	}
}
