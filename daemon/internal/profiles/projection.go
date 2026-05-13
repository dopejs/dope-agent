package profiles

import "time"

type RuntimeProjectionInput struct {
	ResourceKind RuntimeResourceKind
	ResourceID   string
	ThreadID     string
	SessionID    string
	RunID        string
	WorkflowID   string
	HandoffID    string
	OccurredAt   time.Time
}

func BuildRuntimeProjection(profile AgentProfile, selection ActiveSelection, input RuntimeProjectionInput) RuntimeProjection {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return RuntimeProjection{
		TenantID:                      profile.TenantID,
		ProfileID:                     profile.ProfileID,
		ProfileVersionID:              selection.ProfileVersionID,
		SelectionID:                   selection.SelectionID,
		ResourceKind:                  input.ResourceKind,
		ResourceID:                    input.ResourceID,
		ThreadID:                      input.ThreadID,
		SessionSegmentID:              input.SessionID,
		RunID:                         input.RunID,
		WorkflowID:                    input.WorkflowID,
		HandoffLinkID:                 input.HandoffID,
		SelectionScope:                selection.SelectionScope,
		SelectionReason:               selection.SelectionReason,
		SafeDisplayName:               profile.DisplayName,
		SafeSummary:                   SafeProfileSummary(profile),
		ConfigurationScope:            "explicit_profile_configuration",
		DeferredBindingClassification: "roadmap_58_deferred_binding_unapplied",
		OccurredAt:                    occurredAt.UTC(),
		RedactionStatus:               RedactionRedacted,
	}
}

func DefaultLegacyMappingEvidence() []LegacyMappingEvidence {
	return []LegacyMappingEvidence{
		{SourceKind: "provider_defaults", MappingState: OverlayPartial, ReasonCode: "legacy_provider_default_partial", SafeSummary: "Provider defaults remain external unless explicitly copied into the profile.", RedactionStatus: RedactionRedacted},
		{SourceKind: "prompt_config_reference", MappingState: OverlayPartial, ReasonCode: "legacy_prompt_config_partial", SafeSummary: "Legacy prompt/config references are visible as partial evidence and are not hidden profile truth.", RedactionStatus: RedactionRedacted},
	}
}

func DefaultLegacyOverlayReferenceInputs() []OverlayReferenceInput {
	return []OverlayReferenceInput{
		{ReferenceKind: "legacy_provider_defaults", ReferenceURI: "legacy://provider-defaults/tenant-default", Scope: "profile"},
		{ReferenceKind: "legacy_prompt_config", ReferenceURI: "legacy://prompt-config/tenant-default", Scope: "profile"},
	}
}
