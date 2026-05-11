package connectors

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type ManagementState string

const (
	ManagementStateReady          ManagementState = "ready"
	ManagementStateDisabled       ManagementState = "disabled"
	ManagementStateDegraded       ManagementState = "degraded"
	ManagementStateUnavailable    ManagementState = "unavailable"
	ManagementStateActionRequired ManagementState = "action-required"
)

type CapabilitySupport string

const (
	CapabilitySupported   CapabilitySupport = "supported"
	CapabilityLimited     CapabilitySupport = "limited"
	CapabilityUnsupported CapabilitySupport = "unsupported"
)

type DiagnosticFreshness string

const (
	DiagnosticFreshnessFresh DiagnosticFreshness = "fresh"
	DiagnosticFreshnessStale DiagnosticFreshness = "stale"
)

type ManagementActionKind string

const (
	ManagementActionRepair             ManagementActionKind = "repair"
	ManagementActionReconnect          ManagementActionKind = "reconnect"
	ManagementActionCredentialRotation ManagementActionKind = "credential-rotation"
	ManagementActionRouteRevalidate    ManagementActionKind = "route-revalidate"
	ManagementActionDiagnosticRerun    ManagementActionKind = "diagnostic-rerun"
	ManagementActionDisable            ManagementActionKind = "disable"
	ManagementActionReEnable           ManagementActionKind = "re-enable"
)

type ManagementTerminalState string

const (
	ManagementTerminalReady          ManagementTerminalState = "ready"
	ManagementTerminalDegraded       ManagementTerminalState = "degraded"
	ManagementTerminalUnavailable    ManagementTerminalState = "unavailable"
	ManagementTerminalDisabled       ManagementTerminalState = "disabled"
	ManagementTerminalCancelled      ManagementTerminalState = "cancelled"
	ManagementTerminalActionRequired ManagementTerminalState = "action-required"
)

type RouteDecisionOutcome string

const (
	RouteDecisionAccepted    RouteDecisionOutcome = "accepted"
	RouteDecisionIgnored     RouteDecisionOutcome = "ignored"
	RouteDecisionBlocked     RouteDecisionOutcome = "blocked"
	RouteDecisionDuplicate   RouteDecisionOutcome = "duplicate"
	RouteDecisionUnsupported RouteDecisionOutcome = "unsupported"
	RouteDecisionFailed      RouteDecisionOutcome = "failed"
	RouteDecisionDisabled    RouteDecisionOutcome = "disabled"
)

