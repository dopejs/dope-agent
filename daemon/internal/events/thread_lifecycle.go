package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

const (
	ThreadLifecycleResetName    = "thread.lifecycle_reset"
	ThreadLifecycleArchivedName = "thread.lifecycle_archived"
	ThreadLifecycleReopenedName = "thread.lifecycle_reopened"
	ThreadSourceLinkedName      = "thread.source_linked"
	ThreadRuntimeProjectionName = "thread.runtime_projection_recorded"
	ThreadRetentionAppliedName  = "thread.retention_applied"
	ThreadRedactionFailedName   = "thread.redaction_failed"
	ThreadAuditFailedClosedName = "thread.audit_failed_closed"
	ThreadRestartRecoveredName  = "thread.restart_recovered"
)

func ThreadLifecycleEvent(action threads.LifecycleAction) Event {
	occurredAt := action.CompletedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   action.TenantID,
		Category:   "thread",
		Name:       lifecycleEventName(action.ActionKind),
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: action.ResultingSessionSegment,
		},
		Resource: Resource{
			Kind: "thread",
			ID:   action.ThreadID,
		},
		Payload: map[string]any{
			"tenantId":         action.TenantID,
			"threadId":         action.ThreadID,
			"sessionSegmentId": action.ResultingSessionSegment,
			"action":           string(action.ActionKind),
			"outcome":          action.Status,
			"auditEventId":     action.AuditEventID,
			"reasonCode":       action.ReasonCode,
			"redactionStatus":  string(action.RedactionStatus),
		},
	}
}

func ThreadRestartRecoveryEvent(tenantID string, checkedThreads, projectedLegacySessions, partialThreadStates int) Event {
	return Event{
		TenantID:   tenantID,
		Category:   "thread",
		Name:       ThreadRestartRecoveredName,
		OccurredAt: time.Now().UTC(),
		Resource: Resource{
			Kind: "thread_lifecycle_recovery",
			ID:   tenantID,
		},
		Payload: map[string]any{
			"tenantId":                  tenantID,
			"checkedThreads":            checkedThreads,
			"projectedLegacySessions":   projectedLegacySessions,
			"partialThreadStates":       partialThreadStates,
			"outcome":                   "recovered",
			"ambiguousStatePolicy":      "metadata_only_partial_evidence",
			"redactionStatus":           string(threads.RedactionStatusRedacted),
			"semanticMemoryInteraction": "none",
		},
	}
}

func ThreadSourceLinkedEvent(link threads.SourceLinkage) Event {
	occurredAt := link.LinkedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   link.TenantID,
		Category:   "thread",
		Name:       ThreadSourceLinkedName,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: "thread_source_linkage",
			ID:   link.SourceLinkageID,
		},
		Payload: map[string]any{
			"tenantId":        link.TenantID,
			"threadId":        link.ThreadID,
			"sourceLinkageId": link.SourceLinkageID,
			"routingOutcome":  string(link.RoutingOutcome),
			"redactionStatus": string(link.RedactionStatus),
		},
	}
}

func ThreadRuntimeProjectionEvent(projection threads.RuntimeProjection) Event {
	occurredAt := projection.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   projection.TenantID,
		Category:   "thread",
		Name:       ThreadRuntimeProjectionName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: projection.SessionSegmentID,
		},
		Resource: Resource{
			Kind: "thread_runtime_projection",
			ID:   projection.RuntimeProjectionID,
		},
		Payload: map[string]any{
			"tenantId":            projection.TenantID,
			"threadId":            projection.ThreadID,
			"sessionSegmentId":    projection.SessionSegmentID,
			"runtimeProjectionId": projection.RuntimeProjectionID,
			"resourceKind":        string(projection.ResourceKind),
			"resourceId":          projection.ResourceID,
			"status":              projection.Status,
			"redactionStatus":     string(projection.RedactionStatus),
		},
	}
}

func ThreadRetentionAppliedEvent(tenantID, threadID string, expiresAt time.Time, status threads.RedactionStatus) Event {
	if status == "" {
		status = threads.RedactionStatusRedacted
	}
	return Event{
		TenantID:   tenantID,
		Category:   "thread",
		Name:       ThreadRetentionAppliedName,
		OccurredAt: time.Now().UTC(),
		Resource: Resource{
			Kind: "thread",
			ID:   threadID,
		},
		Payload: map[string]any{
			"tenantId":           tenantID,
			"threadId":           threadID,
			"retentionExpiresAt": expiresAt.UTC().Format(time.RFC3339Nano),
			"redactionStatus":    string(status),
		},
	}
}

func ThreadRedactionFailedEvent(tenantID, threadID, reasonCode string) Event {
	return threadFailureEvent(ThreadRedactionFailedName, tenantID, threadID, reasonCode, "redaction_failed", threads.RedactionStatusRedactionFailed)
}

func ThreadAuditFailedClosedEvent(tenantID, threadID, reasonCode string) Event {
	return threadFailureEvent(ThreadAuditFailedClosedName, tenantID, threadID, reasonCode, "failed_closed", threads.RedactionStatusRedacted)
}

func lifecycleEventName(kind threads.LifecycleActionKind) string {
	switch kind {
	case threads.LifecycleActionArchive:
		return ThreadLifecycleArchivedName
	case threads.LifecycleActionReopen:
		return ThreadLifecycleReopenedName
	default:
		return ThreadLifecycleResetName
	}
}

func threadFailureEvent(name, tenantID, threadID, reasonCode, outcome string, status threads.RedactionStatus) Event {
	return Event{
		TenantID:   tenantID,
		Category:   "thread",
		Name:       name,
		OccurredAt: time.Now().UTC(),
		Resource: Resource{
			Kind: "thread",
			ID:   threadID,
		},
		Payload: map[string]any{
			"tenantId":        tenantID,
			"threadId":        threadID,
			"outcome":         outcome,
			"reasonCode":      reasonCode,
			"redactionStatus": string(status),
		},
	}
}
