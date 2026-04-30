package opsreadiness

import "time"

type SmokeReportStatus string

const (
	SmokeReportDraft     SmokeReportStatus = "draft"
	SmokeReportRunning   SmokeReportStatus = "running"
	SmokeReportCompleted SmokeReportStatus = "completed"
	SmokeReportBlocked   SmokeReportStatus = "blocked"
	SmokeReportFailed    SmokeReportStatus = "failed"
	SmokeReportPublished SmokeReportStatus = "published"
)

type SmokeProbeResult string

const (
	SmokeProbePassed  SmokeProbeResult = "passed"
	SmokeProbeFailed  SmokeProbeResult = "failed"
	SmokeProbeBlocked SmokeProbeResult = "blocked"
	SmokeProbeSkipped SmokeProbeResult = "skipped"
)

type SmokeBlockedReason string

const (
	SmokeReasonMissingSafeCredentials     SmokeBlockedReason = "missing_safe_credentials"
	SmokeReasonUnsafeSideEffectScope      SmokeBlockedReason = "unsafe_side_effect_scope"
	SmokeReasonTenantApprovalUnavailable  SmokeBlockedReason = "tenant_approval_unavailable"
	SmokeReasonProviderOutage             SmokeBlockedReason = "provider_outage"
	SmokeReasonUnsupportedDomain          SmokeBlockedReason = "unsupported_domain"
	SmokeReasonOperatorDeferred           SmokeBlockedReason = "operator_deferred"
	SmokeReasonMissingTenantAdminApproval SmokeBlockedReason = "missing_tenant_admin_approval"
	SmokeReasonMissingOperatorApproval    SmokeBlockedReason = "missing_operator_approval"
	SmokeReasonRedactionFailedClosed      SmokeBlockedReason = "redaction_failed_closed"
)

type SmokeMatrixReport struct {
	SmokeReportID      string              `json:"smokeReportId"`
	TenantID           string              `json:"tenantId"`
	ReportKind         string              `json:"reportKind"`
	RequestedBy        string              `json:"requestedBy"`
	Status             SmokeReportStatus   `json:"status"`
	DomainSummary      map[string]string   `json:"domainSummary"`
	StartedAt          time.Time           `json:"startedAt"`
	CompletedAt        *time.Time          `json:"completedAt,omitempty"`
	PublishedAt        *time.Time          `json:"publishedAt,omitempty"`
	ArtifactRefs       []string            `json:"artifactRefs"`
	RetentionExpiresAt time.Time           `json:"retentionExpiresAt"`
	ProbeOutcomes      []SmokeProbeOutcome `json:"probeOutcomes,omitempty"`
}

type SmokeProbeOutcome struct {
	ProbeOutcomeID         string             `json:"probeOutcomeId"`
	TenantID               string             `json:"tenantId"`
	SmokeReportID          string             `json:"smokeReportId"`
	IntegrationID          string             `json:"integrationId"`
	IntegrationAccountID   string             `json:"integrationAccountId,omitempty"`
	DomainKind             string             `json:"domainKind"`
	ProviderKind           string             `json:"providerKind"`
	ProbeAction            string             `json:"probeAction"`
	Result                 SmokeProbeResult   `json:"result"`
	ReasonCode             string             `json:"reasonCode"`
	RemediationHint        string             `json:"remediationHint"`
	RetrySafety            string             `json:"retrySafety"`
	BlockedOrSkippedReason SmokeBlockedReason `json:"blockedOrSkippedReason,omitempty"`
	ApprovalRefs           []string           `json:"approvalRefs,omitempty"`
	ArtifactRefs           []string           `json:"artifactRefs,omitempty"`
	CheckedAt              time.Time          `json:"checkedAt"`
	RedactionStatus        string             `json:"redactionStatus"`
	RetentionExpiresAt     time.Time          `json:"retentionExpiresAt"`
}