type ChannelManagementPage struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"nextCursor,omitempty"`
	Order      string `json:"order"`
}

type ChannelConnectorListResponse struct {
	TenantID string                       `json:"tenantId,omitempty"`
	Page     ChannelManagementPage        `json:"page"`
	Items    []ChannelConnectorProjection `json:"items"`
}

type ChannelConnectorProjection struct {
	ConnectorID         string                       `json:"connectorId"`
	ConnectorKind       string                       `json:"connectorKind"`
	DisplayName         string                       `json:"displayName"`
	EnablementState     ManagementState              `json:"enablementState"`
	SetupState          string                       `json:"setupState"`
	HealthStatus        string                       `json:"healthStatus"`
	DiagnosticFreshness DiagnosticFreshness          `json:"diagnosticFreshness"`
	DeliveryEligible    bool                         `json:"deliveryEligible"`
	NextAction          *ChannelManagementNextAction `json:"nextAction,omitempty"`
	Capabilities        map[string]CapabilitySupport `json:"capabilities"`
	RedactionStatus     RedactionStatus              `json:"redactionStatus"`
	UpdatedAt           time.Time                    `json:"updatedAt"`
}

type ChannelConnectorDetail struct {
	ChannelConnectorProjection
	DiagnosticSummary        *ConnectorDiagnosticState   `json:"diagnosticSummary,omitempty"`
	RoutePolicy              *RoutePolicy                `json:"routePolicy,omitempty"`
	RecentRouteDecisions     []RoutingDecision           `json:"recentRouteDecisions,omitempty"`
	ForegroundReplyOutcomes  []ForegroundReplyOutcome    `json:"foregroundReplyOutcomes,omitempty"`
	BackgroundDelivery       []BackgroundDeliveryOutcome `json:"backgroundDeliveryOutcomes,omitempty"`
	RepairActions            []RepairAction              `json:"repairActions,omitempty"`
	SupportEvidenceAvailable bool                        `json:"supportEvidenceAvailable"`
	Retention                map[string]string           `json:"retention,omitempty"`
	SupportEvidence          *SupportEvidenceBundle      `json:"supportEvidence,omitempty"`
}

type ChannelManagementNextAction struct {
	ActionKind       ManagementActionKind `json:"actionKind"`
	Label            string               `json:"label"`
	ReasonCode       DiagnosticReasonCode `json:"reasonCode,omitempty"`
	RemediationOwner RemediationOwner     `json:"remediationOwner,omitempty"`
}

type EnablementState struct {
	TenantID             string     `json:"tenantId"`
	ConnectorID          string     `json:"connectorId"`
	State                string     `json:"state"`
	ReasonCode           string     `json:"reasonCode,omitempty"`
	ChangedByPrincipalID string     `json:"changedByPrincipalId,omitempty"`
	ChangedAt            time.Time  `json:"changedAt"`
	ValidatedAt          *time.Time `json:"validatedAt,omitempty"`
	AuditEventID         string     `json:"auditEventId"`
}

type EnablementMutationResult struct {
	ConnectorID      string          `json:"connectorId"`
	EnablementState  ManagementState `json:"enablementState"`
	DeliveryEligible bool            `json:"deliveryEligible"`
	AuditEventID     string          `json:"auditEventId"`
	ChangedAt        time.Time       `json:"changedAt"`
}

type RepairAction struct {
	RepairActionID          string                  `json:"repairActionId"`
	TenantID                string                  `json:"tenantId"`
	ConnectorID             string                  `json:"connectorId"`
	ConnectorKind           string                  `json:"connectorKind"`
	ActorPrincipalID        string                  `json:"actorPrincipalId,omitempty"`
	ActionKind              ManagementActionKind    `json:"actionKind"`
	SourceDiagnosticStateID string                  `json:"sourceDiagnosticStateId,omitempty"`
	SetupSessionID          string                  `json:"setupSessionId,omitempty"`
	Status                  ManagementTerminalState `json:"status"`
	RetrySafety             RetrySafety             `json:"retrySafety,omitempty"`
	RemediationOwner        RemediationOwner        `json:"remediationOwner,omitempty"`
	StartedAt               time.Time               `json:"startedAt"`
	CompletedAt             *time.Time              `json:"completedAt,omitempty"`
	AuditEventID            string                  `json:"auditEventId"`
	RedactionStatus         RedactionStatus         `json:"redactionStatus"`
}

type RoutePolicy struct {
	RoutePolicyID              string          `json:"routePolicyId"`
	TenantID                   string          `json:"tenantId"`
	ConnectorID                string          `json:"connectorId"`
	EligibleSenders            []string        `json:"eligibleSenders,omitempty"`
	EligibleConversations      []string        `json:"eligibleConversations,omitempty"`
	EligibleRooms              []string        `json:"eligibleRooms,omitempty"`
	EligibleChannels           []string        `json:"eligibleChannels,omitempty"`
	InvocationGates            []string        `json:"invocationGates,omitempty"`
	BackgroundDeliveryEligible bool            `json:"backgroundDeliveryEligible"`
	ValidationState            string          `json:"validationState"`
	ReasonCode                 string          `json:"reasonCode,omitempty"`
	ValidatedAt                time.Time       `json:"validatedAt"`
	AuditEventID               string          `json:"auditEventId,omitempty"`
	RedactionStatus            RedactionStatus `json:"redactionStatus"`
}

type RoutingDecision struct {
	RoutingDecisionID  string               `json:"routingDecisionId"`
	TenantID           string               `json:"tenantId"`
	ConnectorID        string               `json:"connectorId"`
	ConnectorKind      string               `json:"connectorKind"`
	Outcome            RouteDecisionOutcome `json:"outcome"`
	ReasonCode         string               `json:"reasonCode,omitempty"`
	OccurredAt         time.Time            `json:"occurredAt"`
	SafeEvidence       map[string]string    `json:"safeEvidence,omitempty"`
	RedactionStatus    RedactionStatus      `json:"redactionStatus"`
	RetentionExpiresAt time.Time            `json:"retentionExpiresAt"`
}

type ForegroundReplyOutcome struct {
	ReplyOutcomeID     string            `json:"replyOutcomeId"`
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	RoutingDecisionID  string            `json:"routingDecisionId,omitempty"`
	Status             string            `json:"status"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	OccurredAt         time.Time         `json:"occurredAt"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
	RedactionStatus    RedactionStatus   `json:"redactionStatus"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
}

