package audit

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

const EvaluationProductAuditEventKind = "evaluation.product_audit_recorded"

const (
	EvaluationAuditActionDiscoveryPolicyChanged         = "discovery.policy_changed"
	EvaluationAuditActionDiscoveryStarted               = "discovery.started"
	EvaluationAuditActionDiscoveryCompleted             = "discovery.completed"
	EvaluationAuditActionDiscoveryPartial               = "discovery.partial"
	EvaluationAuditActionDiscoveryFailed                = "discovery.failed"
	EvaluationAuditActionCandidateSuggested             = "discovery.candidate_suggested"
	EvaluationAuditActionCandidateSuppressed            = "discovery.suppressed"
	EvaluationAuditActionDiscoveryRedactionFailed       = "discovery.redaction_failed"
	EvaluationAuditActionDiscoveryRetentionApplied      = "discovery.retention_applied"
	EvaluationAuditActionFixtureCreated                 = "fixture.created"
	EvaluationAuditActionFixtureRevisionCreated         = "fixture.revision_created"
	EvaluationAuditActionFixtureReviewed                = "fixture.reviewed"
	EvaluationAuditActionFixtureSuppressed              = "fixture.suppressed"
	EvaluationAuditActionFixtureArchived                = "fixture.archived"
	EvaluationAuditActionFixtureDeleted                 = "fixture.deleted"
	EvaluationAuditActionFixtureRedactionFailed         = "fixture.redaction_failed"
	EvaluationAuditActionCampaignCreated                = "campaign.created"
	EvaluationAuditActionCampaignStarted                = "campaign.started"
	EvaluationAuditActionCampaignCancelled              = "campaign.cancelled"
	EvaluationAuditActionCampaignCompleted              = "campaign.completed"
	EvaluationAuditActionCampaignFailed                 = "campaign.failed"
	EvaluationAuditActionCampaignResultsPublished       = "campaign.results_published"
	EvaluationAuditActionCampaignRedactionFailed        = "campaign.redaction_failed"
	EvaluationAuditActionDashboardGenerated             = "dashboard.projection_generated"
	EvaluationAuditActionToolInspectionGenerated        = "tool_call_inspection.generated"
	EvaluationAuditActionToolInspectionRedactionFailed  = "tool_call_inspection.redaction_failed"
	EvaluationAuditActionToolInspectionRetentionApplied = "tool_call_inspection.retention_applied"
)

type EvaluationProductAuditInput struct {
	TenantID       string
	PrincipalID    string
	Action         string
	TargetKind     evaluation.ProductResourceKind
	TargetID       string
	Outcome        string
	ReasonCode     string
	EvidenceRefs   []string
	Redaction      evaluation.RedactionStatus
	RetentionAppID string
	CreatedAt      time.Time
}

func BuildEvaluationProductAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	document := map[string]any{
		"action":     stringOrDefault(input.Action, "evaluation.product"),
		"targetKind": string(input.TargetKind),
	}
	if input.TargetID != "" {
		document["targetId"] = input.TargetID
	}
	if len(input.EvidenceRefs) > 0 {
		document["evidenceRefs"] = input.EvidenceRefs
	}
	if input.Redaction != "" {
		document["redactionStatus"] = string(input.Redaction)
	}
	if input.RetentionAppID != "" {
		document["retentionApplicationId"] = input.RetentionAppID
	}
	return identity.TenantAuditEvent{
		EventKind:   EvaluationProductAuditEventKind,
		TenantID:    input.TenantID,
		PrincipalID: input.PrincipalID,
		Outcome:     stringOrDefault(input.Outcome, identity.AuditOutcomeSucceeded),
		ReasonCode:  input.ReasonCode,
		CreatedAt:   createdAt.UTC(),
		Document:    document,
	}
}

func BuildEvaluationDiscoveryAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	if input.TargetKind == "" {
		input.TargetKind = evaluation.ProductResourceDiscoveryRun
	}
	return BuildEvaluationProductAuditEvent(input)
}

func BuildEvaluationFixtureAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	if input.TargetKind == "" {
		input.TargetKind = evaluation.ProductResourceProductFixture
	}
	return BuildEvaluationProductAuditEvent(input)
}

func BuildEvaluationCampaignAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	if input.TargetKind == "" {
		input.TargetKind = evaluation.ProductResourceCampaign
	}
	return BuildEvaluationProductAuditEvent(input)
}

func BuildEvaluationDashboardAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	if input.TargetKind == "" {
		input.TargetKind = evaluation.ProductResourceDashboardProjection
	}
	return BuildEvaluationProductAuditEvent(input)
}

func BuildEvaluationToolCallInspectionAuditEvent(input EvaluationProductAuditInput) identity.TenantAuditEvent {
	if input.TargetKind == "" {
		input.TargetKind = evaluation.ProductResourceToolCallInspection
	}
	return BuildEvaluationProductAuditEvent(input)
}
