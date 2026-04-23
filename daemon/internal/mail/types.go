package mail

import "time"

type OperationClass string

const (
	OperationClassListThreads    OperationClass = "list_threads"
	OperationClassGetThread      OperationClass = "get_thread"
	OperationClassGetMessage     OperationClass = "get_message"
	OperationClassListDrafts     OperationClass = "list_drafts"
	OperationClassGetDraft       OperationClass = "get_draft"
	OperationClassCreateDraft    OperationClass = "create_draft"
	OperationClassUpdateDraft    OperationClass = "update_draft"
	OperationClassSendMessage    OperationClass = "send_message"
	OperationClassSendDraft      OperationClass = "send_draft"
	OperationClassReplyMessage   OperationClass = "reply_message"
	OperationClassForwardMessage OperationClass = "forward_message"
)

type OperationStatus string

const (
	OperationStatusRequested OperationStatus = "requested"
	OperationStatusCompleted OperationStatus = "completed"
	OperationStatusFailed    OperationStatus = "failed"
	OperationStatusBlocked   OperationStatus = "blocked"
	OperationStatusCancelled OperationStatus = "cancelled"
)

type ResultMode string

const (
	ResultModeInspection ResultMode = "inspection"
	ResultModeDraftOnly  ResultMode = "draft_only"
	ResultModeSent       ResultMode = "sent"
	ResultModeBlocked    ResultMode = "blocked"
	ResultModeFailed     ResultMode = "failed"
)

type SendPath string

const (
	SendPathDirect SendPath = "direct"
	SendPathDraft  SendPath = "draft"
)

type ArtifactKind string

