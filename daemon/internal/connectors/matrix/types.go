package matrix

import (
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

const ConnectorKind = baseconnectors.ConnectorKindMatrix

type Config struct {
	Enabled              bool
	ConnectorID          string
	DisplayName          string
	HomeserverURL        string
	HomeserverID         string
	BotUserID            string
	SelectedRoomIDs      []string
	AllowedDirectUserIDs []string
	ConfiguredCommands   []string
}

type TerminalState string

const (
	TerminalReady          TerminalState = "ready"
	TerminalDegraded       TerminalState = "degraded"
	TerminalUnavailable    TerminalState = "unavailable"
	TerminalCancelled      TerminalState = "cancelled"
	TerminalActionRequired TerminalState = "action-required"
)

type BotCredentialState string

const (
	BotCredentialNotStarted          BotCredentialState = "not_started"
	BotCredentialSubmitted           BotCredentialState = "submitted"
	BotCredentialValid               BotCredentialState = "valid"
	BotCredentialInvalid             BotCredentialState = "invalid"
	BotCredentialRevoked             BotCredentialState = "revoked"
	BotCredentialPermissionMissing   BotCredentialState = "permission_missing"
	BotCredentialRedactionSuppressed BotCredentialState = "redaction_suppressed"
	BotCredentialUnknown             BotCredentialState = "unknown"
)

type HomeserverState string

const (
	HomeserverReachable        HomeserverState = "reachable"
	HomeserverUnreachable      HomeserverState = "unreachable"
	HomeserverUnsupported      HomeserverState = "unsupported"
	HomeserverRateLimited      HomeserverState = "rate_limited"
	HomeserverFederationFailed HomeserverState = "federation_failed"
	HomeserverNetworkFailed    HomeserverState = "network_failed"
	HomeserverUnknown          HomeserverState = "unknown"
)

type AuthorizationState string

const (
	AuthorizationValid               AuthorizationState = "valid"
	AuthorizationMissing             AuthorizationState = "missing"
	AuthorizationRevoked             AuthorizationState = "revoked"
	AuthorizationPermissionMissing   AuthorizationState = "permission_missing"
	AuthorizationOwnershipMismatch   AuthorizationState = "ownership_mismatch"
	AuthorizationProviderUnavailable AuthorizationState = "provider_unavailable"
	AuthorizationNetworkFailed       AuthorizationState = "network_failed"
	AuthorizationUnknown             AuthorizationState = "unknown"
)

type HomeserverCapabilityState string

const (
	HomeserverCapabilityValid       HomeserverCapabilityState = "valid"
	HomeserverCapabilityUnsupported HomeserverCapabilityState = "unsupported"
	HomeserverCapabilityStale       HomeserverCapabilityState = "stale"
	HomeserverCapabilityRateLimited HomeserverCapabilityState = "rate_limited"
	HomeserverCapabilityUnknown     HomeserverCapabilityState = "unknown"
)

type RoutePolicyState string

const (
	RoutePolicyNone              RoutePolicyState = "none"
	RoutePolicyPartial           RoutePolicyState = "partial"
	RoutePolicyValid             RoutePolicyState = "valid"
	RoutePolicyStale             RoutePolicyState = "stale"
	RoutePolicyBlocked           RoutePolicyState = "blocked"
	RoutePolicyMissingPermission RoutePolicyState = "missing_permission"
)

type ConversationType string

const (
	ConversationDirectMessage ConversationType = "direct_message"
	ConversationRoom          ConversationType = "room"
)

type RoomSelectionState string

const (
	RoomSelected             RoomSelectionState = "selected"
	RoomNotSelected          RoomSelectionState = "not_selected"
	RoomStale                RoomSelectionState = "stale"
	RoomLeft                 RoomSelectionState = "left"
	RoomBanned               RoomSelectionState = "banned"
	RoomMissingMembership    RoomSelectionState = "missing_membership"
	RoomEncryptedUnsupported RoomSelectionState = "encrypted_unsupported"
	RoomNotApplicable        RoomSelectionState = "not_applicable"
)

type MessageKind string

const (
	MessageUnencryptedText           MessageKind = "unencrypted_text"
	MessageEncryptedUnsupported      MessageKind = "encrypted_unsupported"
	MessageUndecryptableUnsupported  MessageKind = "undecryptable_unsupported"
	MessageMediaUnsupported          MessageKind = "media_unsupported"
	MessageCallUnsupported           MessageKind = "call_unsupported"
	MessageVoiceUnsupported          MessageKind = "voice_unsupported"
	MessageReactionUnsupported       MessageKind = "reaction_unsupported"
	MessageBridgeMetadataUnsupported MessageKind = "bridge_metadata_unsupported"
	MessageUnsupported               MessageKind = "unsupported"
	MessageUnknown                   MessageKind = "unknown"
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

type HomeserverBinding struct {
	TenantID            string
	ConnectorID         string
	HomeserverBindingID string
	HomeserverURL       string
	HomeserverName      string
	BotUserID           string
	BotDeviceID         string
	AuthorizationState  AuthorizationState
	CapabilityState     HomeserverCapabilityState
	ValidatedAt         time.Time
	RedactionStatus     baseconnectors.RedactionStatus
	SafeEvidence        map[string]string
}

type ConversationRoute struct {
	ConversationID     string
	ConversationType   ConversationType
	RoomSelectionState RoomSelectionState
	ValidationState    RoutePolicyState
	ReasonCode         string
	RedactionStatus    baseconnectors.RedactionStatus
	SafeEvidence       map[string]string
}

type RoutePolicy struct {
	TenantID            string
	ConnectorID         string
	HomeserverBindingID string
	SelectedRooms       []ConversationRoute
	AllowedDirectUsers  []string
	RoomInvocationGate  string
	ConfiguredCommands  []string
	EncryptedRoomPolicy string
	ValidationState     RoutePolicyState
	ReasonCode          string
	ValidatedAt         time.Time
	RedactionStatus     baseconnectors.RedactionStatus
	SafeEvidence        map[string]string
}

type HostedSetupInput struct {
	TenantID                  string
	ConnectorID               string
	DisplayName               string
	BotCredentialState        BotCredentialState
	HomeserverBinding         HomeserverBinding
	RoutePolicy               RoutePolicy
	ProviderAvailable         bool
	NetworkAvailable          bool
	ConformancePassed         bool
	Cancelled                 bool
	RedactionSuppressed       bool
	RequestedHostedHomeserver bool
	RequestedAccountProvision bool
	StartedAt                 time.Time
	SetupTimeout              time.Duration
	ValidatedAt               time.Time
}

type HostedSetup struct {
	TenantID             string
	ConnectorID          string
	ConnectorKind        string
	DisplayName          string
	Status               baseconnectors.LifecycleState
	TerminalState        TerminalState
	BotCredentialState   BotCredentialState
	HomeserverState      HomeserverState
	RoutePolicyState     RoutePolicyState
	DeliveryEligible     bool
	HomeserverBindingID  string
	ReasonCode           string
	HomeserverBinding    HomeserverBinding
	RoutePolicy          RoutePolicy
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ValidatedAt          time.Time
	RedactionStatus      baseconnectors.RedactionStatus
	RetentionExpiresAt   time.Time
	SetupCompletedWithin time.Duration
}

type InboundEvent struct {
	TenantID         string
	ConnectorID      string
	HomeserverID     string
	ConversationID   string
	MatrixEventID    string
	SyncBatchID      string
	TransactionID    string
	SenderID         string
	ConversationType ConversationType
	MessageKind      MessageKind
	Text             string
	BotMentioned     bool
	ReceivedAt       time.Time
}

type RouteDecision struct {
	Outcome        RouteOutcome
	ReasonCode     string
	Surface        string
	NormalizedText string
}

type RedactionResult struct {
	Status       baseconnectors.RedactionStatus
	SafeEvidence map[string]string
}

type ReplyOutcome struct {
	TenantID                  string
	ConnectorID               string
	InboundEventIdentity      string
	AssistantExecutionOutcome string
	MatrixReplyOutcome        string
	ReplyProgressionLevel     string
	ReplyContext              ConversationType
	FailureReasonCode         string
	RedactionStatus           baseconnectors.RedactionStatus
}
