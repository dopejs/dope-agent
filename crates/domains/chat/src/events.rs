//! Event emitters and wire-status helpers ported from the Go
//! `daemon/internal/events` package (`thread_continuity.go`,
//! `agent_profiles.go`, `workspace_capability_bindings.go`) and the
//! `daemon/internal/chat/service.go` ID generators.

use chrono::{DateTime, Utc};
use dope_bindings::RuntimeBindingEvidence;
use dope_events::{Event, Resource, Scope};
use dope_llm::DispatchStatus;
use dope_profiles::RuntimeProjection;
use dope_threads::{ContinuityPreview, ContinuityStatus, ContinuityTurn, RedactionStatus};
use serde_json::{Value, json};

/// Go `events.ThreadContinuityTurnRecordedName`.
pub const THREAD_CONTINUITY_TURN_RECORDED_NAME: &str = "thread.continuity_turn_recorded";
/// Go `events.ThreadContinuityPreviewRecordedName`.
pub const THREAD_CONTINUITY_PREVIEW_RECORDED_NAME: &str = "thread.continuity_preview_recorded";
/// Go `events.AgentProfileRuntimeProjectedEvent` name.
pub const AGENT_PROFILE_RUNTIME_PROJECTED_NAME: &str = "agent_profile.runtime_projected";
/// Go `events.BindingRuntimeProjectedEvent` name.
pub const BINDING_RUNTIME_PROJECTED_NAME: &str = "binding.runtime_projected";

/// Go `string(threads.RedactionStatus)`.
#[must_use]
pub fn redaction_status_str(status: RedactionStatus) -> &'static str {
    match status {
        RedactionStatus::Redacted => "redacted",
        RedactionStatus::Suppressed => "suppressed",
        RedactionStatus::RedactionFailed => "redaction_failed",
    }
}

/// Go `string(threads.ContinuityStatus)`.
#[must_use]
pub fn continuity_status_str(status: ContinuityStatus) -> &'static str {
    match status {
        ContinuityStatus::Applied => "applied",
        ContinuityStatus::Empty => "empty",
        ContinuityStatus::Disabled => "disabled",
        ContinuityStatus::Blocked => "blocked",
        ContinuityStatus::Partial => "partial",
        ContinuityStatus::Failed => "failed",
    }
}

/// Go `string(llm.DispatchStatus)`.
#[must_use]
pub fn dispatch_status_str(status: DispatchStatus) -> &'static str {
    match status {
        DispatchStatus::Queued => "queued",
        DispatchStatus::Running => "running",
        DispatchStatus::Completed => "completed",
        DispatchStatus::PartialFailed => "partial_failed",
        DispatchStatus::Failed => "failed",
        DispatchStatus::Cancelled => "cancelled",
    }
}

/// Go `newEventID`: `evt_` + 8 random bytes hex-encoded.
#[must_use]
pub fn new_event_id() -> String {
    let hex = uuid::Uuid::new_v4().simple().to_string();
    format!("evt_{}", &hex[..16])
}

/// Go `newContinuityPreviewID`: `contprev_` + 8 random bytes hex-encoded.
#[must_use]
pub fn new_continuity_preview_id() -> String {
    let hex = uuid::Uuid::new_v4().simple().to_string();
    format!("contprev_{}", &hex[..16])
}

/// Go's inline `if event.EventID == ""` / `if event.OccurredAt.IsZero()`
/// normalization applied before persisting/publishing an event.
pub(crate) fn normalize_event(mut event: Event) -> Event {
    if event.event_id.is_empty() {
        event.event_id = new_event_id();
    }
    if event.occurred_at == DateTime::<Utc>::UNIX_EPOCH {
        event.occurred_at = Utc::now();
    }
    event
}

