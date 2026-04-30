package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

const (
	EvaluationProductAuditRecordedName               = "evaluation.product_audit_recorded"
	EvaluationProductRedactionFailedName             = "evaluation.product_redaction_failed"
	EvaluationProductRetentionAppliedName            = "evaluation.product_retention_applied"
	EvaluationDiscoveryPolicyChangedName             = "evaluation.discovery_policy_changed"
	EvaluationDiscoveryStartedName                   = "evaluation.discovery_started"
	EvaluationDiscoveryCompletedName                 = "evaluation.discovery_completed"
	EvaluationDiscoveryPartialName                   = "evaluation.discovery_partial"
	EvaluationDiscoveryFailedName                    = "evaluation.discovery_failed"
	EvaluationDiscoveryCandidateName                 = "evaluation.discovery_candidate_suggested"
	EvaluationDiscoverySuppressedName                = "evaluation.discovery_suppressed"
	EvaluationDiscoveryRedactionFailedName           = "evaluation.discovery_redaction_failed"
	EvaluationDiscoveryRetentionAppliedName          = "evaluation.discovery_retention_applied"
	EvaluationFixtureCreatedName                     = "evaluation.fixture.created"
	EvaluationFixtureRevisionCreatedName             = "evaluation.fixture.revision_created"
	EvaluationFixtureReviewedName                    = "evaluation.fixture.reviewed"
	EvaluationFixtureRedactionFailedName             = "evaluation.fixture.redaction_failed"
	EvaluationFixtureSuppressedName                  = "evaluation.fixture.suppressed"
	EvaluationFixtureArchivedName                    = "evaluation.fixture.archived"
	EvaluationFixtureDeletedName                     = "evaluation.fixture.deleted"
	EvaluationFixtureDeniedName                      = "evaluation.fixture.denied"
	EvaluationCampaignCreatedName                    = "evaluation.campaign.created"
	EvaluationCampaignStartedName                    = "evaluation.campaign.started"
	EvaluationCampaignCancelledName                  = "evaluation.campaign.cancelled"
	EvaluationCampaignCompletedName                  = "evaluation.campaign.completed"
	EvaluationCampaignFailedName                     = "evaluation.campaign.failed"
	EvaluationCampaignResultsPublishedName           = "evaluation.campaign.results_published"
	EvaluationCampaignRedactionFailedName            = "evaluation.campaign.redaction_failed"
	EvaluationDashboardProjectionGeneratedName       = "evaluation.dashboard.projection_generated"
	EvaluationToolCallInspectionGeneratedName        = "evaluation.tool_call_inspection.generated"
	EvaluationToolCallInspectionRedactionFailedName  = "evaluation.tool_call_inspection.redaction_failed"
	EvaluationToolCallInspectionRetentionAppliedName = "evaluation.tool_call_inspection.retention_applied"
)

type EvaluationProductAuditPayload struct {
	TenantID       string
	ActorID        string
	Action         string
	TargetKind     evaluation.ProductResourceKind
	TargetID       string
	Outcome        string
	ReasonCode     string
	EvidenceRefs   []string
	RetentionAppID string
	OccurredAt     time.Time
}

func EvaluationProductAuditEvent(name string, input EvaluationProductAuditPayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"tenantId":   input.TenantID,
		"actorId":    input.ActorID,
		"action":     input.Action,
		"targetKind": string(input.TargetKind),
		"outcome":    input.Outcome,
		"createdAt":  occurredAt.UTC(),
	}
	if input.TargetID != "" {
		payload["targetId"] = input.TargetID
	}
	if input.ReasonCode != "" {
		payload["reasonCode"] = input.ReasonCode
	}
	if len(input.EvidenceRefs) > 0 {
		payload["evidenceRefs"] = input.EvidenceRefs
	}
	if input.RetentionAppID != "" {
		payload["retentionApplicationId"] = input.RetentionAppID
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: string(input.TargetKind),
			ID:   input.TargetID,
		},
		Payload: payload,
	}
}

type EvaluationDiscoveryPayload struct {
	TenantID               string
	PolicyID               string
	DiscoveryRunID         string
	DiscoveredCandidateID  string
	SuppressionID          string
	Status                 evaluation.ProductLifecycleStatus
	ReasonCode             string
	RedactionStatus        evaluation.RedactionStatus
	RetentionApplicationID string
	OccurredAt             time.Time
}

func EvaluationDiscoveryEvent(name string, input EvaluationDiscoveryPayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	resourceKind := string(evaluation.ProductResourceDiscoveryRun)
	resourceID := input.DiscoveryRunID
	if input.DiscoveredCandidateID != "" {
		resourceKind = string(evaluation.ProductResourceDiscoveredCandidate)
		resourceID = input.DiscoveredCandidateID
	} else if input.SuppressionID != "" {
		resourceKind = string(evaluation.ProductResourceSuppression)
		resourceID = input.SuppressionID
	} else if input.PolicyID != "" && input.DiscoveryRunID == "" {
		resourceKind = string(evaluation.ProductResourceDiscoveryPolicy)
		resourceID = input.PolicyID
	}
	payload := map[string]any{
		"tenantId": input.TenantID,
		"status":   string(input.Status),
	}
	if input.PolicyID != "" {
		payload["policyId"] = input.PolicyID
	}
	if input.DiscoveryRunID != "" {
		payload["discoveryRunId"] = input.DiscoveryRunID
	}
	if input.DiscoveredCandidateID != "" {
		payload["discoveredCandidateId"] = input.DiscoveredCandidateID
	}
	if input.SuppressionID != "" {
		payload["suppressionId"] = input.SuppressionID
	}
	if input.ReasonCode != "" {
		payload["reasonCode"] = input.ReasonCode
	}
	if input.RedactionStatus != "" {
		payload["redactionStatus"] = string(input.RedactionStatus)
	}
	if input.RetentionApplicationID != "" {
		payload["retentionApplicationId"] = input.RetentionApplicationID
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: resourceKind,
			ID:   resourceID,
		},
		Payload: payload,
	}
}

