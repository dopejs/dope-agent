package chat

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestHandoffSourceReferencesExcludePreResetTurns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	link := threads.HandoffLink{
		HandoffLinkID:                "handoff_reset_boundary",
		TenantID:                     "ten_1",
		SourceThreadID:               "thr_source",
		SourceSessionSegmentID:       "seg_after_reset",
		DestinationThreadID:          "thr_destination",
		DestinationSessionSegmentID:  "seg_destination",
		SourceConversationShape:      threads.ConversationShapeRoom,
		DestinationConversationShape: threads.ConversationShapeWeb,
	}
	refs := threads.BuildHandoffSourceReferences(link, []threads.ContinuityTurn{
		{
			ContinuityTurnID:       "turn_pre_reset",
			TenantID:               "ten_1",
			ThreadID:               "thr_source",
			SessionSegmentID:       "seg_before_reset",
			SafeContent:            "pre reset context",
			ContentRedactionStatus: threads.RedactionStatusRedacted,
			RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
		},
		{
			ContinuityTurnID:       "turn_after_reset",
			TenantID:               "ten_1",
			ThreadID:               "thr_source",
			SessionSegmentID:       "seg_after_reset",
			SafeContent:            "post reset context",
			ContentRedactionStatus: threads.RedactionStatusRedacted,
			RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
		},
	}, now)
	if len(refs) != 2 {
		t.Fatalf("refs = %+v", refs)
	}
	if refs[0].ContinuityTurnID != "turn_pre_reset" ||
		refs[0].Decision != threads.HandoffReferenceDecisionExcluded ||
		refs[0].EligibilityStatus != threads.HandoffReferenceResetBoundary {
		t.Fatalf("pre-reset turn became eligible handoff context: %+v", refs[0])
	}
	if refs[1].ContinuityTurnID != "turn_after_reset" ||
		refs[1].Decision != threads.HandoffReferenceDecisionReferenced ||
		refs[1].EligibilityStatus != threads.HandoffReferenceEligible {
		t.Fatalf("post-reset turn should remain eligible: %+v", refs[1])
	}
}

