package connectors

import (
	"errors"
	"sort"
	"strings"
	"time"
)

type LifecycleState string

const (
	LifecycleStateConfigured            LifecycleState = "configured"
	LifecycleStateDisabled              LifecycleState = "disabled"
	LifecycleStateStarting              LifecycleState = "starting"
	LifecycleStateHealthy               LifecycleState = "healthy"
	LifecycleStateDegraded              LifecycleState = "degraded"
	LifecycleStateFailed                LifecycleState = "failed"
	LifecycleStatePermissionBlocked     LifecycleState = "permission_blocked"
	LifecycleStateRateLimited           LifecycleState = "rate_limited"
	LifecycleStateUnsupportedCapability LifecycleState = "unsupported_capability"
)

type ConformanceArea string

const (
	ConformanceAreaTenantOwnership            ConformanceArea = "tenant_ownership"
	ConformanceAreaPermissionGating           ConformanceArea = "permission_gating"
	ConformanceAreaRedaction                  ConformanceArea = "redaction"
	ConformanceAreaActiveTenantAccountBinding ConformanceArea = "active_tenant_account_binding"
	ConformanceAreaInboundIdentity            ConformanceArea = "inbound_identity"
	ConformanceAreaDurableDedupe              ConformanceArea = "durable_dedupe"
	ConformanceAreaStableRouting              ConformanceArea = "stable_routing_decisions"
	ConformanceAreaMinimumForegroundReply     ConformanceArea = "minimum_foreground_reply"
	ConformanceAreaDiagnostics                ConformanceArea = "required_diagnostics"
	ConformanceAreaDeliverySeparation         ConformanceArea = "delivery_separation"
)

type SurfaceSupport string

const (
	SurfaceSupported   SurfaceSupport = "supported"
	SurfaceLimited     SurfaceSupport = "limited"
	SurfaceUnsupported SurfaceSupport = "unsupported"
)

const (
	ConnectorKindMatrix = "matrix"

	MatrixDurableIdentityRuleID = "matrix_homeserver_conversation_event_id"
	MatrixDurableIdentityRule   = "tenant_id + connector_id + homeserver_id + conversation_id + matrix_event_id"

	MatrixSurfaceTenantProvidedBotSetup   = "tenant_provided_bot_setup"
	MatrixSurfaceHostedHomeserver         = "dopeagent_hosted_homeserver"
	MatrixSurfaceAccountProvisioning      = "matrix_account_provisioning"
	MatrixSurfaceDirectMessage            = "direct_message"
	MatrixSurfaceAllowedRoomMention       = "allowed_room_mention"
	MatrixSurfaceAllowedRoomCommand       = "allowed_room_command"
	MatrixSurfaceUnencryptedText          = "unencrypted_text"
	MatrixSurfaceEncryptedRooms           = "encrypted_rooms"
	MatrixSurfaceUndecryptableEvents      = "undecryptable_events"
	MatrixSurfaceFinalOnlyForegroundReply = "final_only_foreground_reply"
	MatrixSurfaceConnectorBackedDelivery  = "connector_backed_delivery"

	GroupRoomSurfaceMentionEvidence           = "group_room_mention_evidence"
	GroupRoomSurfaceAllowlistEvidence         = "group_room_allowlist_evidence"
	GroupRoomSurfaceUnsupportedSourceEvidence = "group_room_unsupported_source_evidence"
	GroupRoomSurfaceDuplicateMessageEvidence  = "group_room_duplicate_message_evidence"
	GroupRoomSurfaceEditedMessageEvidence     = "group_room_edited_message_evidence"
	GroupRoomSurfaceDeletedMessageEvidence    = "group_room_deleted_message_evidence"

	HandoffSurfaceSourceSupport                 = "handoff_source_support"
	HandoffSurfaceDestinationSupport            = "handoff_destination_support"
	HandoffSurfaceFirstResponseSourceReferences = "handoff_first_response_source_references"
)

type ConformanceResultStatus string

