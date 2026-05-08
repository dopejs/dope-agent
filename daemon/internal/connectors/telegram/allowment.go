package telegram

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type ScopeType string

const (
	ScopeUser       ScopeType = "user"
	ScopeDirectChat ScopeType = "direct_chat"
	ScopeGroup      ScopeType = "group"
)

type GroupGate string

const (
	GroupGateNotApplicable             GroupGate = "not_applicable"
	GroupGateMentionOrCommandRequired  GroupGate = "mention_or_command_required"
	GroupGateMentionOrCommandSupported GroupGate = GroupGateMentionOrCommandRequired
)

type AllowmentValidationState string

const (
	AllowmentValid             AllowmentValidationState = "valid"
	AllowmentInvalid           AllowmentValidationState = "invalid"
	AllowmentBlocked           AllowmentValidationState = "blocked"
	AllowmentStale             AllowmentValidationState = "stale"
	AllowmentMissingPermission AllowmentValidationState = "missing_permission"
	AllowmentNotFound          AllowmentValidationState = "not_found"
)

type AllowmentState string

const (
	AllowmentStateNone    AllowmentState = "none"
	AllowmentStatePartial AllowmentState = "partial"
	AllowmentStateValid   AllowmentState = "valid"
	AllowmentStateStale   AllowmentState = "stale"
)

type AllowmentValidation struct {
	TenantID        string                         `json:"tenantId,omitempty"`
	ConnectorID     string                         `json:"connectorId,omitempty"`
	AllowmentID     string                         `json:"allowmentId"`
	ScopeType       ScopeType                      `json:"telegramScopeType"`
	ScopeID         string                         `json:"telegramScopeId"`
	ProviderLabel   string                         `json:"providerLabel,omitempty"`
	Enabled         bool                           `json:"enabled"`
	GroupGate       GroupGate                      `json:"groupGate"`
	ValidationState AllowmentValidationState       `json:"validationState"`
	ReasonCode      string                         `json:"reasonCode,omitempty"`
	ValidatedAt     time.Time                      `json:"validatedAt"`
	RedactionStatus baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence    map[string]string              `json:"safeEvidence,omitempty"`
}

type ConversationType string

const (
	ConversationDirect ConversationType = "direct"
	ConversationGroup  ConversationType = "group"
)

type RouteOutcome string

const (
	RouteAccepted    RouteOutcome = "accepted"
	RouteIgnored     RouteOutcome = "ignored"
	RouteBlocked     RouteOutcome = "blocked"
	RouteDuplicate   RouteOutcome = "duplicate"
	RouteUnsupported RouteOutcome = "unsupported"
	RouteFailed      RouteOutcome = "failed"
)

type InboundUpdate struct {
	UpdateID           string
	MessageID          string
	ChatID             string
	SenderID           string
	Text               string
	ConversationType   ConversationType
	Mentioned          bool
	Command            bool
	UnsupportedSurface string
	ReceivedAt         time.Time
}

type RouteDecision struct {
	Outcome    RouteOutcome
	ReasonCode string
	Surface    string
}

type AllowmentIndex struct {
	users       map[string]AllowmentValidation
	directChats map[string]AllowmentValidation
	groups      map[string]AllowmentValidation
}

func NewAllowmentIndex(items []AllowmentValidation) AllowmentIndex {
	index := AllowmentIndex{
		users:       map[string]AllowmentValidation{},
		directChats: map[string]AllowmentValidation{},
		groups:      map[string]AllowmentValidation{},
	}
	for _, item := range items {
		if !item.Enabled || item.ValidationState != AllowmentValid {
			continue
		}
		scopeID := strings.TrimSpace(item.ScopeID)
		if scopeID == "" {
			continue
		}
		switch item.ScopeType {
		case ScopeUser:
			index.users[scopeID] = item
		case ScopeDirectChat:
			index.directChats[scopeID] = item
		case ScopeGroup:
			index.groups[scopeID] = item
		}
	}
	return index
}

func DecideRoute(update InboundUpdate, allowments AllowmentIndex) RouteDecision {
	surface := string(update.ConversationType)
	if surface == "" {
		surface = "unknown"
	}
	if strings.TrimSpace(update.ChatID) == "" {
		return RouteDecision{Outcome: RouteFailed, ReasonCode: "missing_durable_identity", Surface: surface}
	}
	if strings.TrimSpace(update.UnsupportedSurface) != "" {
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: update.UnsupportedSurface}
	}
	if strings.TrimSpace(update.Text) == "" {
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
	switch update.ConversationType {
	case ConversationDirect, "":
		if _, ok := allowments.directChats[strings.TrimSpace(update.ChatID)]; ok {
			return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: "direct_message"}
		}
		if _, ok := allowments.users[strings.TrimSpace(update.SenderID)]; ok {
			return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: "direct_message"}
		}
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: "direct_message"}
	case ConversationGroup:
		allowment, ok := allowments.groups[strings.TrimSpace(update.ChatID)]
		if !ok {
			return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: "group"}
		}
		if allowment.GroupGate == "" || allowment.GroupGate == GroupGateMentionOrCommandRequired {
			if !update.Mentioned && !update.Command {
				return RouteDecision{Outcome: RouteIgnored, ReasonCode: "mention_required", Surface: "group"}
			}
		}
		return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: "group"}
	default:
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
}

func normalizeAllowments(tenantID, connectorID string, allowments []AllowmentValidation, now time.Time) []AllowmentValidation {
	out := make([]AllowmentValidation, 0, len(allowments))
	for _, item := range allowments {
		item.TenantID = firstNonEmpty(item.TenantID, tenantID)
		item.ConnectorID = firstNonEmpty(item.ConnectorID, connectorID)
		if item.GroupGate == "" {
			if item.ScopeType == ScopeGroup {
				item.GroupGate = GroupGateMentionOrCommandRequired
			} else {
				item.GroupGate = GroupGateNotApplicable
			}
		}
		if item.ValidationState == "" {
			item.ValidationState = AllowmentInvalid
		}
		if item.ReasonCode == "" {
			if item.ValidationState == AllowmentValid {
				item.ReasonCode = "healthy"
			} else {
				item.ReasonCode = string(baseconnectors.DiagnosticBlockedRoute)
			}
		}
		if item.ValidatedAt.IsZero() {
			item.ValidatedAt = now
		}
		if item.RedactionStatus == "" {
			item.RedactionStatus = baseconnectors.RedactionStatusRedacted
		}
		out = append(out, item)
	}
	return out
}

func allowmentState(allowments []AllowmentValidation) AllowmentState {
	if len(allowments) == 0 {
		return AllowmentStateNone
	}
	valid := false
	stale := false
	for _, item := range allowments {
		if item.Enabled && item.ValidationState == AllowmentValid {
			valid = true
		}
		if item.ValidationState == AllowmentStale {
			stale = true
		}
	}
	if valid {
		return AllowmentStateValid
	}
	if stale {
		return AllowmentStateStale
	}
	return AllowmentStatePartial
}

func hasValidAllowment(allowments []AllowmentValidation) bool {
	return allowmentState(allowments) == AllowmentStateValid
}
