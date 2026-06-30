package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
)

// BindingLifecycleInput describes a binding/workspace lifecycle or denial event.
type BindingLifecycleInput struct {
	TenantID                  string
	BindingID                 string
	WorkspaceID               string
	ActorPrincipalID          string
	EventName                 string
	Outcome                   string
	ReasonCode                string
	PermissionGate            string
	SafeSummary               string
	PreviousSelectionSummary  string
	ResultingSelectionSummary string
	AuditEventID              string
}

// BindingLifecycleEvent constructs a tenant-scoped binding lifecycle/denial event
// (FR-010). Payload carries only safe, redacted fields (FR-028).
func BindingLifecycleEvent(input BindingLifecycleInput) Event {
	resourceID := input.BindingID
	if resourceID == "" {
		resourceID = input.WorkspaceID
	}
	return Event{
		Category:   "binding",
		Name:       input.EventName,
		TenantID:   input.TenantID,
		OccurredAt: time.Now().UTC(),
		Resource:   Resource{Kind: "workspace_capability_binding", ID: resourceID},
		Payload: map[string]any{
			"bindingId":                 input.BindingID,
			"workspaceId":               input.WorkspaceID,
			"actorPrincipalId":          input.ActorPrincipalID,
			"outcome":                   input.Outcome,
			"reasonCode":                input.ReasonCode,
			"permissionGate":            input.PermissionGate,
			"safeSummary":               bindings.SafeLabel(input.SafeSummary),
			"previousSelectionSummary":  bindings.SafeLabel(input.PreviousSelectionSummary),
			"resultingSelectionSummary": bindings.SafeLabel(input.ResultingSelectionSummary),
			"auditEventId":              input.AuditEventID,
			"redactionStatus":           string(bindings.RedactionRedacted),
		},
	}
}

// CapabilityVisibilityChangedInput describes a capability visibility change event.
type CapabilityVisibilityChangedInput struct {
	TenantID         string
	ActorPrincipalID string
	ScopeKind        bindings.VisibilityScopeKind
	ScopeRef         string
	CapabilityID     string
	Visibility       bindings.Visibility
	AuditEventID     string
}

// CapabilityVisibilityChangedEvent constructs a visibility change event.
func CapabilityVisibilityChangedEvent(input CapabilityVisibilityChangedInput) Event {
	return Event{
		Category:   "binding",
		Name:       "capability_visibility.changed",
		TenantID:   input.TenantID,
		OccurredAt: time.Now().UTC(),
		Resource:   Resource{Kind: "capability_visibility_policy", ID: bindings.SafeLabel(input.CapabilityID)},
		Payload: map[string]any{
			"actorPrincipalId": input.ActorPrincipalID,
			"scopeKind":        string(input.ScopeKind),
			"scopeRef":         bindings.SafeLabel(input.ScopeRef),
			"capabilityId":     bindings.SafeLabel(input.CapabilityID),
			"visibility":       string(input.Visibility),
			"auditEventId":     input.AuditEventID,
			"redactionStatus":  string(bindings.RedactionRedacted),
		},
	}
}

// BindingRuntimeProjectedEvent constructs a runtime binding evidence event (FR-013).
func BindingRuntimeProjectedEvent(evidence bindings.RuntimeBindingEvidence) Event {
	return Event{
		Category:   "binding",
		Name:       "binding.runtime_projected",
		TenantID:   evidence.TenantID,
		OccurredAt: evidence.OccurredAt,
		Resource:   Resource{Kind: evidence.ResourceKind, ID: evidence.ResourceID},
		Payload: map[string]any{
			"projectionId":        evidence.ProjectionID,
			"selectedProfileId":   evidence.SelectedProfileID,
			"selectedWorkspaceId": evidence.SelectedWorkspaceID,
			"bindingScope":        string(evidence.BindingScope),
			"bindingId":           evidence.BindingID,
			"classification":      string(evidence.Classification),
			"selectionReason":     bindings.SafeReason(evidence.SelectionReason),
			"redactionStatus":     string(evidence.RedactionStatus),
		},
	}
}
