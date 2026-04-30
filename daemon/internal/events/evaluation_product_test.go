package events

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductAuditEventConstruction(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := EvaluationProductAuditEvent(EvaluationProductRetentionAppliedName, EvaluationProductAuditPayload{
		TenantID:       "ten_eval",
		ActorID:        "prn_eval",
		Action:         "retention.apply",
		TargetKind:     evaluation.ProductResourceDiscoveredCandidate,
		TargetID:       "candidate_1",
		Outcome:        "retention_applied",
		ReasonCode:     "evaluation.retention_applied",
		RetentionAppID: "retention_1",
		OccurredAt:     now,
	})
	if event.Category != "evaluation" || event.Name != EvaluationProductRetentionAppliedName || event.TenantID != "ten_eval" {
		t.Fatalf("unexpected event envelope: %+v", event)
	}
	if event.Resource.Kind != string(evaluation.ProductResourceDiscoveredCandidate) || event.Payload["retentionApplicationId"] != "retention_1" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
}

func TestEvaluationDiscoveryEventConstruction(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := EvaluationDiscoveryEvent(EvaluationDiscoveryCandidateName, EvaluationDiscoveryPayload{
		TenantID:              "ten_eval",
		PolicyID:              "policy_1",
		DiscoveryRunID:        "discovery_run_1",
		DiscoveredCandidateID: "candidate_1",
		Status:                evaluation.ProductStatusPartial,
		ReasonCode:            "max_inspected_records",
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		OccurredAt:            now,
	})
	if event.Category != "evaluation" || event.Name != EvaluationDiscoveryCandidateName || event.TenantID != "ten_eval" {
		t.Fatalf("unexpected event envelope: %+v", event)
	}
	if event.Resource.Kind != string(evaluation.ProductResourceDiscoveredCandidate) || event.Resource.ID != "candidate_1" {
		t.Fatalf("unexpected discovery resource: %+v", event.Resource)
	}
	if event.Payload["discoveryRunId"] != "discovery_run_1" || event.Payload["redactionStatus"] != string(evaluation.RedactionStatusRedacted) {
		t.Fatalf("unexpected discovery payload: %+v", event.Payload)
	}
}

func TestEvaluationFixtureEventConstruction(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := EvaluationFixtureEvent(EvaluationFixtureCreatedName, EvaluationFixturePayload{
		TenantID:           "ten_eval",
		ActorID:            "prn_eval",
		FixtureID:          "product_fixture_1",
		RevisionID:         "revision_1",
		SourceCandidateID:  "candidate_1",
		SourceEvidenceRefs: []string{"evidence_1"},
		ReviewState:        evaluation.ProductStatusDraft,
		RedactionStatus:    evaluation.RedactionStatusRedacted,
		Outcome:            "created",
		OccurredAt:         now,
	})
	if event.Category != "evaluation" || event.Name != EvaluationFixtureCreatedName || event.Resource.ID != "product_fixture_1" {
		t.Fatalf("unexpected fixture event: %+v", event)
	}
	if event.Payload["revisionId"] != "revision_1" || event.Payload["redactionStatus"] != string(evaluation.RedactionStatusRedacted) {
		t.Fatalf("unexpected fixture payload: %+v", event.Payload)
	}
}

func TestEvaluationCampaignDashboardAndInspectionEventConstruction(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := EvaluationCampaignEvent(EvaluationCampaignResultsPublishedName, EvaluationCampaignPayload{
		TenantID:        "ten_eval",
		ActorID:         "prn_eval",
		CampaignID:      "campaign_1",
		CampaignItemID:  "campaign_item_1",
		AttemptGroupID:  "attempt_group_1",
		Status:          evaluation.ProductStatusPublished,
		Outcome:         "published",
		RedactionStatus: evaluation.RedactionStatusRedacted,
		OccurredAt:      now,
	})
	if campaign.Name != EvaluationCampaignResultsPublishedName || campaign.Resource.Kind != string(evaluation.ProductResourceCampaign) || campaign.Payload["status"] != string(evaluation.ProductStatusPublished) {
		t.Fatalf("unexpected campaign event: %+v", campaign)
	}

	dashboard := EvaluationDashboardEvent(EvaluationDashboardProjectionGeneratedName, EvaluationDashboardPayload{
		TenantID:     "ten_eval",
		ProjectionID: "projection_1",
		WindowStart:  now.Add(-time.Hour),
		WindowEnd:    now,
		GeneratedAt:  now,
		Outcome:      "generated",
		OccurredAt:   now,
	})
	if dashboard.Name != EvaluationDashboardProjectionGeneratedName || dashboard.Resource.Kind != string(evaluation.ProductResourceDashboardProjection) || dashboard.Payload["projectionId"] != "projection_1" {
		t.Fatalf("unexpected dashboard event: %+v", dashboard)
	}

	inspection := EvaluationToolCallInspectionEvent(EvaluationToolCallInspectionGeneratedName, EvaluationToolCallInspectionPayload{
		TenantID:        "ten_eval",
		InspectionID:    "inspection_1",
		CampaignID:      "campaign_1",
		CampaignItemID:  "campaign_item_1",
		Classification:  evaluation.InspectionMatched,
		RedactionStatus: evaluation.RedactionStatusClean,
		Outcome:         "generated",
		OccurredAt:      now,
	})
	if inspection.Name != EvaluationToolCallInspectionGeneratedName || inspection.Resource.Kind != string(evaluation.ProductResourceToolCallInspection) || inspection.Payload["classification"] != evaluation.InspectionMatched {
		t.Fatalf("unexpected inspection event: %+v", inspection)
	}
}