/// Go `events.ThreadContinuityTurnRecordedEvent`.
#[must_use]
pub fn thread_continuity_turn_recorded_event(turn: &ContinuityTurn, outcome: &str) -> Event {
    let outcome = if outcome.is_empty() {
        "recorded"
    } else {
        outcome
    };
    let occurred_at = if turn.recorded_at == DateTime::<Utc>::UNIX_EPOCH {
        Utc::now()
    } else {
        turn.recorded_at
    };
    Event {
        tenant_id: turn.tenant_id.clone(),
        category: "thread".to_string(),
        name: THREAD_CONTINUITY_TURN_RECORDED_NAME.to_string(),
        occurred_at,
        scope: Scope {
            session_id: turn.session_segment_id.clone(),
            ..Scope::default()
        },
        resource: Resource {
            kind: "thread_continuity_turn".to_string(),
            id: turn.continuity_turn_id.clone(),
        },
        payload: object(&json!({
            "tenantId": turn.tenant_id,
            "threadId": turn.thread_id,
            "sessionSegmentId": turn.session_segment_id,
            "continuityTurnId": turn.continuity_turn_id,
            "dispatchId": turn.dispatch_id,
            "action": "turn_recorded",
            "outcome": outcome,
            "reasonCode": "included_recent",
            "redactionStatus": redaction_status_str(turn.content_redaction_status),
            "acceptanceSequence": turn.acceptance_sequence,
        })),
        ..Event::default()
    }
}

/// Go `events.ThreadContinuityPreviewRecordedEvent`.
#[must_use]
pub fn thread_continuity_preview_recorded_event(preview: &ContinuityPreview) -> Event {
    let occurred_at = if preview.assembly_completed_at == DateTime::<Utc>::UNIX_EPOCH {
        Utc::now()
    } else {
        preview.assembly_completed_at
    };
    Event {
        tenant_id: preview.tenant_id.clone(),
        category: "thread".to_string(),
        name: THREAD_CONTINUITY_PREVIEW_RECORDED_NAME.to_string(),
        occurred_at,
        scope: Scope {
            session_id: preview.session_segment_id.clone(),
            ..Scope::default()
        },
        resource: Resource {
            kind: "thread_continuity_preview".to_string(),
            id: preview.continuity_preview_id.clone(),
        },
        payload: object(&json!({
            "tenantId": preview.tenant_id,
            "threadId": preview.thread_id,
            "sessionSegmentId": preview.session_segment_id,
            "continuityPreviewId": preview.continuity_preview_id,
            "dispatchId": preview.dispatch_id,
            "action": "preview_recorded",
            "outcome": continuity_status_str(preview.status),
            "reasonCode": preview.failure_class,
            "redactionStatus": redaction_status_str(preview.redaction_status),
        })),
        ..Event::default()
    }
}

/// Go `events.AgentProfileRuntimeProjectedEvent`.
#[must_use]
pub fn agent_profile_runtime_projected_event(projection: &RuntimeProjection) -> Event {
    Event {
        category: "agent_profile".to_string(),
        name: AGENT_PROFILE_RUNTIME_PROJECTED_NAME.to_string(),
        tenant_id: projection.tenant_id.clone(),
        occurred_at: projection.occurred_at,
        resource: Resource {
            kind: projection.resource_kind.as_str().to_string(),
            id: projection.resource_id.clone(),
        },
        payload: object(&json!({
            "runtimeProfileProjectionId": projection.runtime_profile_projection_id,
            "profileId": projection.profile_id,
            "profileVersionId": projection.profile_version_id,
            "selectionId": projection.selection_id,
            "selectionScope": projection.selection_scope,
            "selectionReason": projection.selection_reason.as_str(),
            "safeDisplayName": projection.safe_display_name,
            "safeSummary": projection.safe_summary,
            "redactionStatus": projection.redaction_status.as_str(),
        })),
        ..Event::default()
    }
}

/// Go `events.BindingRuntimeProjectedEvent`.
#[must_use]
pub fn binding_runtime_projected_event(evidence: &RuntimeBindingEvidence) -> Event {
    Event {
        category: "binding".to_string(),
        name: BINDING_RUNTIME_PROJECTED_NAME.to_string(),
        tenant_id: evidence.tenant_id.clone(),
        occurred_at: evidence.occurred_at,
        resource: Resource {
            kind: evidence.resource_kind.clone(),
            id: evidence.resource_id.clone(),
        },
        payload: object(&json!({
            "projectionId": evidence.projection_id,
            "selectedProfileId": evidence.selected_profile_id,
            "selectedWorkspaceId": evidence.selected_workspace_id,
            "bindingScope": evidence.binding_scope.as_str(),
            "bindingId": evidence.binding_id,
            "classification": evidence.classification.as_str(),
            "selectionReason": dope_bindings::safe_reason(&evidence.selection_reason),
            "redactionStatus": evidence.redaction_status.as_str(),
        })),
        ..Event::default()
    }
}

fn object(value: &Value) -> serde_json::Map<String, serde_json::Value> {
    value.as_object().cloned().unwrap_or_default()
}
