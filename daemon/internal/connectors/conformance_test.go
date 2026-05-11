package connectors_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestConnectorSourceLinkageConformanceSurvivesRestartReplay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_replay",
		TenantID:                "ten_replay",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_replay",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack / channel_redacted",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	key := threads.SourceContinuationKey{
		TenantID:             "ten_replay",
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
	}
	if err := sqliteStore.SaveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      "src_replay_current",
		ThreadID:             "thr_replay",
		TenantID:             "ten_replay",
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          "slack-main",
		ConnectorKind:        "slack",
		SourceAccountID:      key.SourceAccountID,
		SourceConversationID: key.SourceConversationID,
		SourceMessageID:      "msg_1",
		RoutingOutcome:       threads.RoutingOutcomeAccepted,
		Current:              true,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restored, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore restored: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	current, found, err := restored.GetCurrentThreadForSource(ctx, key)
	if err != nil || !found {
		t.Fatalf("GetCurrentThreadForSource after restart found=%v err=%v", found, err)
	}
	if current.ThreadID != "thr_replay" || current.CurrentSessionSegmentID != "seg_replay" {
		t.Fatalf("connector replay source did not resolve to daemon-owned thread: %+v", current)
	}
}

func TestConnectorConformanceDeclaresGroupRoomEvidenceCapabilities(t *testing.T) {
	t.Parallel()

	core := map[connectors.ConformanceArea]connectors.ConformanceResultStatus{}
	for _, area := range connectors.CoreInvariantAreas() {
		core[area] = connectors.ConformanceResultPass
	}
	results, profile, err := connectors.RunMatrixCase(connectors.MatrixCase{
		ScenarioID:           "group_room_evidence",
		ConnectorKind:        connectors.ConnectorKindMatrix,
		ConnectorID:          "matrix-main",
		TenantID:             "ten_matrix",
		CoreInvariantResults: core,
		GroupRoomCapabilities: connectors.GroupRoomCapabilities{
			MentionEvidence:           connectors.SurfaceSupported,
			AllowlistEvidence:         connectors.SurfaceSupported,
			UnsupportedSourceEvidence: connectors.SurfaceLimited,
			DuplicateMessageEvidence:  connectors.SurfaceSupported,
			EditedMessageEvidence:     connectors.SurfaceUnsupported,
			DeletedMessageEvidence:    connectors.SurfaceUnsupported,
		},
		Now: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RunMatrixCase: %v", err)
	}
	if profile.GroupRoomCapabilities.MentionEvidence != connectors.SurfaceSupported ||
		profile.GroupRoomCapabilities.AllowlistEvidence != connectors.SurfaceSupported ||
		profile.GroupRoomCapabilities.UnsupportedSourceEvidence != connectors.SurfaceLimited ||
		profile.GroupRoomCapabilities.DuplicateMessageEvidence != connectors.SurfaceSupported ||
		profile.GroupRoomCapabilities.EditedMessageEvidence != connectors.SurfaceUnsupported ||
		profile.GroupRoomCapabilities.DeletedMessageEvidence != connectors.SurfaceUnsupported {
		t.Fatalf("profile did not retain group/room capabilities: %+v", profile.GroupRoomCapabilities)
	}
	want := map[string]connectors.ConformanceResultStatus{
		connectors.GroupRoomSurfaceMentionEvidence:           connectors.ConformanceResultSupported,
		connectors.GroupRoomSurfaceAllowlistEvidence:         connectors.ConformanceResultSupported,
		connectors.GroupRoomSurfaceUnsupportedSourceEvidence: connectors.ConformanceResultLimited,
		connectors.GroupRoomSurfaceDuplicateMessageEvidence:  connectors.ConformanceResultSupported,
		connectors.GroupRoomSurfaceEditedMessageEvidence:     connectors.ConformanceResultUnsupported,
		connectors.GroupRoomSurfaceDeletedMessageEvidence:    connectors.ConformanceResultUnsupported,
	}
	got := map[string]connectors.ConformanceResultStatus{}
	for _, result := range results {
		got[result.Area] = result.Result
	}
	for area, status := range want {
		if got[area] != status {
			t.Fatalf("result area %s = %s, want %s (all=%+v)", area, got[area], status, got)
		}
	}
}

func TestConnectorConformanceDeclaresHandoffCapabilities(t *testing.T) {
	t.Parallel()

	core := map[connectors.ConformanceArea]connectors.ConformanceResultStatus{}
	for _, area := range connectors.CoreInvariantAreas() {
		core[area] = connectors.ConformanceResultPass
	}
	results, profile, err := connectors.RunMatrixCase(connectors.MatrixCase{
		ScenarioID:           "handoff",
		ConnectorKind:        connectors.ConnectorKindMatrix,
		ConnectorID:          "matrix-main",
		TenantID:             "ten_matrix",
		CoreInvariantResults: core,
		HandoffCapabilities: connectors.HandoffCapabilities{
			SourceSupport:                 connectors.SurfaceSupported,
			DestinationSupport:            connectors.SurfaceLimited,
			FirstResponseSourceReferences: connectors.SurfaceSupported,
		},
		Now: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RunMatrixCase: %v", err)
	}
	if profile.HandoffCapabilities.SourceSupport != connectors.SurfaceSupported ||
		profile.HandoffCapabilities.DestinationSupport != connectors.SurfaceLimited ||
		profile.HandoffCapabilities.FirstResponseSourceReferences != connectors.SurfaceSupported {
		t.Fatalf("profile did not retain handoff capabilities: %+v", profile.HandoffCapabilities)
	}
	got := map[string]connectors.ConformanceResultStatus{}
	for _, result := range results {
		got[result.Area] = result.Result
	}
	want := map[string]connectors.ConformanceResultStatus{
		connectors.HandoffSurfaceSourceSupport:                 connectors.ConformanceResultSupported,
		connectors.HandoffSurfaceDestinationSupport:            connectors.ConformanceResultLimited,
		connectors.HandoffSurfaceFirstResponseSourceReferences: connectors.ConformanceResultSupported,
	}
	for area, status := range want {
		if got[area] != status {
			t.Fatalf("result area %s = %s, want %s (all=%+v)", area, got[area], status, got)
		}
	}
}

func TestConnectorConformanceDoesNotInferMemoryFromGroupRoomOrHandoff(t *testing.T) {
	t.Parallel()

	core := map[connectors.ConformanceArea]connectors.ConformanceResultStatus{}
	for _, area := range connectors.CoreInvariantAreas() {
		core[area] = connectors.ConformanceResultPass
	}
	_, profile, err := connectors.RunMatrixCase(connectors.MatrixCase{
		ScenarioID:           "non_memory_scope",
		ConnectorKind:        "slack",
		ConnectorID:          "slack-main",
		CoreInvariantResults: core,
		ProviderSurfaceResults: map[string]connectors.SurfaceSupport{
			"memory_based_team_context":  connectors.SurfaceUnsupported,
			"semantic_cross_room_recall": connectors.SurfaceUnsupported,
		},
		GroupRoomCapabilities: connectors.GroupRoomCapabilities{
			MentionEvidence:   connectors.SurfaceSupported,
			AllowlistEvidence: connectors.SurfaceSupported,
		},
		HandoffCapabilities: connectors.HandoffCapabilities{
			SourceSupport:                 connectors.SurfaceSupported,
			DestinationSupport:            connectors.SurfaceSupported,
			FirstResponseSourceReferences: connectors.SurfaceSupported,
		},
		Now: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RunMatrixCase: %v", err)
	}
	if profile.ProviderSurfaceResults["memory_based_team_context"] != connectors.SurfaceUnsupported ||
		profile.ProviderSurfaceResults["semantic_cross_room_recall"] != connectors.SurfaceUnsupported {
		t.Fatalf("group room handoff conformance implied memory-like capability: %+v", profile.ProviderSurfaceResults)
	}
}
