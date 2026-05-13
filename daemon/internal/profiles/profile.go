package profiles

import "time"

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	StatusDisabled Status = "disabled"
)

type ChangeKind string

const (
	ChangeCreated    ChangeKind = "created"
	ChangeUpdated    ChangeKind = "updated"
	ChangeActivated  ChangeKind = "activated"
	ChangeRolledBack ChangeKind = "rolled_back"
	ChangeArchived   ChangeKind = "archived"
	ChangeDisabled   ChangeKind = "disabled"
	ChangeValidated  ChangeKind = "validated"
)

type RollbackEligibility string

const (
	RollbackEligible        RollbackEligibility = "eligible"
	RollbackInvalidProvider RollbackEligibility = "invalid_provider"
	RollbackInvalidOverlay  RollbackEligibility = "invalid_overlay"
	RollbackPolicyBlocked   RollbackEligibility = "policy_blocked"
	RollbackProfileArchived RollbackEligibility = "profile_archived"
	RollbackProfileDisabled RollbackEligibility = "profile_disabled"
	RollbackRedactionFailed RollbackEligibility = "redaction_failed"
)

type RedactionStatus string

const (
	RedactionRedacted   RedactionStatus = "redacted"
	RedactionSuppressed RedactionStatus = "suppressed"
	RedactionFailed     RedactionStatus = "redaction_failed"
)

type OverlayValidationState string

const (
	OverlayValid            OverlayValidationState = "valid"
	OverlayPartial          OverlayValidationState = "partial"
	OverlayMissing          OverlayValidationState = "missing"
	OverlayPermissionDenied OverlayValidationState = "permission_denied"
	OverlayOutOfScope       OverlayValidationState = "out_of_scope"
	OverlayTooLarge         OverlayValidationState = "too_large"
	OverlayUnsafeContent    OverlayValidationState = "unsafe_content"
	OverlayRedactionFailed  OverlayValidationState = "redaction_failed"
)

type RuntimeResourceKind string

const (
	RuntimeResourceThread             RuntimeResourceKind = "thread"
	RuntimeResourceSession            RuntimeResourceKind = "session"
	RuntimeResourceRun                RuntimeResourceKind = "run"
	RuntimeResourceWorkflow           RuntimeResourceKind = "workflow"
	RuntimeResourceHandoffDestination RuntimeResourceKind = "handoff_destination"
)

type SelectionReason string

const (
	SelectionDefaultSeeded     SelectionReason = "default_seeded"
	SelectionUserActivated     SelectionReason = "user_activated"
	SelectionRollbackActivated SelectionReason = "rollback_activated"
	SelectionSystemFallback    SelectionReason = "system_fallback"
)

const SelectionScopeTenantDefault = "tenant_default"

type DisplayIdentity struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	SafeSummary string `json:"safeSummary,omitempty"`
}

type Persona struct {
	Tone         string `json:"tone,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	SafeSummary  string `json:"safeSummary,omitempty"`
}

type DefaultProviderPreference struct {
	ProviderID        string                 `json:"providerId,omitempty"`
	Model             string                 `json:"model,omitempty"`
	ReasoningLevel    string                 `json:"reasoningLevel,omitempty"`
	ValidationState   OverlayValidationState `json:"validationState,omitempty"`
	FailureReasonCode string                 `json:"failureReasonCode,omitempty"`
}

type SafetyDefaults struct {
	ApprovalPosture   string                 `json:"approvalPosture,omitempty"`
	RiskTolerance     string                 `json:"riskTolerance,omitempty"`
	ValidationState   OverlayValidationState `json:"validationState,omitempty"`
	FailureReasonCode string                 `json:"failureReasonCode,omitempty"`
}

type OverlayReference struct {
	OverlayReferenceID string                 `json:"overlayReferenceId"`
	ProfileID          string                 `json:"profileId"`
	ProfileVersionID   string                 `json:"profileVersionId,omitempty"`
	TenantID           string                 `json:"tenantId"`
	ReferenceKind      string                 `json:"referenceKind"`
	Scope              string                 `json:"scope"`
	ReferenceURI       string                 `json:"referenceUri"`
	SafeDisplayLabel   string                 `json:"safeDisplayLabel"`
	ValidationState    OverlayValidationState `json:"validationState"`
	FailureReasonCode  string                 `json:"failureReasonCode,omitempty"`
	LastValidatedAt    *time.Time             `json:"lastValidatedAt,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
	RedactionStatus    RedactionStatus        `json:"redactionStatus"`
}

