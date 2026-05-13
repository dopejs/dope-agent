package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

type AgentProfileLifecycleInput struct {
	TenantID         string
	ProfileID        string
	ProfileVersionID string
	ActorPrincipalID string
	EventName        string
	Outcome          string
	ReasonCode       string
	PermissionGate   string
	SafeSummary      string
	AuditEventID     string
	RedactionStatus  profiles.RedactionStatus
}

type AgentProfileVersionInput struct {
	TenantID         string
	ProfileID        string
	ProfileVersionID string
	ChangeKind       profiles.ChangeKind
	VersionNumber    int
	ReasonCode       string
	RedactionStatus  profiles.RedactionStatus
}

func AgentProfileLifecycleEvent(input AgentProfileLifecycleInput) Event {
	status := input.RedactionStatus
	if status == "" {
		status = profiles.RedactionRedacted
	}
	return Event{
		Category:   "agent_profile",
		Name:       input.EventName,
		TenantID:   input.TenantID,
		OccurredAt: time.Now().UTC(),
		Resource:   Resource{Kind: "agent_profile", ID: input.ProfileID},
		Payload: map[string]any{
			"profileId":        input.ProfileID,
			"profileVersionId": input.ProfileVersionID,
			"actorPrincipalId": input.ActorPrincipalID,
			"outcome":          input.Outcome,
			"reasonCode":       input.ReasonCode,
			"permissionGate":   input.PermissionGate,
			"safeSummary":      input.SafeSummary,
			"auditEventId":     input.AuditEventID,
			"redactionStatus":  string(status),
		},
	}
}

func AgentProfileVersionCreatedEvent(input AgentProfileVersionInput) Event {
	status := input.RedactionStatus
	if status == "" {
		status = profiles.RedactionRedacted
	}
	return Event{
		Category:   "agent_profile",
		Name:       "agent_profile.version_created",
		TenantID:   input.TenantID,
		OccurredAt: time.Now().UTC(),
		Resource:   Resource{Kind: "agent_profile_version", ID: input.ProfileVersionID},
		Payload: map[string]any{
			"profileId":        input.ProfileID,
			"profileVersionId": input.ProfileVersionID,
			"changeKind":       string(input.ChangeKind),
			"versionNumber":    input.VersionNumber,
			"reasonCode":       input.ReasonCode,
			"redactionStatus":  string(status),
		},
	}
}

func AgentProfileRuntimeProjectedEvent(projection profiles.RuntimeProjection) Event {
	return Event{
		Category:   "agent_profile",
		Name:       "agent_profile.runtime_projected",
		TenantID:   projection.TenantID,
		OccurredAt: projection.OccurredAt,
		Resource:   Resource{Kind: string(projection.ResourceKind), ID: projection.ResourceID},
		Payload: map[string]any{
			"runtimeProfileProjectionId": projection.RuntimeProfileProjectionID,
			"profileId":                  projection.ProfileID,
			"profileVersionId":           projection.ProfileVersionID,
			"selectionId":                projection.SelectionID,
			"selectionScope":             projection.SelectionScope,
			"selectionReason":            string(projection.SelectionReason),
			"safeDisplayName":            projection.SafeDisplayName,
			"safeSummary":                projection.SafeSummary,
			"redactionStatus":            string(projection.RedactionStatus),
		},
	}
}
