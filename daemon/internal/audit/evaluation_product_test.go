package audit

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestEvaluationProductAuditEventConstruction(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := BuildEvaluationProductAuditEvent(EvaluationProductAuditInput{
		TenantID:       "ten_eval",
		PrincipalID:    "prn_eval",
		Action:         "retention.apply",
		TargetKind:     evaluation.ProductResourceDiscoveredCandidate,
		TargetID:       "candidate_1",
		Outcome:        identity.AuditOutcomeSucceeded,
		ReasonCode:     "evaluation.retention_applied",
		EvidenceRefs:   []string{"evidence_1"},
		Redaction:      evaluation.RedactionStatusRedacted,
		RetentionAppID: "retention_1",
		CreatedAt:      now,
	})
	if event.EventKind != EvaluationProductAuditEventKind || event.TenantID != "ten_eval" || event.PrincipalID != "prn_eval" {
		t.Fatalf("unexpected audit identity: %+v", event)
	}
	if event.Document["targetKind"] != string(evaluation.ProductResourceDiscoveredCandidate) || event.Document["retentionApplicationId"] != "retention_1" {
		t.Fatalf("audit document lost product evidence: %+v", event.Document)
	}
}

func TestEvaluationDiscoveryAuditEventDefaultsTargetKind(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := BuildEvaluationDiscoveryAuditEvent(EvaluationProductAuditInput{
		TenantID:    "ten_eval",
		PrincipalID: "prn_eval",
		Action:      EvaluationAuditActionDiscoveryPartial,
		TargetID:    "discovery_run_1",
		Outcome:     identity.AuditOutcomeSucceeded,
		ReasonCode:  "max_inspected_records",
		CreatedAt:   now,
	})
	if event.Document["targetKind"] != string(evaluation.ProductResourceDiscoveryRun) {
		t.Fatalf("target kind=%v, want discovery run", event.Document["targetKind"])
	}
	if event.Document["action"] != EvaluationAuditActionDiscoveryPartial || event.ReasonCode != "max_inspected_records" {
		t.Fatalf("unexpected discovery audit event: %+v", event)
	}
}

func TestEvaluationFixtureAuditEventDefaultsTargetKind(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	event := BuildEvaluationFixtureAuditEvent(EvaluationProductAuditInput{
		TenantID:     "ten_eval",
		PrincipalID:  "prn_eval",
		Action:       EvaluationAuditActionFixtureReviewed,
		TargetID:     "product_fixture_1",
		Outcome:      identity.AuditOutcomeSucceeded,
		ReasonCode:   "approved",
		EvidenceRefs: []string{"revision_1"},
		CreatedAt:    now,
	})
	if event.Document["targetKind"] != string(evaluation.ProductResourceProductFixture) {
		t.Fatalf("target kind=%v, want product fixture", event.Document["targetKind"])
	}
	if event.Document["action"] != EvaluationAuditActionFixtureReviewed || event.ReasonCode != "approved" {
		t.Fatalf("unexpected fixture audit event: %+v", event)
	}
}

func TestEvaluationCampaignDashboardAndInspectionAuditEventDefaultsTargetKinds(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := BuildEvaluationCampaignAuditEvent(EvaluationProductAuditInput{
		TenantID:    "ten_eval",
		PrincipalID: "prn_eval",
		Action:      EvaluationAuditActionCampaignResultsPublished,
		TargetID:    "campaign_1",
		Outcome:     identity.AuditOutcomeSucceeded,
		CreatedAt:   now,
	})
	if campaign.Document["targetKind"] != string(evaluation.ProductResourceCampaign) || campaign.Document["action"] != EvaluationAuditActionCampaignResultsPublished {
		t.Fatalf("unexpected campaign audit: %+v", campaign)
	}

	dashboard := BuildEvaluationDashboardAuditEvent(EvaluationProductAuditInput{
		TenantID:    "ten_eval",
		PrincipalID: "prn_eval",
		Action:      EvaluationAuditActionDashboardGenerated,
		TargetID:    "projection_1",
		Outcome:     identity.AuditOutcomeSucceeded,
		CreatedAt:   now,
	})
	if dashboard.Document["targetKind"] != string(evaluation.ProductResourceDashboardProjection) || dashboard.Document["action"] != EvaluationAuditActionDashboardGenerated {
		t.Fatalf("unexpected dashboard audit: %+v", dashboard)
	}

	inspection := BuildEvaluationToolCallInspectionAuditEvent(EvaluationProductAuditInput{
		TenantID:     "ten_eval",
		PrincipalID:  "prn_eval",
		Action:       EvaluationAuditActionToolInspectionGenerated,
		TargetID:     "inspection_1",
		Outcome:      identity.AuditOutcomeSucceeded,
		EvidenceRefs: []string{"original_1", "replay_1", "ledger_1"},
		Redaction:    evaluation.RedactionStatusRedacted,
		CreatedAt:    now,
	})
	if inspection.Document["targetKind"] != string(evaluation.ProductResourceToolCallInspection) || inspection.Document["redactionStatus"] != string(evaluation.RedactionStatusRedacted) {
		t.Fatalf("unexpected inspection audit: %+v", inspection)
	}
}