type BackgroundDeliveryOutcome struct {
	DeliveryOutcomeID  string            `json:"deliveryOutcomeId"`
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	DeliveryTargetID   string            `json:"deliveryTargetId,omitempty"`
	Status             string            `json:"status"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	OccurredAt         time.Time         `json:"occurredAt"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
	RedactionStatus    RedactionStatus   `json:"redactionStatus"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
}

type SupportEvidenceBundle struct {
	SupportEvidenceID      string            `json:"supportEvidenceId"`
	TenantID               string            `json:"tenantId"`
	ConnectorID            string            `json:"connectorId"`
	GeneratedByPrincipalID string            `json:"generatedByPrincipalId,omitempty"`
	GeneratedAt            time.Time         `json:"generatedAt"`
	CurrentState           ManagementState   `json:"currentState"`
	StateTransitions       []string          `json:"stateTransitions,omitempty"`
	DiagnosticRefs         []string          `json:"diagnosticRefs,omitempty"`
	RepairRefs             []string          `json:"repairRefs,omitempty"`
	RoutingDecisionRefs    []string          `json:"routingDecisionRefs,omitempty"`
	ReplyOutcomeRefs       []string          `json:"replyOutcomeRefs,omitempty"`
	DeliveryOutcomeRefs    []string          `json:"deliveryOutcomeRefs,omitempty"`
	AuditRefs              []string          `json:"auditRefs,omitempty"`
	Redactions             []string          `json:"redactions,omitempty"`
	RetentionExpiresAt     time.Time         `json:"retentionExpiresAt"`
	RedactionStatus        RedactionStatus   `json:"redactionStatus"`
	SafeEvidence           map[string]string `json:"safeEvidence,omitempty"`
}

type ConnectorAuditRecord struct {
	AuditEventID    string          `json:"auditEventId"`
	TenantID        string          `json:"tenantId"`
	ConnectorID     string          `json:"connectorId"`
	PrincipalID     string          `json:"principalId,omitempty"`
	Action          string          `json:"action"`
	PermissionGate  string          `json:"permissionGate"`
	Outcome         string          `json:"outcome"`
	ReasonCode      string          `json:"reasonCode,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	RedactionStatus RedactionStatus `json:"redactionStatus"`
}

type ProjectionInput struct {
	TenantID    string
	Connectors  []Connector
	Diagnostics map[string][]ConnectorDiagnosticState
	Now         time.Time
	Limit       int
	Cursor      string
	StateFilter string
	KindFilter  string
}

func BuildConnectorPage(input ProjectionInput) ChannelConnectorListResponse {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	items := make([]ChannelConnectorProjection, 0, len(input.Connectors))
	for _, connector := range input.Connectors {
		if input.TenantID != "" && connector.TenantID != "" && connector.TenantID != input.TenantID {
			continue
		}
		if input.KindFilter != "" && connector.Kind != input.KindFilter {
			continue
		}
		projection := BuildConnectorProjection(connector, latestDiagnostic(input.Diagnostics[connector.ConnectorID]), now)
		if input.StateFilter != "" && string(projection.EnablementState) != input.StateFilter {
			continue
		}
		items = append(items, projection)
	}
	SortConnectorProjections(items)
	offset := parseCursorOffset(input.Cursor)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	nextCursor := ""
	if end < len(items) {
		nextCursor = strconv.Itoa(end)
	} else {
		end = len(items)
	}
	return ChannelConnectorListResponse{
		TenantID: input.TenantID,
		Page: ChannelManagementPage{
			Limit:      limit,
			NextCursor: nextCursor,
			Order:      "attention_disabled_ready_name_id",
		},
		Items: items[offset:end],
	}
}

func BuildConnectorProjection(connector Connector, diagnostic *ConnectorDiagnosticState, now time.Time) ChannelConnectorProjection {
	state := ManagementStateForConnector(connector, diagnostic)
	freshness := DiagnosticFreshnessFresh
	if diagnostic == nil || FreshnessAt(diagnostic.EvidenceTimestamp, now) == FreshnessStale {
		freshness = DiagnosticFreshnessStale
	}
	deliveryEligible := state == ManagementStateReady
	projection := ChannelConnectorProjection{
		ConnectorID:         connector.ConnectorID,
		ConnectorKind:       connector.Kind,
		DisplayName:         connector.DisplayName,
		EnablementState:     state,
		SetupState:          setupStateForManagement(state),
		HealthStatus:        healthStatusForManagement(connector, diagnostic),
		DiagnosticFreshness: freshness,
		DeliveryEligible:    deliveryEligible,
		Capabilities:        CapabilityProfileForKind(connector.Kind),
		RedactionStatus:     RedactionStatusRedacted,
		UpdatedAt:           connector.UpdatedAt,
	}
	if diagnostic != nil && state != ManagementStateReady {
		projection.NextAction = &ChannelManagementNextAction{
			ActionKind:       nextActionForDiagnostic(*diagnostic),
			Label:            nextActionLabelForDiagnostic(*diagnostic),
			ReasonCode:       diagnostic.ReasonCode,
			RemediationOwner: diagnostic.RemediationOwner,
		}
	} else if state == ManagementStateDisabled {
		projection.NextAction = &ChannelManagementNextAction{ActionKind: ManagementActionReEnable, Label: "Re-enable connector"}
	}
	return projection
}