const (
	ArtifactKindThreadSnapshot  ArtifactKind = "thread_snapshot"
	ArtifactKindMessageSnapshot ArtifactKind = "message_snapshot"
	ArtifactKindDraftSnapshot   ArtifactKind = "draft_snapshot"
	ArtifactKindAttachmentRef   ArtifactKind = "attachment_reference"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type DeliveryState string

const (
	DeliveryStateReceived   DeliveryState = "received"
	DeliveryStateSent       DeliveryState = "sent"
	DeliveryStateBlocked    DeliveryState = "blocked"
	DeliveryStateHistorical DeliveryState = "historical"
)

type DraftStatus string

const (
	DraftStatusDraft         DraftStatus = "draft"
	DraftStatusUpdated       DraftStatus = "updated"
	DraftStatusSendBlocked   DraftStatus = "send_blocked"
	DraftStatusSentFromDraft DraftStatus = "sent_from_draft"
	DraftStatusStaleSnapshot DraftStatus = "stale_snapshot"
	DraftStatusUnavailable   DraftStatus = "unavailable"
)

type ComposeMode string

const (
	ComposeModeNewMessage ComposeMode = "new_message"
	ComposeModeReply      ComposeMode = "reply"
	ComposeModeForward    ComposeMode = "forward"
)

type AttachmentResolutionStatus string

const (
	AttachmentResolutionResolved   AttachmentResolutionStatus = "resolved"
	AttachmentResolutionUnresolved AttachmentResolutionStatus = "unresolved"
	AttachmentResolutionFailed     AttachmentResolutionStatus = "failed"
)

type ReplyForwardResultMode string

const (
	ReplyForwardResultModeDraft ReplyForwardResultMode = "draft"
	ReplyForwardResultModeSend  ReplyForwardResultMode = "send"
)

type AccountProjection struct {
	MailAccountID            string    `json:"mailAccountId"`
	IntegrationID            string    `json:"integrationId"`
	DomainKind               string    `json:"domainKind"`
	EnvironmentScope         string    `json:"environmentScope"`
	AccountKey               string    `json:"accountKey,omitempty"`
	AccountLabel             string    `json:"accountLabel,omitempty"`
	ReadinessStatus          string    `json:"readinessStatus"`
	CanonicalDefault         bool      `json:"canonicalDefault"`
	SelectionMode            string    `json:"selectionMode,omitempty"`
	MailboxAddress           string    `json:"mailboxAddress"`
	MailboxLabel             string    `json:"mailboxLabel"`
	SupportsThreadInspection bool      `json:"supportsThreadInspection"`
	SupportsDrafts           bool      `json:"supportsDrafts"`
	SupportsDirectSend       bool      `json:"supportsDirectSend"`
	SupportsReply            bool      `json:"supportsReply"`
	SupportsForward          bool      `json:"supportsForward"`
	LastSyncedAt             time.Time `json:"lastSyncedAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

type ThreadSnapshot struct {
	ThreadID           string    `json:"threadId"`
	OperationID        string    `json:"operationId"`
	IntegrationID      string    `json:"integrationId"`
	MailAccountID      string    `json:"mailAccountId"`
	Subject            string    `json:"subject"`
	ParticipantSummary []string  `json:"participantSummary,omitempty"`
	MessageIDs         []string  `json:"messageIds,omitempty"`
	DraftIDs           []string  `json:"draftIds,omitempty"`
	LatestMessageAt    time.Time `json:"latestMessageAt"`
	MessageCount       int       `json:"messageCount"`
	DraftCount         int       `json:"draftCount"`
	CreatedAt          time.Time `json:"createdAt"`
}

type MessageSnapshot struct {
	MessageID              string        `json:"messageId"`
	ThreadID               string        `json:"threadId,omitempty"`
	OperationID            string        `json:"operationId"`
	IntegrationID          string        `json:"integrationId"`
	MailAccountID          string        `json:"mailAccountId"`
	Direction              Direction     `json:"direction"`
	SenderSummary          string        `json:"senderSummary,omitempty"`
	RecipientSummary       []string      `json:"recipientSummary,omitempty"`
	Subject                string        `json:"subject"`
	BodyPreview            string        `json:"bodyPreview,omitempty"`
	ReplyToMessageID       string        `json:"replyToMessageId,omitempty"`
	ForwardedFromMessageID string        `json:"forwardedFromMessageId,omitempty"`
	DeliveryState          DeliveryState `json:"deliveryState"`
	AttachmentRefIDs       []string      `json:"attachmentRefIds,omitempty"`
	SentAt                 *time.Time    `json:"sentAt,omitempty"`
	ReceivedAt             *time.Time    `json:"receivedAt,omitempty"`
	CreatedAt              time.Time     `json:"createdAt"`
}

type DraftSnapshot struct {
	DraftID          string      `json:"draftId"`
	ThreadID         string      `json:"threadId,omitempty"`
	OperationID      string      `json:"operationId"`
	IntegrationID    string      `json:"integrationId"`
	MailAccountID    string      `json:"mailAccountId"`
	ComposeMode      ComposeMode `json:"composeMode"`
	SourceMessageID  string      `json:"sourceMessageId,omitempty"`
	RecipientSummary []string    `json:"recipientSummary,omitempty"`
	Subject          string      `json:"subject,omitempty"`
	BodyPreview      string      `json:"bodyPreview,omitempty"`
	AttachmentRefIDs []string    `json:"attachmentRefIds,omitempty"`
	DraftStatus      DraftStatus `json:"draftStatus"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

type AttachmentReference struct {
	AttachmentRefID  string                     `json:"attachmentRefId"`
	OperationID      string                     `json:"operationId,omitempty"`
	IntegrationID    string                     `json:"integrationId,omitempty"`
	ParentKind       string                     `json:"parentKind"`
	ParentID         string                     `json:"parentId"`
	DisplayName      string                     `json:"displayName,omitempty"`
	MediaType        string                     `json:"mediaType,omitempty"`
	SizeBytes        int64                      `json:"sizeBytes,omitempty"`
	ResolutionStatus AttachmentResolutionStatus `json:"resolutionStatus"`
	FailureReason    string                     `json:"failureReason,omitempty"`
	CreatedAt        time.Time                  `json:"createdAt"`
}

type Artifact struct {
	ArtifactID       string               `json:"artifactId"`
	OperationID      string               `json:"operationId"`
	Kind             ArtifactKind         `json:"kind"`
	IntegrationID    string               `json:"integrationId"`
	EnvironmentScope string               `json:"environmentScope"`
	ThreadID         string               `json:"threadId,omitempty"`
	MessageID        string               `json:"messageId,omitempty"`
	DraftID          string               `json:"draftId,omitempty"`
	AttachmentRefID  string               `json:"attachmentRefId,omitempty"`
	Thread           *ThreadSnapshot      `json:"thread,omitempty"`
	Message          *MessageSnapshot     `json:"message,omitempty"`
	Draft            *DraftSnapshot       `json:"draft,omitempty"`
	Attachment       *AttachmentReference `json:"attachment,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
}

type Operation struct {
	OperationID             string          `json:"operationId"`
	OperationClass          OperationClass  `json:"operationClass"`
	Status                  OperationStatus `json:"status"`
	ResultMode              ResultMode      `json:"resultMode"`
	SendPath                SendPath        `json:"sendPath,omitempty"`
	IntegrationID           string          `json:"integrationId"`
	MailAccountID           string          `json:"mailAccountId"`
	EnvironmentScope        string          `json:"environmentScope"`
	SelectionMode           string          `json:"selectionMode,omitempty"`
	ThreadID                string          `json:"threadId,omitempty"`
	MessageID               string          `json:"messageId,omitempty"`
	DraftID                 string          `json:"draftId,omitempty"`
	RequestSummary          string          `json:"requestSummary,omitempty"`
	FailureClass            string          `json:"failureClass,omitempty"`
	FailureReason           string          `json:"failureReason,omitempty"`
	BackgroundSendPermitted bool            `json:"backgroundSendPermitted,omitempty"`
	RunID                   string          `json:"runId,omitempty"`
	StepID                  string          `json:"stepId,omitempty"`
	ToolCallID              string          `json:"toolCallId,omitempty"`
	WorkflowID              string          `json:"workflowId,omitempty"`
	WorkflowStepID          string          `json:"workflowStepId,omitempty"`
	ScheduleID              string          `json:"scheduleId,omitempty"`
	ScheduleAttemptID       string          `json:"scheduleAttemptId,omitempty"`
	DeliveryID              string          `json:"deliveryId,omitempty"`
	ArtifactIDs             []string        `json:"artifactIds,omitempty"`
	CreatedAt               time.Time       `json:"createdAt"`
	CompletedAt             *time.Time      `json:"completedAt,omitempty"`
	UpdatedAt               time.Time       `json:"updatedAt"`
}

type OperationSummary struct {
	OperationID    string          `json:"operationId"`
	OperationClass OperationClass  `json:"operationClass"`
	IntegrationID  string          `json:"integrationId"`
	ThreadID       string          `json:"threadId,omitempty"`
	MessageID      string          `json:"messageId,omitempty"`
	DraftID        string          `json:"draftId,omitempty"`
	ResultMode     ResultMode      `json:"resultMode"`
	SendPath       SendPath        `json:"sendPath,omitempty"`
	Status         OperationStatus `json:"status"`
	CapturedAt     time.Time       `json:"capturedAt"`
}

type Selection struct {
	IntegrationID string
}

type SourceLinkage struct {
	RunID                string
	StepID               string
	ToolCallID           string
	WorkflowID           string
	WorkflowStepID       string
	ScheduleID           string
	ScheduleAttemptID    string
	DeliveryID           string
	AllowSendSideEffects bool
}

type AttachmentRefInput struct {
	AttachmentRefID string `json:"attachmentRefId,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	MediaType       string `json:"mediaType,omitempty"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
}

type Action struct {
	OperationClass       OperationClass         `json:"operationClass"`
	IntegrationID        string                 `json:"integrationId,omitempty"`
	ThreadID             string                 `json:"threadId,omitempty"`
	MessageID            string                 `json:"messageId,omitempty"`
	DraftID              string                 `json:"draftId,omitempty"`
	ComposeMode          ComposeMode            `json:"composeMode,omitempty"`
	ResultMode           ReplyForwardResultMode `json:"resultMode,omitempty"`
	To                   []string               `json:"to,omitempty"`
	Cc                   []string               `json:"cc,omitempty"`
	Bcc                  []string               `json:"bcc,omitempty"`
	Subject              string                 `json:"subject,omitempty"`
	Body                 string                 `json:"body,omitempty"`
	AttachmentRefs       []AttachmentRefInput   `json:"attachmentRefs,omitempty"`
	AllowSendSideEffects bool                   `json:"allowSendSideEffects,omitempty"`
}

type ListThreadsInput struct {
	Selection Selection
	Limit     int
	Cursor    string
	Source    SourceLinkage
}

type GetThreadInput struct {
	Selection Selection
	ThreadID  string
	Source    SourceLinkage
}

type GetMessageInput struct {
	Selection Selection
	MessageID string
	Source    SourceLinkage
}

type ListDraftsInput struct {
	Selection Selection
	Source    SourceLinkage
}

type GetDraftInput struct {
	Selection Selection
	DraftID   string
	Source    SourceLinkage
}

type CreateDraftInput struct {
	Selection       Selection
	ComposeMode     ComposeMode
	ThreadID        string
	SourceMessageID string
	To              []string
	Cc              []string
	Bcc             []string
	Subject         string
	Body            string
	AttachmentRefs  []AttachmentRefInput
	Source          SourceLinkage
}

type UpdateDraftInput struct {
	Selection      Selection
	DraftID        string
	To             []string
	Cc             []string
	Bcc            []string
	Subject        string
	Body           string
	AttachmentRefs []AttachmentRefInput
	Source         SourceLinkage
}

type SendMessageInput struct {
	Selection      Selection
	To             []string
	Cc             []string
	Bcc            []string
	Subject        string
	Body           string
	AttachmentRefs []AttachmentRefInput
	Source         SourceLinkage
}

type SendDraftInput struct {
	Selection Selection
	DraftID   string
	Source    SourceLinkage
}

type ReplyMessageInput struct {
	Selection      Selection
	MessageID      string
	ResultMode     ReplyForwardResultMode
	Subject        string
	Body           string
	AttachmentRefs []AttachmentRefInput
	Source         SourceLinkage
}

type ForwardMessageInput struct {
	Selection      Selection
	MessageID      string
	ResultMode     ReplyForwardResultMode
	To             []string
	Cc             []string
	Bcc            []string
	Subject        string
	Body           string
	AttachmentRefs []AttachmentRefInput
	Source         SourceLinkage
}

type OperationFilter struct {
	IntegrationID  string
	RunID          string
	WorkflowID     string
	ScheduleID     string
	DeliveryID     string
	OperationClass OperationClass
	Status         OperationStatus
	ResultMode     ResultMode
	ThreadID       string
	MessageID      string
	DraftID        string
}