const (
	ConformanceResultPass        ConformanceResultStatus = "pass"
	ConformanceResultFail        ConformanceResultStatus = "fail"
	ConformanceResultSupported   ConformanceResultStatus = "supported"
	ConformanceResultLimited     ConformanceResultStatus = "limited"
	ConformanceResultUnsupported ConformanceResultStatus = "unsupported"
)

type RedactionStatus string

const (
	RedactionStatusRedacted   RedactionStatus = "redacted"
	RedactionStatusSuppressed RedactionStatus = "suppressed"
	RedactionStatusFailed     RedactionStatus = "redaction_failed"
)

type AccountBindingSummary struct {
	TenantID             string          `json:"tenantId,omitempty"`
	ConnectorID          string          `json:"connectorId,omitempty"`
	ConnectorAccountID   string          `json:"connectorAccountId"`
	ProviderAccountLabel string          `json:"providerAccountLabel,omitempty"`
	PermissionState      string          `json:"permissionState"`
	RedactionStatus      RedactionStatus `json:"redactionStatus"`
}

type CapabilityProfile struct {
	ProfileID                       string                                      `json:"profileId"`
	TenantID                        string                                      `json:"tenantId,omitempty"`
	ConnectorID                     string                                      `json:"connectorId"`
	ConnectorKind                   string                                      `json:"connectorKind"`
	CoreInvariantResults            map[ConformanceArea]ConformanceResultStatus `json:"coreInvariantResults"`
	ProviderSurfaceResults          map[string]SurfaceSupport                   `json:"providerSurfaceResults,omitempty"`
	GroupRoomCapabilities           GroupRoomCapabilities                       `json:"groupRoomCapabilities,omitempty"`
	HandoffCapabilities             HandoffCapabilities                         `json:"handoffCapabilities,omitempty"`
	EquivalentDurableIdentityRuleID string                                      `json:"equivalentDurableIdentityRuleId,omitempty"`
	EquivalentDurableIdentityRule   string                                      `json:"equivalentDurableIdentityRule,omitempty"`
	DeclaredAt                      time.Time                                   `json:"declaredAt"`
}

type ConformanceResult struct {
	ConformanceResultID string                  `json:"conformanceResultId"`
	TenantID            string                  `json:"tenantId,omitempty"`
	ConnectorKind       string                  `json:"connectorKind"`
	ConnectorID         string                  `json:"connectorId,omitempty"`
	ScenarioID          string                  `json:"scenarioId"`
	Area                string                  `json:"area"`
	Result              ConformanceResultStatus `json:"result"`
	ReasonCode          string                  `json:"reasonCode,omitempty"`
	RedactionStatus     RedactionStatus         `json:"redactionStatus"`
	EvidenceTimestamp   time.Time               `json:"evidenceTimestamp"`
	RetentionExpiresAt  time.Time               `json:"retentionExpiresAt"`
}

type MatrixCase struct {
	ScenarioID                      string
	ConnectorKind                   string
	ConnectorID                     string
	TenantID                        string
	CoreInvariantResults            map[ConformanceArea]ConformanceResultStatus
	ProviderSurfaceResults          map[string]SurfaceSupport
	GroupRoomCapabilities           GroupRoomCapabilities
	HandoffCapabilities             HandoffCapabilities
	EquivalentDurableIdentityRuleID string
	EquivalentDurableIdentityRule   string
	UnsafeIncrementalUpdateDegraded bool
	RedactionStatus                 RedactionStatus
	Now                             time.Time
}

type GroupRoomCapabilities struct {
	MentionEvidence           SurfaceSupport `json:"mentionEvidence,omitempty"`
	AllowlistEvidence         SurfaceSupport `json:"allowlistEvidence,omitempty"`
	UnsupportedSourceEvidence SurfaceSupport `json:"unsupportedSourceEvidence,omitempty"`
	DuplicateMessageEvidence  SurfaceSupport `json:"duplicateMessageEvidence,omitempty"`
	EditedMessageEvidence     SurfaceSupport `json:"editedMessageEvidence,omitempty"`
	DeletedMessageEvidence    SurfaceSupport `json:"deletedMessageEvidence,omitempty"`
}

