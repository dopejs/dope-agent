package threads

import "time"

type LifecycleState string

const (
	LifecycleStateActive   LifecycleState = "active"
	LifecycleStateReset    LifecycleState = "reset"
	LifecycleStateArchived LifecycleState = "archived"
	LifecycleStateReopened LifecycleState = "reopened"
)

type LifecycleActionKind string

const (
	LifecycleActionReset   LifecycleActionKind = "reset"
	LifecycleActionArchive LifecycleActionKind = "archive"
	LifecycleActionReopen  LifecycleActionKind = "reopen"
)

type Thread struct {
	ThreadID                string
	TenantID                string
	LifecycleState          LifecycleState
	CurrentSessionSegmentID string
	SourceKind              SourceKind
	SourceSummary           string
	LastActivityAt          time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
	RetentionExpiresAt      time.Time
	RedactionStatus         RedactionStatus
}

type SessionSegment struct {
	SessionSegmentID        string     `json:"sessionSegmentId"`
	ThreadID                string     `json:"threadId,omitempty"`
	TenantID                string     `json:"tenantId,omitempty"`
	SessionID               string     `json:"sessionId,omitempty"`
	Generation              int        `json:"generation"`
	State                   string     `json:"state"`
	StartedAt               time.Time  `json:"startedAt"`
	EndedAt                 *time.Time `json:"endedAt,omitempty"`
	LastActiveAt            time.Time  `json:"lastActiveAt"`
	ResetFromSessionSegment string     `json:"resetFromSessionSegmentId,omitempty"`
	PartialEvidence         bool       `json:"partialEvidence"`
}

type LifecycleAction struct {
	LifecycleActionID       string              `json:"lifecycleActionId"`
	ThreadID                string              `json:"threadId,omitempty"`
	TenantID                string              `json:"tenantId,omitempty"`
	ActionKind              LifecycleActionKind `json:"actionKind"`
	ActorPrincipalID        string              `json:"actorPrincipalId,omitempty"`
	PriorState              LifecycleState      `json:"priorState"`
	ResultingState          LifecycleState      `json:"resultingState"`
	PriorSessionSegmentID   string              `json:"priorSessionSegmentId,omitempty"`
	ResultingSessionSegment string              `json:"resultingSessionSegmentId,omitempty"`
	ReasonCode              string              `json:"reasonCode,omitempty"`
	RequestedAt             time.Time           `json:"requestedAt"`
	CompletedAt             time.Time           `json:"completedAt"`
	Status                  string              `json:"status"`
	AuditEventID            string              `json:"auditEventId"`
	RetentionExpiresAt      time.Time           `json:"retentionExpiresAt,omitempty"`
	RedactionStatus         RedactionStatus     `json:"redactionStatus"`
}

type LifecycleAuditRecord struct {
	AuditEventID    string
	TenantID        string
	ThreadID        string
	PrincipalID     string
	Action          string
	PermissionGate  string
	Outcome         string
	ReasonCode      string
	CreatedAt       time.Time
	RedactionStatus RedactionStatus
}

type LegacySessionEvidence struct {
	LegacyEvidenceID    string
	TenantID            string
	ThreadID            string
	SessionID           string
	RoutingKey          string
	Channel             string
	AccountID           string
	PeerID              string
	ThreadIDFromSession string
	ProjectionStatus    string
	MissingFields       []string
	CreatedAt           time.Time
	RedactionStatus     RedactionStatus
}

type LifecycleMutationInput struct {
	ActorPrincipalID string
	ReasonCode       string
	AuditEventID     string
	Now              time.Time
	NewSegmentID     string
}

func ResetThread(thread Thread, input LifecycleMutationInput) (Thread, LifecycleAction, SessionSegment, error) {
	if err := requireAudit(input); err != nil {
		return thread, LifecycleAction{}, SessionSegment{}, err
	}
	if thread.LifecycleState != LifecycleStateActive && thread.LifecycleState != LifecycleStateReopened && thread.LifecycleState != LifecycleStateReset {
		return thread, LifecycleAction{}, SessionSegment{}, ErrLifecycleTransitionNotAllowed
	}
	now := mutationTime(input.Now)
	segmentID := input.NewSegmentID
	if segmentID == "" {
		segmentID = thread.CurrentSessionSegmentID + "_reset"
	}
	updated := thread
	updated.LifecycleState = LifecycleStateReset
	updated.CurrentSessionSegmentID = segmentID
	updated.UpdatedAt = now
	updated.LastActivityAt = now
	action := lifecycleAction(thread, input, LifecycleActionReset, LifecycleStateReset, now)
	action.ResultingSessionSegment = segmentID
	segment := SessionSegment{
		SessionSegmentID:        segmentID,
		ThreadID:                thread.ThreadID,
		TenantID:                thread.TenantID,
		Generation:              1,
		State:                   string(LifecycleStateActive),
		StartedAt:               now,
		LastActiveAt:            now,
		ResetFromSessionSegment: thread.CurrentSessionSegmentID,
	}
	return updated, action, segment, nil
}

func ArchiveThread(thread Thread, input LifecycleMutationInput) (Thread, LifecycleAction, error) {
	if err := requireAudit(input); err != nil {
		return thread, LifecycleAction{}, err
	}
	if thread.LifecycleState == LifecycleStateArchived {
		return thread, LifecycleAction{}, ErrLifecycleTransitionNotAllowed
	}
	now := mutationTime(input.Now)
	updated := thread
	updated.LifecycleState = LifecycleStateArchived
	updated.UpdatedAt = now
	updated.LastActivityAt = now
	action := lifecycleAction(thread, input, LifecycleActionArchive, LifecycleStateArchived, now)
	return updated, action, nil
}

func ReopenThread(thread Thread, input LifecycleMutationInput) (Thread, LifecycleAction, error) {
	if err := requireAudit(input); err != nil {
		return thread, LifecycleAction{}, err
	}
	if thread.LifecycleState != LifecycleStateArchived {
		return thread, LifecycleAction{}, ErrLifecycleTransitionNotAllowed
	}
	now := mutationTime(input.Now)
	updated := thread
	updated.LifecycleState = LifecycleStateReopened
	updated.UpdatedAt = now
	updated.LastActivityAt = now
	action := lifecycleAction(thread, input, LifecycleActionReopen, LifecycleStateReopened, now)
	return updated, action, nil
}

func lifecycleAction(thread Thread, input LifecycleMutationInput, kind LifecycleActionKind, resulting LifecycleState, now time.Time) LifecycleAction {
	return LifecycleAction{
		ThreadID:              thread.ThreadID,
		TenantID:              thread.TenantID,
		ActionKind:            kind,
		ActorPrincipalID:      input.ActorPrincipalID,
		PriorState:            thread.LifecycleState,
		ResultingState:        resulting,
		PriorSessionSegmentID: thread.CurrentSessionSegmentID,
		ReasonCode:            input.ReasonCode,
		RequestedAt:           now,
		CompletedAt:           now,
		Status:                "succeeded",
		AuditEventID:          input.AuditEventID,
		RedactionStatus:       RedactionStatusRedacted,
	}
}

func mutationTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func requireAudit(input LifecycleMutationInput) error {
	if input.AuditEventID == "" {
		return ErrAuditEvidenceRequired
	}
	return nil
}
