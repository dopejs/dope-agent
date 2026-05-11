package events

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadLifecycleEvents(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	action := threads.LifecycleAction{
		ThreadID:        "thr_1",
		TenantID:        "ten_1",
		ActionKind:      threads.LifecycleActionReset,
		ResultingState:  threads.LifecycleStateReset,
		AuditEventID:    "audit_1",
		ReasonCode:      "user_requested_reset",
		CompletedAt:     now,
		RedactionStatus: threads.RedactionStatusRedacted,
	}
	event := ThreadLifecycleEvent(action)
	if event.Category != "thread" || event.Name != "thread.lifecycle_reset" || event.TenantID != "ten_1" {
		t.Fatalf("unexpected lifecycle event: %#v", event)
	}
	if event.Payload["auditEventId"] != "audit_1" || event.Resource.ID != "thr_1" {
		t.Fatalf("unexpected lifecycle payload: %#v", event.Payload)
	}
}

func TestThreadSourceRuntimeRetentionAndFailureEvents(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	source := ThreadSourceLinkedEvent(threads.SourceLinkage{
		SourceLinkageID: "src_1",
		ThreadID:        "thr_1",
		TenantID:        "ten_1",
		RoutingOutcome:  threads.RoutingOutcomeAccepted,
		LinkedAt:        now,
		RedactionStatus: threads.RedactionStatusRedacted,
	})
	if source.Name != ThreadSourceLinkedName || source.Payload["routingOutcome"] != "accepted" {
		t.Fatalf("unexpected source event: %#v", source)
	}
	runtime := ThreadRuntimeProjectionEvent(threads.RuntimeProjection{
		RuntimeProjectionID: "rtp_1",
		ThreadID:            "thr_1",
		TenantID:            "ten_1",
		ResourceKind:        threads.RuntimeResourceRun,
		ResourceID:          "run_1",
		Status:              "completed",
		OccurredAt:          now,
		RedactionStatus:     threads.RedactionStatusRedacted,
	})
	if runtime.Name != ThreadRuntimeProjectionName || runtime.Payload["resourceKind"] != "run" {
		t.Fatalf("unexpected runtime projection event: %#v", runtime)
	}
	retention := ThreadRetentionAppliedEvent("ten_1", "thr_1", now, threads.RedactionStatusRedacted)
	if retention.Name != ThreadRetentionAppliedName || retention.Payload["retentionExpiresAt"] == "" {
		t.Fatalf("unexpected retention event: %#v", retention)
	}
	redaction := ThreadRedactionFailedEvent("ten_1", "thr_1", "unsafe_provider_detail")
	if redaction.Name != ThreadRedactionFailedName || redaction.Payload["reasonCode"] != "unsafe_provider_detail" {
		t.Fatalf("unexpected redaction event: %#v", redaction)
	}
	audit := ThreadAuditFailedClosedEvent("ten_1", "thr_1", "audit_unavailable")
	if audit.Name != ThreadAuditFailedClosedName || audit.Payload["outcome"] != "failed_closed" {
		t.Fatalf("unexpected audit event: %#v", audit)
	}
	recovery := ThreadRestartRecoveryEvent("ten_1", 4, 1, 1)
	if recovery.Name != ThreadRestartRecoveredName || recovery.Payload["partialThreadStates"] != 1 || recovery.Payload["semanticMemoryInteraction"] != "none" {
		t.Fatalf("unexpected restart recovery event: %#v", recovery)
	}
}