type EvaluationFixturePayload struct {
	TenantID           string
	ActorID            string
	FixtureID          string
	RevisionID         string
	SourceCandidateID  string
	SourceEvidenceRefs []string
	ReviewState        evaluation.ProductLifecycleStatus
	RedactionStatus    evaluation.RedactionStatus
	Outcome            string
	ReasonCode         string
	OccurredAt         time.Time
}

func EvaluationFixtureEvent(name string, input EvaluationFixturePayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"tenantId":  input.TenantID,
		"fixtureId": input.FixtureID,
		"outcome":   input.Outcome,
	}
	if input.ActorID != "" {
		payload["actorId"] = input.ActorID
	}
	if input.RevisionID != "" {
		payload["revisionId"] = input.RevisionID
	}
	if input.SourceCandidateID != "" {
		payload["sourceCandidateId"] = input.SourceCandidateID
	}
	if len(input.SourceEvidenceRefs) > 0 {
		payload["sourceEvidenceRefs"] = input.SourceEvidenceRefs
	}
	if input.ReviewState != "" {
		payload["reviewState"] = string(input.ReviewState)
	}
	if input.RedactionStatus != "" {
		payload["redactionStatus"] = string(input.RedactionStatus)
	}
	if input.ReasonCode != "" {
		payload["reasonCode"] = input.ReasonCode
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: string(evaluation.ProductResourceProductFixture),
			ID:   input.FixtureID,
		},
		Payload: payload,
	}
}

type EvaluationCampaignPayload struct {
	TenantID        string
	ActorID         string
	CampaignID      string
	CampaignItemID  string
	AttemptGroupID  string
	Status          evaluation.ProductLifecycleStatus
	Outcome         string
	ReasonCode      string
	RedactionStatus evaluation.RedactionStatus
	OccurredAt      time.Time
}

func EvaluationCampaignEvent(name string, input EvaluationCampaignPayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"tenantId":   input.TenantID,
		"campaignId": input.CampaignID,
		"status":     string(input.Status),
	}
	if input.ActorID != "" {
		payload["actorId"] = input.ActorID
	}
	if input.CampaignItemID != "" {
		payload["campaignItemId"] = input.CampaignItemID
	}
	if input.AttemptGroupID != "" {
		payload["attemptGroupId"] = input.AttemptGroupID
	}
	if input.Outcome != "" {
		payload["outcome"] = input.Outcome
	}
	if input.ReasonCode != "" {
		payload["reasonCode"] = input.ReasonCode
	}
	if input.RedactionStatus != "" {
		payload["redactionStatus"] = string(input.RedactionStatus)
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: string(evaluation.ProductResourceCampaign),
			ID:   input.CampaignID,
		},
		Payload: payload,
	}
}

type EvaluationDashboardPayload struct {
	TenantID     string
	ProjectionID string
	WindowStart  time.Time
	WindowEnd    time.Time
	GeneratedAt  time.Time
	Outcome      string
	OccurredAt   time.Time
}

func EvaluationDashboardEvent(name string, input EvaluationDashboardPayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = occurredAt
	}
	payload := map[string]any{
		"tenantId":     input.TenantID,
		"projectionId": input.ProjectionID,
		"generatedAt":  generatedAt.UTC(),
	}
	if !input.WindowStart.IsZero() {
		payload["windowStart"] = input.WindowStart.UTC()
	}
	if !input.WindowEnd.IsZero() {
		payload["windowEnd"] = input.WindowEnd.UTC()
	}
	if input.Outcome != "" {
		payload["outcome"] = input.Outcome
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: string(evaluation.ProductResourceDashboardProjection),
			ID:   input.ProjectionID,
		},
		Payload: payload,
	}
}

type EvaluationToolCallInspectionPayload struct {
	TenantID               string
	InspectionID           string
	CampaignID             string
	CampaignItemID         string
	Classification         string
	RedactionStatus        evaluation.RedactionStatus
	RetentionApplicationID string
	Outcome                string
	ReasonCode             string
	OccurredAt             time.Time
}

func EvaluationToolCallInspectionEvent(name string, input EvaluationToolCallInspectionPayload) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"tenantId":       input.TenantID,
		"inspectionId":   input.InspectionID,
		"campaignId":     input.CampaignID,
		"classification": input.Classification,
	}
	if input.CampaignItemID != "" {
		payload["campaignItemId"] = input.CampaignItemID
	}
	if input.RedactionStatus != "" {
		payload["redactionStatus"] = string(input.RedactionStatus)
	}
	if input.RetentionApplicationID != "" {
		payload["retentionApplicationId"] = input.RetentionApplicationID
	}
	if input.Outcome != "" {
		payload["outcome"] = input.Outcome
	}
	if input.ReasonCode != "" {
		payload["reasonCode"] = input.ReasonCode
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: string(evaluation.ProductResourceToolCallInspection),
			ID:   input.InspectionID,
		},
		Payload: payload,
	}
}