type HandoffCapabilities struct {
	SourceSupport                 SurfaceSupport `json:"sourceSupport,omitempty"`
	DestinationSupport            SurfaceSupport `json:"destinationSupport,omitempty"`
	FirstResponseSourceReferences SurfaceSupport `json:"firstResponseSourceReferences,omitempty"`
}

func (capabilities GroupRoomCapabilities) surfaceResults() map[string]SurfaceSupport {
	results := map[string]SurfaceSupport{}
	if capabilities.MentionEvidence != "" {
		results[GroupRoomSurfaceMentionEvidence] = capabilities.MentionEvidence
	}
	if capabilities.AllowlistEvidence != "" {
		results[GroupRoomSurfaceAllowlistEvidence] = capabilities.AllowlistEvidence
	}
	if capabilities.UnsupportedSourceEvidence != "" {
		results[GroupRoomSurfaceUnsupportedSourceEvidence] = capabilities.UnsupportedSourceEvidence
	}
	if capabilities.DuplicateMessageEvidence != "" {
		results[GroupRoomSurfaceDuplicateMessageEvidence] = capabilities.DuplicateMessageEvidence
	}
	if capabilities.EditedMessageEvidence != "" {
		results[GroupRoomSurfaceEditedMessageEvidence] = capabilities.EditedMessageEvidence
	}
	if capabilities.DeletedMessageEvidence != "" {
		results[GroupRoomSurfaceDeletedMessageEvidence] = capabilities.DeletedMessageEvidence
	}
	return results
}

func (capabilities HandoffCapabilities) surfaceResults() map[string]SurfaceSupport {
	results := map[string]SurfaceSupport{}
	if capabilities.SourceSupport != "" {
		results[HandoffSurfaceSourceSupport] = capabilities.SourceSupport
	}
	if capabilities.DestinationSupport != "" {
		results[HandoffSurfaceDestinationSupport] = capabilities.DestinationSupport
	}
	if capabilities.FirstResponseSourceReferences != "" {
		results[HandoffSurfaceFirstResponseSourceReferences] = capabilities.FirstResponseSourceReferences
	}
	return results
}

var (
	ErrConformanceScenarioRequired = errors.New("conformance scenario id is required")
	ErrConformanceKindRequired     = errors.New("connector kind is required")
	ErrCoreInvariantFailed         = errors.New("core invariant failed")
	ErrEquivalentIdentityRequired  = errors.New("equivalent durable identity rule is required")
)

func CoreInvariantAreas() []ConformanceArea {
	return []ConformanceArea{
		ConformanceAreaTenantOwnership,
		ConformanceAreaPermissionGating,
		ConformanceAreaRedaction,
		ConformanceAreaActiveTenantAccountBinding,
		ConformanceAreaInboundIdentity,
		ConformanceAreaDurableDedupe,
		ConformanceAreaStableRouting,
		ConformanceAreaMinimumForegroundReply,
		ConformanceAreaDiagnostics,
		ConformanceAreaDeliverySeparation,
	}
}

func ValidateCapabilityProfile(profile CapabilityProfile) error {
	if strings.TrimSpace(profile.ConnectorID) == "" {
		return ErrConnectorIDRequired
	}
	if strings.TrimSpace(profile.ConnectorKind) == "" {
		return ErrConnectorKindRequired
	}
	for _, area := range CoreInvariantAreas() {
		if profile.CoreInvariantResults[area] != ConformanceResultPass {
			return ErrCoreInvariantFailed
		}
	}
	if strings.TrimSpace(profile.EquivalentDurableIdentityRuleID) != "" && strings.TrimSpace(profile.EquivalentDurableIdentityRule) == "" {
		return ErrEquivalentIdentityRequired
	}
	return nil
}