func TestFirstDestinationResponseUsesAndConsumesHandoffSourceReferences(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	for _, thread := range []threads.Thread{
		{ThreadID: "thr_source", TenantID: "ten_1", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_source", SourceKind: threads.SourceKindChannel, LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted},
		{ThreadID: "thr_destination", TenantID: "ten_1", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_destination", SourceKind: threads.SourceKindShell, LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted},
	} {
		if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
			t.Fatalf("UpsertThread %s: %v", thread.ThreadID, err)
		}
		if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: thread.CurrentSessionSegmentID, ThreadID: thread.ThreadID, TenantID: thread.TenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
			t.Fatalf("UpsertThreadSessionSegment %s: %v", thread.ThreadID, err)
		}
	}
	link, err := sqliteStore.SaveHandoffLink(ctx, threads.HandoffLink{
		HandoffLinkID:                "handoff_1",
		TenantID:                     "ten_1",
		SourceThreadID:               "thr_source",
		SourceSessionSegmentID:       "seg_source",
		DestinationThreadID:          "thr_destination",
		DestinationSessionSegmentID:  "seg_destination",
		SourceConversationShape:      threads.ConversationShapeRoom,
		DestinationConversationShape: threads.ConversationShapeWeb,
		Status:                       threads.HandoffStatusSucceeded,
		SourceReferenceStatus:        threads.HandoffSourceReferenceAvailable,
		PermissionGate:               "connectors.manage",
		CreatedAt:                    now,
		RedactionStatus:              threads.RedactionStatusRedacted,
	})
	if err != nil {
		t.Fatalf("SaveHandoffLink: %v", err)
	}
	if _, err := sqliteStore.SaveHandoffSourceReferences(ctx, []threads.HandoffSourceReference{{
		HandoffLinkID:               link.HandoffLinkID,
		TenantID:                    "ten_1",
		SourceThreadID:              "thr_source",
		SourceSessionSegmentID:      "seg_source",
		DestinationThreadID:         "thr_destination",
		DestinationSessionSegmentID: "seg_destination",
		ContinuityTurnID:            "turn_source_1",
		EligibilityStatus:           threads.HandoffReferenceEligible,
		Decision:                    threads.HandoffReferenceDecisionReferenced,
		SafeSummary:                 "safe handoff source context",
		RedactionStatus:             threads.RedactionStatusRedacted,
		CreatedAt:                   now,
		RetentionExpiresAt:          now.Add(90 * 24 * time.Hour),
	}, {
		HandoffLinkID:               link.HandoffLinkID,
		TenantID:                    "ten_1",
		SourceThreadID:              "thr_source",
		SourceSessionSegmentID:      "seg_source",
		DestinationThreadID:         "thr_destination",
		DestinationSessionSegmentID: "seg_destination",
		ContinuityTurnID:            "turn_source_redacted",
		EligibilityStatus:           threads.HandoffReferenceRedactionFailed,
		Decision:                    threads.HandoffReferenceDecisionExcluded,
		SafeSummary:                 "suppressed source context",
		RedactionStatus:             threads.RedactionStatusSuppressed,
		CreatedAt:                   now,
		RetentionExpiresAt:          now.Add(90 * 24 * time.Hour),
	}}); err != nil {
		t.Fatalf("SaveHandoffSourceReferences: %v", err)
	}

	provider := &capturingProvider{name: "handoff-context"}
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(provider)
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	result, err := service.Query(ctx, QueryInput{TenantID: "ten_1", ThreadID: "thr_destination", Provider: "handoff-context", Model: "model-a", Query: "first destination response"})
	if err != nil {
		t.Fatalf("first Query: %v", err)
	}
	if !provider.sawMessage("safe handoff source context") {
		t.Fatalf("first destination response did not receive handoff source reference: %+v", provider.requests)
	}
	preview, found, err := sqliteStore.GetContinuityPreviewDetail(ctx, "ten_1", "thr_destination", result.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	foundIncludedHandoff := false
	foundExcludedHandoff := false
	for _, item := range preview.Items {
		if item.ItemKind != threads.ContinuityItemHandoffSource {
			continue
		}
		if item.Decision == threads.ContinuityDecisionIncluded && item.ContinuityTurnID == "turn_source_1" {
			foundIncludedHandoff = true
		}
		if item.Decision == threads.ContinuityDecisionExcluded && item.ReasonCode == threads.ContinuityReasonRedactionFailed {
			foundExcludedHandoff = true
		}
	}
	if !foundIncludedHandoff || !foundExcludedHandoff {
		t.Fatalf("continuity preview did not classify handoff refs: %+v", preview.Items)
	}
	consumed, found, err := sqliteStore.GetHandoffLink(ctx, "ten_1", link.HandoffLinkID)
	if err != nil || !found {
		t.Fatalf("GetHandoffLink found=%v err=%v", found, err)
	}
	if consumed.SourceReferenceStatus != threads.HandoffSourceReferenceConsumed || consumed.FirstDestinationResponseID == "" {
		t.Fatalf("handoff link was not consumed after first response: %+v", consumed)
	}

	secondProvider := &capturingProvider{name: "handoff-context-2"}
	secondDispatcher := llm.NewDispatcher()
	secondDispatcher.RegisterProvider(secondProvider)
	secondService := NewService(secondDispatcher, nil, nil, events.NewBus(), sqliteStore)
	if _, err := secondService.Query(ctx, QueryInput{TenantID: "ten_1", ThreadID: "thr_destination", Provider: "handoff-context-2", Model: "model-a", Query: "second destination response"}); err != nil {
		t.Fatalf("second Query: %v", err)
	}
	if secondProvider.sawMessage("safe handoff source context") {
		t.Fatalf("second destination response reused consumed source reference: %+v", secondProvider.requests)
	}
}

func TestHandoffSourceReferenceAssemblyPerformance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	link := threads.HandoffLink{
		HandoffLinkID:          "handoff_perf",
		TenantID:               "ten_1",
		SourceThreadID:         "thr_source",
		SourceSessionSegmentID: "seg_source",
		DestinationThreadID:    "thr_destination",
	}
	turns := make([]threads.ContinuityTurn, 0, 64)
	for i := 0; i < 64; i++ {
		turns = append(turns, threads.ContinuityTurn{
			ContinuityTurnID:       "turn_perf",
			ThreadID:               "thr_source",
			SessionSegmentID:       "seg_source",
			SafeContent:            "safe source context",
			ContentRedactionStatus: threads.RedactionStatusRedacted,
			RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
		})
	}
	started := time.Now()
	for i := 0; i < 1000; i++ {
		refs := threads.BuildHandoffSourceReferences(link, turns, now)
		if len(refs) != len(turns) {
			t.Fatalf("refs=%d turns=%d", len(refs), len(turns))
		}
	}
	elapsed := time.Since(started)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("handoff source reference assembly exceeded 500ms p95 proxy: %s", elapsed)
	}
}
