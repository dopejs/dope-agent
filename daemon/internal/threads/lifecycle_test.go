package threads

import (
	"testing"
	"time"
)

func TestLifecycleResetPreservesThreadAndCreatesNewSegment(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := Thread{ThreadID: "thr_1", TenantID: "ten_1", LifecycleState: LifecycleStateActive, CurrentSessionSegmentID: "seg_old"}

	updated, action, segment, err := ResetThread(thread, LifecycleMutationInput{
		ActorPrincipalID: "prn_1",
		ReasonCode:       "user_requested_reset",
		AuditEventID:     "audit_1",
		Now:              now,
		NewSegmentID:     "seg_new",
	})
	if err != nil {
		t.Fatalf("ResetThread returned error: %v", err)
	}
	if updated.ThreadID != thread.ThreadID || updated.LifecycleState != LifecycleStateReset || updated.CurrentSessionSegmentID != "seg_new" {
		t.Fatalf("unexpected reset thread: %#v", updated)
	}
	if action.PriorState != LifecycleStateActive || action.ResultingState != LifecycleStateReset || action.AuditEventID != "audit_1" {
		t.Fatalf("unexpected reset action: %#v", action)
	}
	if segment.ThreadID != thread.ThreadID || segment.SessionSegmentID != "seg_new" || segment.ResetFromSessionSegment != "seg_old" || !segment.StartedAt.Equal(now) {
		t.Fatalf("unexpected reset segment: %#v", segment)
	}
}

func TestLifecycleArchiveAndReopenRules(t *testing.T) {
	now := time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC)
	thread := Thread{ThreadID: "thr_1", TenantID: "ten_1", LifecycleState: LifecycleStateActive, CurrentSessionSegmentID: "seg_1"}
	archived, action, err := ArchiveThread(thread, LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_archive", Now: now})
	if err != nil {
		t.Fatalf("ArchiveThread returned error: %v", err)
	}
	if archived.LifecycleState != LifecycleStateArchived || action.ResultingState != LifecycleStateArchived {
		t.Fatalf("unexpected archive result: %#v %#v", archived, action)
	}
	reopened, reopenAction, err := ReopenThread(archived, LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_reopen", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("ReopenThread returned error: %v", err)
	}
	if reopened.LifecycleState != LifecycleStateReopened || reopenAction.PriorState != LifecycleStateArchived {
		t.Fatalf("unexpected reopen result: %#v %#v", reopened, reopenAction)
	}
	if _, _, err := ArchiveThread(reopened, LifecycleMutationInput{ActorPrincipalID: "prn_1"}); err == nil {
		t.Fatal("expected archive without audit evidence to fail closed")
	}
}
