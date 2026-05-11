package threads

import "time"

type RuntimeResourceKind string

const (
	RuntimeResourceSession            RuntimeResourceKind = "session"
	RuntimeResourceRun                RuntimeResourceKind = "run"
	RuntimeResourceWorkflow           RuntimeResourceKind = "workflow"
	RuntimeResourceApproval           RuntimeResourceKind = "approval"
	RuntimeResourceForegroundReply    RuntimeResourceKind = "foreground_reply"
	RuntimeResourceBackgroundDelivery RuntimeResourceKind = "background_delivery"
	RuntimeResourceConnectorMessage   RuntimeResourceKind = "connector_message"
)

type RuntimeProjection struct {
	RuntimeProjectionID string              `json:"runtimeProjectionId"`
	ThreadID            string              `json:"threadId,omitempty"`
	TenantID            string              `json:"tenantId,omitempty"`
	SessionSegmentID    string              `json:"sessionSegmentId,omitempty"`
	ResourceKind        RuntimeResourceKind `json:"resourceKind"`
	ResourceID          string              `json:"resourceId"`
	Status              string              `json:"status"`
	ReasonCode          string              `json:"reasonCode,omitempty"`
	OccurredAt          time.Time           `json:"occurredAt"`
	Route               string              `json:"route,omitempty"`
	SafeSummary         string              `json:"safeSummary,omitempty"`
	RetentionExpiresAt  time.Time           `json:"retentionExpiresAt,omitempty"`
	RedactionStatus     RedactionStatus     `json:"redactionStatus"`
}

type ThreadListResponse struct {
	TenantID string           `json:"tenantId"`
	Page     ThreadPage       `json:"page"`
	Items    []ThreadResource `json:"items"`
}

type ThreadDetailResponse struct {
	Thread             ThreadResource      `json:"thread"`
	SessionSegments    []SessionSegment    `json:"sessionSegments"`
	SourceLinkages     []SourceLinkage     `json:"sourceLinkages"`
	RuntimeProjections []RuntimeProjection `json:"runtimeProjections"`
	LifecycleActions   []LifecycleAction   `json:"lifecycleActions"`
}

type ThreadPage struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"nextCursor,omitempty"`
	Order      string `json:"order"`
}

type ThreadResource struct {
	ThreadID                string                `json:"threadId"`
	TenantID                string                `json:"tenantId"`
	LifecycleState          LifecycleState        `json:"lifecycleState"`
	SourceKind              SourceKind            `json:"sourceKind"`
	SourceSummary           string                `json:"sourceSummary,omitempty"`
	CurrentSessionSegmentID string                `json:"currentSessionSegmentId,omitempty"`
	CurrentSessionID        string                `json:"currentSessionId,omitempty"`
	LastActivityAt          time.Time             `json:"lastActivityAt"`
	AvailableActions        []LifecycleActionKind `json:"availableActions"`
	RedactionStatus         RedactionStatus       `json:"redactionStatus"`
	RetentionExpiresAt      time.Time             `json:"retentionExpiresAt,omitempty"`
	UpdatedAt               time.Time             `json:"updatedAt"`
}

func BuildThreadResource(thread Thread, currentSessionID string) ThreadResource {
	return ThreadResource{
		ThreadID:                thread.ThreadID,
		TenantID:                thread.TenantID,
		LifecycleState:          thread.LifecycleState,
		SourceKind:              thread.SourceKind,
		SourceSummary:           thread.SourceSummary,
		CurrentSessionSegmentID: thread.CurrentSessionSegmentID,
		CurrentSessionID:        currentSessionID,
		LastActivityAt:          thread.LastActivityAt,
		AvailableActions:        AvailableActions(thread.LifecycleState),
		RedactionStatus:         thread.RedactionStatus,
		RetentionExpiresAt:      thread.RetentionExpiresAt,
		UpdatedAt:               thread.UpdatedAt,
	}
}

type RuntimeProjectionInput struct {
	ProjectionID       string
	ThreadID           string
	TenantID           string
	SessionSegmentID   string
	ResourceKind       RuntimeResourceKind
	ResourceID         string
	Status             string
	ReasonCode         string
	OccurredAt         time.Time
	Route              string
	SafeSummary        string
	RetentionExpiresAt time.Time
	RedactionStatus    RedactionStatus
}

func BuildRuntimeProjection(input RuntimeProjectionInput) RuntimeProjection {
	redactionStatus := input.RedactionStatus
	if redactionStatus == "" {
		redactionStatus = RedactionStatusRedacted
	}
	return RuntimeProjection{
		RuntimeProjectionID: input.ProjectionID,
		ThreadID:            input.ThreadID,
		TenantID:            input.TenantID,
		SessionSegmentID:    input.SessionSegmentID,
		ResourceKind:        input.ResourceKind,
		ResourceID:          input.ResourceID,
		Status:              input.Status,
		ReasonCode:          input.ReasonCode,
		OccurredAt:          input.OccurredAt,
		Route:               input.Route,
		SafeSummary:         SafeSummary(input.SafeSummary, true).Text,
		RetentionExpiresAt:  input.RetentionExpiresAt,
		RedactionStatus:     redactionStatus,
	}
}

func AvailableActions(state LifecycleState) []LifecycleActionKind {
	switch state {
	case LifecycleStateArchived:
		return []LifecycleActionKind{LifecycleActionReopen}
	default:
		return []LifecycleActionKind{LifecycleActionReset, LifecycleActionArchive}
	}
}