type AgentProfile struct {
	ProfileID                 string                    `json:"profileId"`
	TenantID                  string                    `json:"tenantId"`
	DisplayName               string                    `json:"displayName"`
	DisplayIdentity           DisplayIdentity           `json:"displayIdentity"`
	Persona                   Persona                   `json:"persona"`
	DefaultProviderPreference DefaultProviderPreference `json:"defaultProviderPreference"`
	SafetyDefaults            SafetyDefaults            `json:"safetyDefaults"`
	LegacyMappingEvidence     []LegacyMappingEvidence   `json:"legacyMappingEvidence,omitempty"`
	Status                    Status                    `json:"status"`
	ActiveVersionID           string                    `json:"activeVersionId,omitempty"`
	TenantDefault             bool                      `json:"tenantDefault"`
	OverlayReferenceCount     int                       `json:"overlayReferenceCount"`
	CreatedAt                 time.Time                 `json:"createdAt"`
	UpdatedAt                 time.Time                 `json:"updatedAt"`
	ArchivedAt                *time.Time                `json:"archivedAt,omitempty"`
	DisabledAt                *time.Time                `json:"disabledAt,omitempty"`
	CreatedByPrincipalID      string                    `json:"createdByPrincipalId,omitempty"`
	UpdatedByPrincipalID      string                    `json:"updatedByPrincipalId,omitempty"`
	RedactionStatus           RedactionStatus           `json:"redactionStatus"`
}

type LegacyMappingEvidence struct {
	SourceKind      string                 `json:"sourceKind"`
	MappingState    OverlayValidationState `json:"mappingState"`
	ReasonCode      string                 `json:"reasonCode"`
	SafeSummary     string                 `json:"safeSummary,omitempty"`
	RedactionStatus RedactionStatus        `json:"redactionStatus"`
}

type ProfileVersion struct {
	ProfileVersionID    string              `json:"profileVersionId"`
	ProfileID           string              `json:"profileId"`
	TenantID            string              `json:"tenantId"`
	VersionNumber       int                 `json:"versionNumber"`
	SourceVersionID     string              `json:"sourceVersionId,omitempty"`
	ChangeKind          ChangeKind          `json:"changeKind"`
	ChangeSummary       string              `json:"changeSummary"`
	Snapshot            AgentProfile        `json:"snapshot"`
	RollbackEligibility RollbackEligibility `json:"rollbackEligibility"`
	ActorPrincipalID    string              `json:"actorPrincipalId,omitempty"`
	CreatedAt           time.Time           `json:"createdAt"`
	AuditEventID        string              `json:"auditEventId,omitempty"`
	RedactionStatus     RedactionStatus     `json:"redactionStatus"`
}

type ActiveSelection struct {
	SelectionID           string          `json:"selectionId"`
	TenantID              string          `json:"tenantId"`
	ProfileID             string          `json:"profileId"`
	ProfileVersionID      string          `json:"profileVersionId"`
	SelectionScope        string          `json:"selectionScope"`
	SelectionReason       SelectionReason `json:"selectionReason"`
	SelectedByPrincipalID string          `json:"selectedByPrincipalId,omitempty"`
	SelectedAt            time.Time       `json:"selectedAt"`
	AuditEventID          string          `json:"auditEventId,omitempty"`
	RedactionStatus       RedactionStatus `json:"redactionStatus"`
}

