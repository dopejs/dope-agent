package threads

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildThreadResourceIsMetadataOnly(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	resource := BuildThreadResource(Thread{
		ThreadID:                "thr_1",
		TenantID:                "ten_1",
		LifecycleState:          LifecycleStateActive,
		CurrentSessionSegmentID: "seg_1",
		SourceKind:              SourceKindChannel,
		SourceSummary:           "Slack Main / #support",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         RedactionStatusRedacted,
		UpdatedAt:               now,
	}, "sess_1")
	if resource.ThreadID != "thr_1" || resource.CurrentSessionID != "sess_1" {
		t.Fatalf("unexpected resource identity: %#v", resource)
	}
	if len(resource.AvailableActions) != 2 || resource.AvailableActions[0] != LifecycleActionReset || resource.AvailableActions[1] != LifecycleActionArchive {
		t.Fatalf("unexpected active available actions: %#v", resource.AvailableActions)
	}
}

func TestBuildRuntimeProjectionSupportsOperatorTraceResourceKinds(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	kinds := []RuntimeResourceKind{
		RuntimeResourceSession,
		RuntimeResourceRun,
		RuntimeResourceWorkflow,
		RuntimeResourceApproval,
		RuntimeResourceForegroundReply,
		RuntimeResourceBackgroundDelivery,
		RuntimeResourceConnectorMessage,
	}
	for _, kind := range kinds {
		projection := BuildRuntimeProjection(RuntimeProjectionInput{
			ProjectionID:     "rtp_" + string(kind),
			ThreadID:         "thr_1",
			TenantID:         "ten_1",
			SessionSegmentID: "seg_1",
			ResourceKind:     kind,
			ResourceID:       "res_" + string(kind),
			Status:           "completed",
			ReasonCode:       "accepted",
			OccurredAt:       now,
			Route:            "/trace/" + string(kind),
			SafeSummary:      "metadata summary for " + string(kind),
		})
		if projection.ResourceKind != kind || projection.RuntimeProjectionID == "" || projection.ResourceID == "" {
			t.Fatalf("unexpected projection identity for %s: %#v", kind, projection)
		}
		if projection.SafeSummary == "" || projection.SafeSummary == "suppressed" || projection.RedactionStatus != RedactionStatusRedacted {
			t.Fatalf("projection should be metadata-only redacted summary for %s: %#v", kind, projection)
		}
	}
}

func TestRuntimeProjectionRejectsMemoryBehaviorFields(t *testing.T) {
	projection := BuildRuntimeProjection(RuntimeProjectionInput{
		ProjectionID: "rtp_no_memory",
		ThreadID:     "thr_1",
		TenantID:     "ten_1",
		ResourceKind: RuntimeResourceRun,
		ResourceID:   "run_1",
		Status:       "completed",
		OccurredAt:   time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
		SafeSummary:  "metadata only",
	})
	data, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("Marshal projection: %v", err)
	}
	raw := string(data)
	for _, forbidden := range []string{"semanticSummary", "recalledMemory", "contextPacking", "autonomousPruning"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("projection leaked memory behavior field %s in %s", forbidden, raw)
		}
	}
}