func ManagementStateForConnector(connector Connector, diagnostic *ConnectorDiagnosticState) ManagementState {
	if connector.Status == StatusDisabled {
		return ManagementStateDisabled
	}
	if diagnostic != nil {
		switch diagnostic.Status {
		case LifecycleStatePermissionBlocked, LifecycleStateFailed:
			return ManagementStateActionRequired
		case LifecycleStateRateLimited:
			return ManagementStateUnavailable
		case LifecycleStateDegraded:
			return ManagementStateDegraded
		case LifecycleStateUnsupportedCapability:
			return ManagementStateDegraded
		}
	}
	switch connector.Status {
	case StatusHealthy:
		return ManagementStateReady
	case StatusRegistered:
		return ManagementStateReady
	case StatusDegraded, StatusBackingOff:
		return ManagementStateDegraded
	case StatusFailed:
		return ManagementStateActionRequired
	case StatusDisabled:
		return ManagementStateDisabled
	default:
		return ManagementStateUnavailable
	}
}

func CapabilityProfileForKind(kind string) map[string]CapabilitySupport {
	capabilities := map[string]CapabilitySupport{
		"disable":                    CapabilitySupported,
		"re-enable":                  CapabilitySupported,
		"repair":                     CapabilitySupported,
		"reconnect":                  CapabilitySupported,
		"credential-rotation":        CapabilityLimited,
		"route-edit":                 CapabilitySupported,
		"foreground-reply-status":    CapabilitySupported,
		"background-delivery-status": CapabilitySupported,
		"support-evidence":           CapabilitySupported,
	}
	switch strings.ToLower(kind) {
	case "discord", "telegram", "slack", "matrix":
		return capabilities
	default:
		capabilities["reconnect"] = CapabilityUnsupported
		capabilities["credential-rotation"] = CapabilityUnsupported
		capabilities["route-edit"] = CapabilityUnsupported
		return capabilities
	}
}

func SortConnectorProjections(items []ChannelConnectorProjection) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if rank := managementStateRank(left.EnablementState) - managementStateRank(right.EnablementState); rank != 0 {
			return rank < 0
		}
		if left.DisplayName != right.DisplayName {
			return left.DisplayName < right.DisplayName
		}
		return left.ConnectorID < right.ConnectorID
	})
}

func latestDiagnostic(items []ConnectorDiagnosticState) *ConnectorDiagnosticState {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.EvidenceTimestamp.After(latest.EvidenceTimestamp) {
			latest = item
		}
	}
	return &latest
}

func managementStateRank(state ManagementState) int {
	switch state {
	case ManagementStateActionRequired, ManagementStateUnavailable, ManagementStateDegraded:
		return 0
	case ManagementStateDisabled:
		return 1
	case ManagementStateReady:
		return 2
	default:
		return 3
	}
}

func setupStateForManagement(state ManagementState) string {
	switch state {
	case ManagementStateReady:
		return "ready"
	case ManagementStateDisabled:
		return "disabled"
	default:
		return string(state)
	}
}

func healthStatusForManagement(connector Connector, diagnostic *ConnectorDiagnosticState) string {
	if diagnostic != nil {
		return string(diagnostic.Status)
	}
	return string(connector.Status)
}

func nextActionForDiagnostic(diagnostic ConnectorDiagnosticState) ManagementActionKind {
	switch diagnostic.ReasonCode {
	case DiagnosticAuthMissing, DiagnosticPermissionMissing:
		return ManagementActionReconnect
	case DiagnosticBlockedRoute:
		return ManagementActionRouteRevalidate
	case DiagnosticUnsupportedCapability:
		return ManagementActionDisable
	default:
		return ManagementActionRepair
	}
}

func nextActionLabelForDiagnostic(diagnostic ConnectorDiagnosticState) string {
	switch nextActionForDiagnostic(diagnostic) {
	case ManagementActionReconnect:
		return "Reconnect authorization"
	case ManagementActionRouteRevalidate:
		return "Review route policy"
	case ManagementActionDisable:
		return "Disable unsupported connector"
	default:
		return "Repair connector"
	}
}

func parseCursorOffset(cursor string) int {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}