type RuntimeProjection struct {
	RuntimeProfileProjectionID    string              `json:"runtimeProfileProjectionId"`
	TenantID                      string              `json:"tenantId"`
	ProfileID                     string              `json:"profileId"`
	ProfileVersionID              string              `json:"profileVersionId"`
	SelectionID                   string              `json:"selectionId"`
	ResourceKind                  RuntimeResourceKind `json:"resourceKind"`
	ResourceID                    string              `json:"resourceId"`
	ThreadID                      string              `json:"threadId,omitempty"`
	SessionSegmentID              string              `json:"sessionSegmentId,omitempty"`
	RunID                         string              `json:"runId,omitempty"`
	WorkflowID                    string              `json:"workflowId,omitempty"`
	HandoffLinkID                 string              `json:"handoffLinkId,omitempty"`
	SelectionScope                string              `json:"selectionScope"`
	SelectionReason               SelectionReason     `json:"selectionReason"`
	SafeDisplayName               string              `json:"safeDisplayName"`
	SafeSummary                   string              `json:"safeSummary"`
	ConfigurationScope            string              `json:"configurationScope,omitempty"`
	DeferredBindingClassification string              `json:"deferredBindingClassification,omitempty"`
	OccurredAt                    time.Time           `json:"occurredAt"`
	RetentionExpiresAt            *time.Time          `json:"retentionExpiresAt,omitempty"`
	RedactionStatus               RedactionStatus     `json:"redactionStatus"`
}

type AuditEvent struct {
	AuditEventID     string          `json:"auditEventId"`
	TenantID         string          `json:"tenantId"`
	ProfileID        string          `json:"profileId,omitempty"`
	ProfileVersionID string          `json:"profileVersionId,omitempty"`
	ActorPrincipalID string          `json:"actorPrincipalId,omitempty"`
	EventKind        string          `json:"eventKind"`
	Outcome          string          `json:"outcome"`
	PermissionGate   string          `json:"permissionGate,omitempty"`
	ReasonCode       string          `json:"reasonCode"`
	SafeSummary      string          `json:"safeSummary,omitempty"`
	OccurredAt       time.Time       `json:"occurredAt"`
	RedactionStatus  RedactionStatus `json:"redactionStatus"`
}

type ProfileDetail struct {
	Profile           AgentProfile       `json:"profile"`
	Versions          []ProfileVersion   `json:"versions"`
	OverlayReferences []OverlayReference `json:"overlayReferences"`
	AuditEvents       []AuditEvent       `json:"auditEvents"`
}

type ListResponse struct {
	TenantID string         `json:"tenantId"`
	Page     Page           `json:"page"`
	Items    []AgentProfile `json:"items"`
}

type Page struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"nextCursor"`
	Order      string `json:"order"`
}

type MutationInput struct {
	DisplayName               string                    `json:"displayName"`
	DisplayIdentity           DisplayIdentity           `json:"displayIdentity"`
	Persona                   Persona                   `json:"persona"`
	DefaultProviderPreference DefaultProviderPreference `json:"defaultProviderPreference"`
	SafetyDefaults            SafetyDefaults            `json:"safetyDefaults"`
	LegacyMappingEvidence     []LegacyMappingEvidence   `json:"legacyMappingEvidence,omitempty"`
	OverlayReferences         []OverlayReferenceInput   `json:"overlayReferences"`
	Activate                  bool                      `json:"activate"`
	ReasonCode                string                    `json:"reasonCode"`
}

type OverlayReferenceInput struct {
	ReferenceKind string `json:"referenceKind"`
	ReferenceURI  string `json:"referenceUri"`
	Scope         string `json:"scope"`
}

type ActivationInput struct {
	ProfileVersionID string `json:"profileVersionId"`
	ReasonCode       string `json:"reasonCode"`
}

type RollbackInput struct {
	SourceProfileVersionID string `json:"sourceProfileVersionId"`
	ReasonCode             string `json:"reasonCode"`
}

type RetirementInput struct {
	ReasonCode string `json:"reasonCode"`
}

type MutationResult struct {
	Profile      AgentProfile    `json:"profile"`
	Version      ProfileVersion  `json:"version"`
	Selection    ActiveSelection `json:"selection,omitempty"`
	AuditEventID string          `json:"auditEventId"`
}