func RunMatrixCase(input MatrixCase) ([]ConformanceResult, CapabilityProfile, error) {
	if strings.TrimSpace(input.ScenarioID) == "" {
		return nil, CapabilityProfile{}, ErrConformanceScenarioRequired
	}
	if strings.TrimSpace(input.ConnectorKind) == "" {
		return nil, CapabilityProfile{}, ErrConformanceKindRequired
	}
	if strings.TrimSpace(input.EquivalentDurableIdentityRuleID) != "" && strings.TrimSpace(input.EquivalentDurableIdentityRule) == "" {
		return nil, CapabilityProfile{}, ErrEquivalentIdentityRequired
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	redactionStatus := input.RedactionStatus
	if redactionStatus == "" {
		redactionStatus = RedactionStatusRedacted
	}

	core := make(map[ConformanceArea]ConformanceResultStatus, len(input.CoreInvariantResults))
	for _, area := range CoreInvariantAreas() {
		result := input.CoreInvariantResults[area]
		if result == "" {
			result = ConformanceResultFail
		}
		core[area] = result
	}
	surfaces := map[string]SurfaceSupport{}
	for key, value := range input.ProviderSurfaceResults {
		surfaces[key] = value
	}
	for key, value := range input.GroupRoomCapabilities.surfaceResults() {
		surfaces[key] = value
	}
	for key, value := range input.HandoffCapabilities.surfaceResults() {
		surfaces[key] = value
	}
	if input.UnsafeIncrementalUpdateDegraded {
		surfaces["incremental_visible_updates"] = SurfaceLimited
	}

	results := make([]ConformanceResult, 0, len(core)+len(surfaces))
	for _, area := range CoreInvariantAreas() {
		results = append(results, ConformanceResult{
			ConformanceResultID: "conf_" + input.ScenarioID + "_" + string(area),
			TenantID:            input.TenantID,
			ConnectorKind:       input.ConnectorKind,
			ConnectorID:         input.ConnectorID,
			ScenarioID:          input.ScenarioID,
			Area:                string(area),
			Result:              core[area],
			ReasonCode:          coreReasonCode(core[area]),
			RedactionStatus:     redactionStatus,
			EvidenceTimestamp:   now,
			RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		})
	}
	surfaceKeys := make([]string, 0, len(surfaces))
	for key := range surfaces {
		surfaceKeys = append(surfaceKeys, key)
	}
	sort.Strings(surfaceKeys)
	for _, key := range surfaceKeys {
		results = append(results, ConformanceResult{
			ConformanceResultID: "conf_" + input.ScenarioID + "_" + key,
			TenantID:            input.TenantID,
			ConnectorKind:       input.ConnectorKind,
			ConnectorID:         input.ConnectorID,
			ScenarioID:          input.ScenarioID,
			Area:                key,
			Result:              surfaceResult(surfaces[key]),
			RedactionStatus:     redactionStatus,
			EvidenceTimestamp:   now,
			RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		})
	}

	profile := CapabilityProfile{
		ProfileID:                       "profile_" + input.ScenarioID,
		TenantID:                        input.TenantID,
		ConnectorID:                     input.ConnectorID,
		ConnectorKind:                   input.ConnectorKind,
		CoreInvariantResults:            core,
		ProviderSurfaceResults:          surfaces,
		GroupRoomCapabilities:           input.GroupRoomCapabilities,
		HandoffCapabilities:             input.HandoffCapabilities,
		EquivalentDurableIdentityRuleID: input.EquivalentDurableIdentityRuleID,
		EquivalentDurableIdentityRule:   input.EquivalentDurableIdentityRule,
		DeclaredAt:                      now,
	}
	return results, profile, nil
}

func surfaceResult(surface SurfaceSupport) ConformanceResultStatus {
	switch surface {
	case SurfaceSupported:
		return ConformanceResultSupported
	case SurfaceLimited:
		return ConformanceResultLimited
	default:
		return ConformanceResultUnsupported
	}
}

func coreReasonCode(result ConformanceResultStatus) string {
	if result == ConformanceResultFail {
		return "core_invariant_failed"
	}
	return ""
}
