package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/reminders"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type SystemInfoResponse struct {
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
	BindAddr    string `json:"bindAddr"`
	DataDir     string `json:"dataDir"`
	LogLevel    string `json:"logLevel"`
}

type ConfigResponse struct {
	Environment    string                   `json:"environment"`
	BindAddr       string                   `json:"bindAddr"`
	DataDir        string                   `json:"dataDir"`
	ConfigFilePath string                   `json:"configFilePath"`
	LogLevel       string                   `json:"logLevel"`
	Version        string                   `json:"version"`
	LLM            ConfigLLMResponse        `json:"llm"`
	Connectors     ConfigConnectorsResponse `json:"connectors"`
	MCP            ConfigMCPResponse        `json:"mcp"`
	Sandbox        ConfigSandboxResponse    `json:"sandbox"`
	RedactedFields []string                 `json:"redactedFields"`
}

type ConfigLLMResponse struct {
	DefaultProvider   string                                 `json:"defaultProvider"`
	DefaultModel      string                                 `json:"defaultModel"`
	DefaultTimeoutMs  int                                    `json:"defaultTimeoutMs"`
	DefaultMaxRetries int                                    `json:"defaultMaxRetries"`
	OpenAICompatible  ConfigOpenAICompatibleProviderResponse `json:"openaiCompatible"`
	Claude            ConfigManagedCLIProviderResponse       `json:"claude"`
	Codex             ConfigManagedCLIProviderResponse       `json:"codex"`
}

type ConfigOpenAICompatibleProviderResponse struct {
	Configured                bool   `json:"configured"`
	BaseURL                   string `json:"baseURL"`
	Model                     string `json:"model"`
	TimeoutMs                 int    `json:"timeoutMs"`
	StreamFirstChunkTimeoutMs int    `json:"streamFirstChunkTimeoutMs"`
	StreamIdleTimeoutMs       int    `json:"streamIdleTimeoutMs"`
	StreamMaxDurationMs       int    `json:"streamMaxDurationMs"`
	APIKeyConfigured          bool   `json:"apiKeyConfigured"`
	APIKeyEnv                 string `json:"apiKeyEnv,omitempty"`
}

type ConfigManagedCLIProviderResponse struct {
	Configured   bool           `json:"configured"`
	CLIPath      string         `json:"cliPath,omitempty"`
	DefaultModel string         `json:"defaultModel,omitempty"`
	WorkDir      string         `json:"workDir,omitempty"`
	Sandbox      map[string]any `json:"sandbox,omitempty"`
}

type ConfigConnectorsResponse struct {
	Discord  ConfigDiscordConnectorResponse  `json:"discord"`
	Telegram ConfigTelegramConnectorResponse `json:"telegram"`
}

type ConfigMCPResponse struct {
	Servers    []mcp.ServerResource      `json:"servers"`
	Catalog    []mcp.CatalogEntry        `json:"catalog,omitempty"`
	Transports []mcp.TransportCapability `json:"transports"`
}

type ConfigSandboxResponse struct {
	Backends []sandbox.BackendCapabilityProfile `json:"backends"`
}

type ConfigDiscordConnectorResponse struct {
	Enabled            bool                                    `json:"enabled"`
	Configured         bool                                    `json:"configured"`
	ConnectorID        string                                  `json:"connectorId"`
	DisplayName        string                                  `json:"displayName"`
	DeliveryMode       string                                  `json:"deliveryMode"`
	RequireMention     bool                                    `json:"requireMention"`
	RespondInDM        bool                                    `json:"respondInDM"`
	AllowedGuildIDs    []string                                `json:"allowedGuildIds"`
	AllowedChannelIDs  []string                                `json:"allowedChannelIds"`
	BotTokenConfigured bool                                    `json:"botTokenConfigured"`
	BotTokenEnv        string                                  `json:"botTokenEnv,omitempty"`
	HostedReadiness    config.DiscordHostedReadinessProjection `json:"hostedReadiness"`
}

type ConfigTelegramConnectorResponse struct {
	Enabled              bool                                     `json:"enabled"`
	Configured           bool                                     `json:"configured"`
	ConnectorID          string                                   `json:"connectorId"`
	DisplayName          string                                   `json:"displayName"`
	BotTokenConfigured   bool                                     `json:"botTokenConfigured"`
	BotTokenEnv          string                                   `json:"botTokenEnv,omitempty"`
	BotUsername          string                                   `json:"botUsername,omitempty"`
	AllowedUserIDs       []string                                 `json:"allowedUserIds"`
	AllowedDirectChatIDs []string                                 `json:"allowedDirectChatIds"`
	AllowedGroupIDs      []string                                 `json:"allowedGroupIds"`
	HostedReadiness      config.TelegramHostedReadinessProjection `json:"hostedReadiness"`
}

type CreateIntegrationDiagnosticRunRequest struct {
	Capabilities []string `json:"capabilities,omitempty"`
	ForceRefresh bool     `json:"forceRefresh,omitempty"`
	ClientKey    string   `json:"clientKey"`
	Reason       string   `json:"reason,omitempty"`
}

type IntegrationDiagnosticListResponse struct {
	IntegrationID    string                          `json:"integrationId,omitempty"`
	TenantID         string                          `json:"tenantId,omitempty"`
	FreshnessSummary string                          `json:"freshnessSummary,omitempty"`
	Items            []integrations.DiagnosticResult `json:"items"`
	NextCursor       string                          `json:"nextCursor,omitempty"`
}

type IntegrationDiagnosticRunListResponse struct {
	Items      []integrations.DiagnosticRun `json:"items"`
	NextCursor string                       `json:"nextCursor,omitempty"`
}

type CreateIntegrationDiagnosticSmokeRequest struct {
	ReportID      string                                  `json:"reportId,omitempty"`
	IntegrationID string                                  `json:"integrationId"`
	Probes        []CreateIntegrationDiagnosticSmokeProbe `json:"probes,omitempty"`
}

type CreateIntegrationDiagnosticSmokeProbe struct {
	IntegrationID            string         `json:"integrationId,omitempty"`
	DomainKind               string         `json:"domainKind,omitempty"`
	ProbeAction              string         `json:"probeAction"`
	SafeCredentialsAvailable bool           `json:"safeCredentialsAvailable"`
	TenantApprovalAvailable  bool           `json:"tenantApprovalAvailable"`
	ProviderAvailable        bool           `json:"providerAvailable"`
	Supported                bool           `json:"supported"`
	ReadOnlyOrReversible     bool           `json:"readOnlyOrReversible"`
	TenantAdminApproved      bool           `json:"tenantAdminApproved,omitempty"`
	OperatorApproved         bool           `json:"operatorApproved,omitempty"`
	OperatorDeferred         bool           `json:"operatorDeferred,omitempty"`
	ReasonCode               string         `json:"reasonCode,omitempty"`
	ProviderEvidence         map[string]any `json:"providerEvidence,omitempty"`
	ArtifactRefs             []string       `json:"artifactRefs,omitempty"`
}

type ChatQueryResponse struct {
	DispatchID     string           `json:"dispatchId"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
	Skills         []string         `json:"skills"`
	SkillContracts []map[string]any `json:"skillContracts,omitempty"`
	Query          string           `json:"query"`
	Status         string           `json:"status"`
	Partial        bool             `json:"partial"`
	Reply          string           `json:"reply"`
	FinishReason   string           `json:"finishReason,omitempty"`
	Usage          llm.Usage        `json:"usage"`
	ErrorCode      string           `json:"errorCode,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type ChatQueryStreamStarted struct {
	DispatchID     string           `json:"dispatchId"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
	Skills         []string         `json:"skills"`
	SkillContracts []map[string]any `json:"skillContracts,omitempty"`
	Query          string           `json:"query"`
}

type ChatQueryStreamDelta struct {
	DispatchID string `json:"dispatchId"`
	Delta      string `json:"delta"`
	Reply      string `json:"reply"`
}

type OperatorOnboardingResponse struct {
	EnvironmentScope        string                      `json:"environmentScope"`
	Status                  string                      `json:"status"`
	CurrentStepID           string                      `json:"currentStepId,omitempty"`
	CompletedStepIDs        []string                    `json:"completedStepIds,omitempty"`
	BlockingItemIDs         []string                    `json:"blockingItemIds"`
	OptionalFollowUpItemIDs []string                    `json:"optionalFollowUpItemIds"`
	RecommendedActionID     string                      `json:"recommendedActionId,omitempty"`
	ReadinessItems          []OperatorReadinessItem     `json:"readinessItems"`
	FirstUsefulActions      []OperatorFirstUsefulAction `json:"firstUsefulActions"`
	LastEvaluatedAt         time.Time                   `json:"lastEvaluatedAt"`
}

type OperatorReadinessItem struct {
	ItemID                    string    `json:"itemId"`
	ItemKind                  string    `json:"itemKind"`
	ResourceID                string    `json:"resourceId,omitempty"`
	DisplayName               string    `json:"displayName"`
	Status                    string    `json:"status"`
	HealthState               string    `json:"healthState,omitempty"`
	Reason                    string    `json:"reason,omitempty"`
	DiagnosticFreshness       string    `json:"diagnosticFreshness,omitempty"`
	RemediationOwner          string    `json:"remediationOwner,omitempty"`
	RetrySafety               string    `json:"retrySafety,omitempty"`
	RequiredOperatorAction    string    `json:"requiredOperatorAction,omitempty"`
	RequiredForSelectedAction bool      `json:"requiredForSelectedAction"`
	DetailRoute               string    `json:"detailRoute,omitempty"`
	EnvironmentScope          string    `json:"environmentScope"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

type OperatorFirstUsefulAction struct {
	ActionID        string   `json:"actionId"`
	ActionKind      string   `json:"actionKind"`
	DisplayName     string   `json:"displayName"`
	Recommended     bool     `json:"recommended"`
	Available       bool     `json:"available"`
	BlockingItemIDs []string `json:"blockingItemIds,omitempty"`
	Summary         string   `json:"summary,omitempty"`
	InvokeRoute     string   `json:"invokeRoute"`
	ResultRoute     string   `json:"resultRoute,omitempty"`
}

type OperatorResourceRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Route string `json:"route,omitempty"`
}

type OperatorActivityRecord struct {
	ActivityID          string                `json:"activityId"`
	SourceKind          string                `json:"sourceKind"`
	SourceID            string                `json:"sourceId"`
	Title               string                `json:"title"`
	Status              string                `json:"status"`
	Summary             string                `json:"summary"`
	AttentionLevel      string                `json:"attentionLevel"`
	OccurredAt          time.Time             `json:"occurredAt"`
	DetailRoute         string                `json:"detailRoute,omitempty"`
	RelatedResourceRefs []OperatorResourceRef `json:"relatedResourceRefs,omitempty"`
	EnvironmentScope    string                `json:"environmentScope"`
}

type OperatorActivityListResponse struct {
	EnvironmentScope string                   `json:"environmentScope"`
	Items            []OperatorActivityRecord `json:"items"`
	GeneratedAt      time.Time                `json:"generatedAt"`
}

type OperatorDiagnosticFinding struct {
	FindingID           string                `json:"findingId"`
	SourceKind          string                `json:"sourceKind"`
	SourceID            string                `json:"sourceId"`
	Plane               string                `json:"plane"`
	Severity            string                `json:"severity"`
	Status              string                `json:"status"`
	Reason              string                `json:"reason"`
	RecommendedAction   string                `json:"recommendedAction,omitempty"`
	DetailRoute         string                `json:"detailRoute,omitempty"`
	RelatedResourceRefs []OperatorResourceRef `json:"relatedResourceRefs,omitempty"`
	EnvironmentScope    string                `json:"environmentScope"`
	CapturedAt          time.Time             `json:"capturedAt"`
}

type OperatorDiagnosticListResponse struct {
	EnvironmentScope string                      `json:"environmentScope"`
	Items            []OperatorDiagnosticFinding `json:"items"`
	GeneratedAt      time.Time                   `json:"generatedAt"`
}

type SessionRouteRequest struct {
	Kind      router.SessionKind `json:"kind,omitempty"`
	Channel   string             `json:"channel,omitempty"`
	AccountID string             `json:"accountId,omitempty"`
	PeerID    string             `json:"peerId,omitempty"`
	ThreadID  string             `json:"threadId,omitempty"`
}

type CreateRunRequest struct {
	SessionID  string               `json:"sessionId,omitempty"`
	Route      *SessionRouteRequest `json:"route,omitempty"`
	Entrypoint string               `json:"entrypoint"`
	Goal       string               `json:"goal,omitempty"`
	Input      any                  `json:"input,omitempty"`
}

type CreateWorkflowRequest struct {
	Goal           string                         `json:"goal,omitempty"`
	CalendarAction *CalendarWorkflowActionRequest `json:"calendarAction,omitempty"`
	MailAction     *MailWorkflowActionRequest     `json:"mailAction,omitempty"`
}

type CreateIntegrationRequest struct {
	IntegrationID      string                      `json:"integrationId"`
	DomainKind         string                      `json:"domainKind"`
	DisplayName        string                      `json:"displayName"`
	BackendKind        integrations.BackendKind    `json:"backendKind"`
	BackendRefID       string                      `json:"backendRefId,omitempty"`
	BackendDisplayName string                      `json:"backendDisplayName,omitempty"`
	AccountBinding     integrations.AccountBinding `json:"accountBinding,omitempty"`
	CanonicalDefault   bool                        `json:"canonicalDefault,omitempty"`
}

type IntegrationListResponse struct {
	Items []integrations.Resource `json:"items"`
}

type ReportIntegrationReadinessRequest struct {
	ReadinessStatus        integrations.ReadinessStatus `json:"readinessStatus"`
	AuthState              integrations.AuthState       `json:"authState,omitempty"`
	HealthState            integrations.HealthState     `json:"healthState,omitempty"`
	Reason                 string                       `json:"reason,omitempty"`
	RequiredOperatorAction string                       `json:"requiredOperatorAction,omitempty"`
	AccountBinding         integrations.AccountBinding  `json:"accountBinding,omitempty"`
	SecretResolution       string                       `json:"secretResolution,omitempty"`
}

type SetIntegrationDefaultRequest struct{}

type CreateIntegrationProbeRequest struct {
	ProbeKind  integrations.ProbeKind `json:"probeKind"`
	ApprovalID string                 `json:"approvalId,omitempty"`
	Input      map[string]any         `json:"input,omitempty"`
}

type IntegrationProbeResponse struct {
	RunID               string                        `json:"runId"`
	StepID              string                        `json:"stepId,omitempty"`
	ToolCallID          string                        `json:"toolCallId,omitempty"`
	Status              string                        `json:"status"`
	IntegrationBindings []integrations.BindingSummary `json:"integrationBindings,omitempty"`
	Approval            *policy.Approval              `json:"approval,omitempty"`
}

type CalendarSourceLinkageRequest struct {
	RunID             string `json:"runId,omitempty"`
	StepID            string `json:"stepId,omitempty"`
	ToolCallID        string `json:"toolCallId,omitempty"`
	WorkflowID        string `json:"workflowId,omitempty"`
	WorkflowStepID    string `json:"workflowStepId,omitempty"`
	ScheduleID        string `json:"scheduleId,omitempty"`
	ScheduleAttemptID string `json:"scheduleAttemptId,omitempty"`
	DeliveryID        string `json:"deliveryId,omitempty"`
}

type CalendarAttendeeRequest struct {
	Email string `json:"email,omitempty"`
}

type CalendarWorkflowActionRequest struct {
	OperationClass  calendar.OperationClass `json:"operationClass"`
	IntegrationID   string                  `json:"integrationId,omitempty"`
	ExternalEventID string                  `json:"externalEventId,omitempty"`
	WindowStart     string                  `json:"windowStart,omitempty"`
	WindowEnd       string                  `json:"windowEnd,omitempty"`
	Title           string                  `json:"title,omitempty"`
	Description     string                  `json:"description,omitempty"`
	Location        string                  `json:"location,omitempty"`
	StartsAt        string                  `json:"startsAt,omitempty"`
	EndsAt          string                  `json:"endsAt,omitempty"`
	Timezone        string                  `json:"timezone,omitempty"`
	CalendarRef     string                  `json:"calendarRef,omitempty"`
	AllDay          bool                    `json:"allDay,omitempty"`
	Recurring       bool                    `json:"recurring,omitempty"`
	Attendees       []string                `json:"attendees,omitempty"`
	Reason          string                  `json:"reason,omitempty"`
}

type MailSourceLinkageRequest struct {
	RunID                string `json:"runId,omitempty"`
	StepID               string `json:"stepId,omitempty"`
	ToolCallID           string `json:"toolCallId,omitempty"`
	WorkflowID           string `json:"workflowId,omitempty"`
	WorkflowStepID       string `json:"workflowStepId,omitempty"`
	ScheduleID           string `json:"scheduleId,omitempty"`
	ScheduleAttemptID    string `json:"scheduleAttemptId,omitempty"`
	DeliveryID           string `json:"deliveryId,omitempty"`
	AllowSendSideEffects bool   `json:"allowSendSideEffects,omitempty"`
}

type MailAttachmentRefRequest struct {
	AttachmentRefID string `json:"attachmentRefId,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	MediaType       string `json:"mediaType,omitempty"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
}

type MailWorkflowActionRequest struct {
	OperationClass       mail.OperationClass         `json:"operationClass"`
	IntegrationID        string                      `json:"integrationId,omitempty"`
	ThreadID             string                      `json:"threadId,omitempty"`
	MessageID            string                      `json:"messageId,omitempty"`
	DraftID              string                      `json:"draftId,omitempty"`
	ComposeMode          mail.ComposeMode            `json:"composeMode,omitempty"`
	ResultMode           mail.ReplyForwardResultMode `json:"resultMode,omitempty"`
	To                   []string                    `json:"to,omitempty"`
	Cc                   []string                    `json:"cc,omitempty"`
	Bcc                  []string                    `json:"bcc,omitempty"`
	Subject              string                      `json:"subject,omitempty"`
	Body                 string                      `json:"body,omitempty"`
	AttachmentRefs       []MailAttachmentRefRequest  `json:"attachmentRefs,omitempty"`
	AllowSendSideEffects bool                        `json:"allowSendSideEffects,omitempty"`
}

type CreateCalendarAvailabilityQueryRequest struct {
	IntegrationID string                        `json:"integrationId,omitempty"`
	WindowStart   string                        `json:"windowStart"`
	WindowEnd     string                        `json:"windowEnd"`
	Timezone      string                        `json:"timezone,omitempty"`
	Source        *CalendarSourceLinkageRequest `json:"source,omitempty"`
}

type CreateCalendarEventRequest struct {
	IntegrationID string                        `json:"integrationId,omitempty"`
	CalendarRef   string                        `json:"calendarRef,omitempty"`
	Title         string                        `json:"title,omitempty"`
	Description   string                        `json:"description,omitempty"`
	Location      string                        `json:"location,omitempty"`
	StartsAt      string                        `json:"startsAt"`
	EndsAt        string                        `json:"endsAt"`
	Timezone      string                        `json:"timezone,omitempty"`
	AllDay        bool                          `json:"allDay,omitempty"`
	Recurring     bool                          `json:"recurring,omitempty"`
	Attendees     []CalendarAttendeeRequest     `json:"attendees,omitempty"`
	Source        *CalendarSourceLinkageRequest `json:"source,omitempty"`
}

type UpdateCalendarEventRequest = CreateCalendarEventRequest

type CancelCalendarEventRequest struct {
	IntegrationID string                        `json:"integrationId,omitempty"`
	CalendarRef   string                        `json:"calendarRef,omitempty"`
	Reason        string                        `json:"reason,omitempty"`
	Source        *CalendarSourceLinkageRequest `json:"source,omitempty"`
}

type CreateMailDraftRequest struct {
	IntegrationID   string                     `json:"integrationId,omitempty"`
	ComposeMode     mail.ComposeMode           `json:"composeMode"`
	ThreadID        string                     `json:"threadId,omitempty"`
	SourceMessageID string                     `json:"sourceMessageId,omitempty"`
	To              []string                   `json:"to,omitempty"`
	Cc              []string                   `json:"cc,omitempty"`
	Bcc             []string                   `json:"bcc,omitempty"`
	Subject         string                     `json:"subject,omitempty"`
	Body            string                     `json:"body,omitempty"`
	AttachmentRefs  []MailAttachmentRefRequest `json:"attachmentRefs,omitempty"`
	Source          *MailSourceLinkageRequest  `json:"source,omitempty"`
}

type UpdateMailDraftRequest struct {
	IntegrationID  string                     `json:"integrationId,omitempty"`
	To             []string                   `json:"to,omitempty"`
	Cc             []string                   `json:"cc,omitempty"`
	Bcc            []string                   `json:"bcc,omitempty"`
	Subject        string                     `json:"subject,omitempty"`
	Body           string                     `json:"body,omitempty"`
	AttachmentRefs []MailAttachmentRefRequest `json:"attachmentRefs,omitempty"`
	Source         *MailSourceLinkageRequest  `json:"source,omitempty"`
}

type SendMailMessageRequest struct {
	IntegrationID  string                     `json:"integrationId,omitempty"`
	To             []string                   `json:"to,omitempty"`
	Cc             []string                   `json:"cc,omitempty"`
	Bcc            []string                   `json:"bcc,omitempty"`
	Subject        string                     `json:"subject,omitempty"`
	Body           string                     `json:"body,omitempty"`
	AttachmentRefs []MailAttachmentRefRequest `json:"attachmentRefs,omitempty"`
	Source         *MailSourceLinkageRequest  `json:"source,omitempty"`
}

type SendMailDraftRequest struct {
	IntegrationID string                    `json:"integrationId,omitempty"`
	Source        *MailSourceLinkageRequest `json:"source,omitempty"`
}

type ReplyMailMessageRequest struct {
	IntegrationID  string                      `json:"integrationId,omitempty"`
	ResultMode     mail.ReplyForwardResultMode `json:"resultMode,omitempty"`
	Subject        string                      `json:"subject,omitempty"`
	Body           string                      `json:"body,omitempty"`
	AttachmentRefs []MailAttachmentRefRequest  `json:"attachmentRefs,omitempty"`
	Source         *MailSourceLinkageRequest   `json:"source,omitempty"`
}

type ForwardMailMessageRequest struct {
	IntegrationID  string                      `json:"integrationId,omitempty"`
	ResultMode     mail.ReplyForwardResultMode `json:"resultMode,omitempty"`
	To             []string                    `json:"to,omitempty"`
	Cc             []string                    `json:"cc,omitempty"`
	Bcc            []string                    `json:"bcc,omitempty"`
	Subject        string                      `json:"subject,omitempty"`
	Body           string                      `json:"body,omitempty"`
	AttachmentRefs []MailAttachmentRefRequest  `json:"attachmentRefs,omitempty"`
	Source         *MailSourceLinkageRequest   `json:"source,omitempty"`
}

type CalendarAccountListResponse = ListResponse[calendar.AccountProjection]
type CalendarOperationListResponse = ListResponse[calendar.Operation]
type MailAccountListResponse = ListResponse[mail.AccountProjection]
type MailOperationListResponse = ListResponse[mail.Operation]

type CalendarEventListResponse struct {
	Account   calendar.AccountProjection `json:"account"`
	Items     []calendar.Event           `json:"items"`
	Operation calendar.Operation         `json:"operation"`
	Artifacts []calendar.Artifact        `json:"artifacts,omitempty"`
}

type CalendarEventResponse struct {
	Account   calendar.AccountProjection `json:"account"`
	Event     calendar.Event             `json:"event"`
	Operation calendar.Operation         `json:"operation"`
	Artifacts []calendar.Artifact        `json:"artifacts,omitempty"`
}

type CalendarAvailabilityQueryResponse struct {
	Account   calendar.AccountProjection `json:"account"`
	Query     calendar.AvailabilityQuery `json:"query"`
	Operation calendar.Operation         `json:"operation"`
	Artifacts []calendar.Artifact        `json:"artifacts,omitempty"`
}

type CalendarOperationResponse struct {
	Operation calendar.Operation  `json:"operation"`
	Artifacts []calendar.Artifact `json:"artifacts,omitempty"`
}

type MailThreadListResponse struct {
	Account   mail.AccountProjection `json:"account"`
	Items     []mail.ThreadSnapshot  `json:"items"`
	Operation mail.Operation         `json:"operation"`
	Artifacts []mail.Artifact        `json:"artifacts,omitempty"`
}

type MailThreadResponse struct {
	Account   mail.AccountProjection `json:"account"`
	Thread    mail.ThreadSnapshot    `json:"thread"`
	Operation mail.Operation         `json:"operation"`
	Artifacts []mail.Artifact        `json:"artifacts,omitempty"`
}

type MailMessageResponse struct {
	Account   mail.AccountProjection `json:"account"`
	Message   mail.MessageSnapshot   `json:"message"`
	Operation mail.Operation         `json:"operation"`
	Artifacts []mail.Artifact        `json:"artifacts,omitempty"`
}

type MailDraftListResponse struct {
	Account   mail.AccountProjection `json:"account"`
	Items     []mail.DraftSnapshot   `json:"items"`
	Operation mail.Operation         `json:"operation"`
	Artifacts []mail.Artifact        `json:"artifacts,omitempty"`
}

type MailDraftResponse struct {
	Account   mail.AccountProjection `json:"account"`
	Draft     mail.DraftSnapshot     `json:"draft"`
	Operation mail.Operation         `json:"operation"`
	Artifacts []mail.Artifact        `json:"artifacts,omitempty"`
}

type MailOperationResponse struct {
	Operation mail.Operation  `json:"operation"`
	Artifacts []mail.Artifact `json:"artifacts,omitempty"`
}

type CreateComputerUseSessionRequest struct {
	WorkflowID     string `json:"workflowId,omitempty"`
	WorkflowStepID string `json:"workflowStepId,omitempty"`
	DriverKind     string `json:"driverKind,omitempty"`
	InitialURL     string `json:"initialUrl,omitempty"`
}

type CreateComputerUseActionRequest struct {
	ActionKind         computeruse.ActionKind          `json:"actionKind"`
	URL                string                          `json:"url,omitempty"`
	Value              string                          `json:"value,omitempty"`
	SelectedValue      string                          `json:"selectedValue,omitempty"`
	WaitMs             int                             `json:"waitMs,omitempty"`
	PageTarget         computeruse.PageTarget          `json:"pageTarget,omitempty"`
	TargetMatchContext *computeruse.TargetMatchContext `json:"targetMatchContext,omitempty"`
	Rationale          string                          `json:"rationale,omitempty"`
}

type ComputerUseArtifactContentResponse struct {
	ArtifactID string `json:"artifactId"`
	MIMEType   string `json:"mimeType,omitempty"`
	FileName   string `json:"fileName,omitempty"`
	Status     string `json:"status"`
	Content    string `json:"content,omitempty"`
}

type CreateScheduleRequest struct {
	Trigger     ScheduleTriggerRequest `json:"trigger"`
	Target      ScheduleTargetRequest  `json:"target"`
	RetryPolicy scheduler.RetryPolicy  `json:"retryPolicy"`
}

type CreateReminderRequest struct {
	Title                string                         `json:"title"`
	Details              string                         `json:"details,omitempty"`
	BehaviorMode         reminders.BehaviorMode         `json:"behaviorMode,omitempty"`
	Trigger              ScheduleTriggerRequest         `json:"trigger"`
	WorkflowLaunchConfig *ReminderWorkflowLaunchRequest `json:"workflowLaunchConfig,omitempty"`
	FollowUpLink         *ReminderFollowUpLinkRequest   `json:"followUpLink,omitempty"`
}

type ReminderWorkflowLaunchRequest struct {
	SessionID      string                         `json:"sessionId,omitempty"`
	Entrypoint     string                         `json:"entrypoint"`
	RunGoal        string                         `json:"runGoal,omitempty"`
	WorkflowGoal   string                         `json:"workflowGoal,omitempty"`
	CalendarAction *CalendarWorkflowActionRequest `json:"calendarAction,omitempty"`
	MailAction     *MailWorkflowActionRequest     `json:"mailAction,omitempty"`
}

type ReminderFollowUpLinkRequest struct {
	LinkKind           reminders.FollowUpLinkKind `json:"linkKind"`
	SourceID           string                     `json:"sourceId"`
	EnvironmentScope   string                     `json:"environmentScope,omitempty"`
	SourceSummary      string                     `json:"sourceSummary,omitempty"`
	SourceDisplayState string                     `json:"sourceDisplayState,omitempty"`
}

type ReminderTransitionRequest struct {
	OccurrenceID string                  `json:"occurrenceId,omitempty"`
	Reason       string                  `json:"reason,omitempty"`
	ActorKind    reminders.ActorKind     `json:"actorKind,omitempty"`
	SnoozedUntil string                  `json:"snoozedUntil,omitempty"`
	Trigger      *ScheduleTriggerRequest `json:"trigger,omitempty"`
}

type ScheduleTriggerRequest struct {
	Kind     scheduler.TriggerKind `json:"kind"`
	FireAt   string                `json:"fireAt,omitempty"`
	CronExpr string                `json:"cronExpr,omitempty"`
	Timezone string                `json:"timezone,omitempty"`
}

type ScheduleTargetRequest struct {
	Kind     scheduler.TargetKind           `json:"kind"`
	Run      *scheduler.RunTarget           `json:"run,omitempty"`
	Workflow *ScheduleWorkflowTargetRequest `json:"workflow,omitempty"`
}

type ScheduleWorkflowTargetRequest struct {
	SessionID      string                         `json:"sessionId,omitempty"`
	Entrypoint     string                         `json:"entrypoint"`
	RunGoal        string                         `json:"runGoal,omitempty"`
	WorkflowGoal   string                         `json:"workflowGoal,omitempty"`
	CalendarAction *CalendarWorkflowActionRequest `json:"calendarAction,omitempty"`
	MailAction     *MailWorkflowActionRequest     `json:"mailAction,omitempty"`
}

type ScheduleListResponse = ListResponse[scheduler.Schedule]
type ReminderListResponse = ListResponse[reminders.Reminder]
type ReminderOccurrenceListResponse = ListResponse[reminders.Occurrence]
type ReminderActionListResponse = ListResponse[reminders.ActionRecord]

type CreateDeliveryTargetRequest struct {
	TargetID         string                     `json:"targetId"`
	DisplayName      string                     `json:"displayName"`
	TargetKind       delivery.TargetKind        `json:"targetKind"`
	ConnectorBinding *delivery.ConnectorBinding `json:"connectorBinding,omitempty"`
	AddressSummary   string                     `json:"addressSummary,omitempty"`
}

type UpdateDeliveryTargetStatusRequest struct{}

type DeliveryTargetListResponse = ListResponse[delivery.DeliveryTarget]

type UpsertDeliveryPreferenceRequest struct {
	PreferenceID            string                          `json:"preferenceId"`
	EnvironmentScope        string                          `json:"environmentScope,omitempty"`
	ScopeKind               delivery.PreferenceScopeKind    `json:"scopeKind"`
	IntegrationID           string                          `json:"integrationId,omitempty"`
	PreferredTargetsByClass map[delivery.ResultClass]string `json:"preferredTargetsByClass"`
	SummaryPolicy           delivery.SummaryPolicy          `json:"summaryPolicy,omitempty"`
	SuppressionPolicy       delivery.SuppressionPolicy      `json:"suppressionPolicy,omitempty"`
}

type DeliveryPreferenceListResponse = ListResponse[delivery.DeliveryPreference]
type DeliveryOutcomeListResponse = ListResponse[delivery.DeliveryOutcome]
type DeliverySummaryWindowListResponse = ListResponse[delivery.SummaryWindow]

func applyLatestDeliveryToRun(run runtime.Run, summary delivery.LatestSummary) runtime.Run {
	run.LatestDeliveryID = summary.LatestDeliveryID
	run.LatestDeliveryStatus = summary.LatestDeliveryStatus
	run.LatestDeliveryTargetID = summary.LatestDeliveryTargetID
	return run
}

func applyLatestDeliveryToWorkflow(workflow orchestration.Workflow, summary delivery.LatestSummary) orchestration.Workflow {
	workflow.LatestDeliveryID = summary.LatestDeliveryID
	workflow.LatestDeliveryStatus = summary.LatestDeliveryStatus
	workflow.LatestDeliveryTargetID = summary.LatestDeliveryTargetID
	return workflow
}

func applyLatestDeliveryToSchedule(schedule scheduler.Schedule, summaries map[string]delivery.LatestSummary) scheduler.Schedule {
	if len(summaries) == 0 {
		return schedule
	}
	for idx := range schedule.Attempts {
		summary, ok := summaries[schedule.Attempts[idx].AttemptID]
		if !ok {
			continue
		}
		schedule.Attempts[idx].LatestDeliveryID = summary.LatestDeliveryID
		schedule.Attempts[idx].LatestDeliveryStatus = summary.LatestDeliveryStatus
		schedule.Attempts[idx].LatestDeliveryTargetID = summary.LatestDeliveryTargetID
	}
	return schedule
}

type ConnectorIngressMessage struct {
	MessageID               string `json:"messageId"`
	ConnectorAccountID      string `json:"connectorAccountId,omitempty"`
	ChannelOrConversationID string `json:"channelOrConversationId,omitempty"`
	ProviderMessageID       string `json:"providerMessageId,omitempty"`
	EquivalentRuleID        string `json:"equivalentRuleId,omitempty"`
	Text                    string `json:"text,omitempty"`
	Payload                 any    `json:"payload,omitempty"`
}

type ConnectorIngressRunRequest struct {
	Entrypoint string `json:"entrypoint"`
	Goal       string `json:"goal,omitempty"`
}

type ConnectorIngressMessageRequest struct {
	TenantID string                      `json:"tenantId,omitempty"`
	Route    SessionRouteRequest         `json:"route"`
	Message  ConnectorIngressMessage     `json:"message"`
	Run      *ConnectorIngressRunRequest `json:"run,omitempty"`
}

type ConnectorIngressMessageResponse struct {
	IngressID       string          `json:"ingressId"`
	ConnectorID     string          `json:"connectorId"`
	Outcome         string          `json:"outcome"`
	ReasonCode      string          `json:"reasonCode"`
	RedactionStatus string          `json:"redactionStatus"`
	AcceptedAt      time.Time       `json:"acceptedAt"`
	Session         *router.Session `json:"session,omitempty"`
	SessionCreated  bool            `json:"sessionCreated"`
	Run             *runtime.Run    `json:"run,omitempty"`
	RunCreated      bool            `json:"runCreated"`
}

type EventListResponse struct {
	Items      []events.Event `json:"items"`
	NextCursor int64          `json:"nextCursor,omitempty"`
}

type ProviderListResponse struct {
	Items []providers.Profile `json:"items"`
}

type ProviderCheckListResponse struct {
	Items []providers.Check `json:"items"`
}

type ProviderAuthStateResponse struct {
	Auth providers.AuthState `json:"auth"`
}

type ProviderModelListResponse struct {
	Items []providers.Model `json:"items"`
}

type ProviderDefaultModelRequest struct {
	Model string `json:"model"`
}

type ProviderDefaultModelResponse struct {
	ProviderID   string    `json:"providerId"`
	DefaultModel string    `json:"defaultModel"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SkillFileResponse struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

type SkillSummaryResponse struct {
	SkillID            string                     `json:"skillId"`
	Name               string                     `json:"name"`
	Description        string                     `json:"description"`
	Source             string                     `json:"source"`
	RootPath           string                     `json:"rootPath"`
	SkillPath          string                     `json:"skillPath"`
	InstructionPath    string                     `json:"instructionPath"`
	Files              []SkillFileResponse        `json:"files"`
	Frontmatter        map[string]string          `json:"frontmatter"`
	ExecutionManifest  *skills.ExecutableManifest `json:"executionManifest,omitempty"`
	AvailabilityStatus string                     `json:"availabilityStatus,omitempty"`
	AvailabilityReason string                     `json:"availabilityReason,omitempty"`
	Sandbox            map[string]any             `json:"sandbox,omitempty"`
}

type SkillDetailResponse struct {
	SkillSummaryResponse
	FrontmatterRaw string `json:"frontmatterRaw,omitempty"`
	Body           string `json:"body"`
}

type SkillOverlayResponse struct {
	OverlayID  string    `json:"overlayId"`
	Source     string    `json:"source"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"sizeBytes"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type SkillRegistryResponse struct {
	LoadedAt time.Time              `json:"loadedAt"`
	Items    []SkillSummaryResponse `json:"items"`
	Overlays []SkillOverlayResponse `json:"overlays"`
}

type SandboxExplainResponse struct {
	Decision sandbox.Decision `json:"decision"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
}

type AuthMeResponse struct {
	Token          auth.AccessToken            `json:"token"`
	Principal      identity.Principal          `json:"principal"`
	DefaultTenant  identity.Tenant             `json:"defaultTenant"`
	CurrentTenant  identity.Tenant             `json:"currentTenant"`
	AllowedTenants []identity.Tenant           `json:"allowedTenants"`
	TokenGrants    []identity.TokenTenantGrant `json:"tokenGrants"`
	Permissions    []identity.Permission       `json:"permissions"`
	TenantContext  identity.TenantContext      `json:"tenantContext"`
}

type TenantListResponse = ListResponse[identity.Tenant]

type TenantDetailResponse struct {
	Tenant        identity.Tenant        `json:"tenant"`
	TenantContext identity.TenantContext `json:"tenantContext"`
}

func withTenantContext(ctx context.Context, tenantContext identity.TenantContext) context.Context {
	// Roadmap 35: route the carrier through daemon/internal/tenantctx so the
	// store-layer tenancy guards can read the same value without depending on
	// this api-private package.
	return tenantctx.WithContext(ctx, tenantContext)
}

func tenantContextFromContext(ctx context.Context) (identity.TenantContext, bool) {
	return tenantctx.FromContext(ctx)
}

func withTenantAuditStore(ctx context.Context, sqliteStore *store.SQLiteStore) context.Context {
	return context.WithValue(ctx, tenantAuditStoreKey, sqliteStore)
}

func tenantAuditStoreFromContext(ctx context.Context) (*store.SQLiteStore, bool) {
	sqliteStore, ok := ctx.Value(tenantAuditStoreKey).(*store.SQLiteStore)
	return sqliteStore, ok
}

func RequirePermission(ctx context.Context, permission identity.Permission) (identity.TenantContext, error) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok {
		return identity.TenantContext{}, identity.ErrTenantAccessDenied
	}
	if err := identity.RequirePermission(tenantContext, permission); err != nil {
		if sqliteStore, ok := tenantAuditStoreFromContext(ctx); ok && sqliteStore != nil {
			_, _ = sqliteStore.AppendTenantAuditEvent(ctx, identity.TenantAuditEvent{
				EventKind:   "tenant.permission_denied",
				TenantID:    tenantContext.TenantID,
				PrincipalID: tenantContext.PrincipalID,
				TokenID:     tenantContext.TokenID,
				Outcome:     identity.AuditOutcomeDenied,
				ReasonCode:  "permission_denied:" + string(permission),
				CreatedAt:   time.Now().UTC(),
			})
		}
		return identity.TenantContext{}, err
	}
	return tenantContext, nil
}

type WorkflowListResponse = ListResponse[orchestration.Workflow]
type ComputerUseSessionListResponse = ListResponse[computeruse.Session]
type ComputerUseActionListResponse = ListResponse[computeruse.Action]

func buildSystemInfoResponse(cfg config.Config) SystemInfoResponse {
	return SystemInfoResponse{
		Service:     "dope",
		Environment: effectiveEnvironment(cfg),
		Version:     cfg.Version,
		BindAddr:    cfg.BindAddr,
		DataDir:     cfg.DataDir,
		LogLevel:    cfg.LogLevel,
	}
}

func buildConfigResponse(cfg config.Config, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager) ConfigResponse {
	redactedFields := []string{}
	if cfg.LLM.OpenAICompatible.APIKey != "" {
		redactedFields = append(redactedFields, "llm.openaiCompatible.apiKey")
	}
	if cfg.Connectors.Discord.BotToken != "" {
		redactedFields = append(redactedFields, "connectors.discord.botToken")
	}
	if cfg.Connectors.Telegram.BotToken != "" {
		redactedFields = append(redactedFields, "connectors.telegram.botToken")
	}
	defaultTimeoutMs := cfg.LLM.DefaultTimeoutMs
	if defaultTimeoutMs <= 0 {
		defaultTimeoutMs = 30000
	}
	openAITimeoutMs := cfg.LLM.OpenAICompatible.TimeoutMs
	if openAITimeoutMs <= 0 {
		openAITimeoutMs = defaultTimeoutMs
	}
	firstChunkTimeoutMs := cfg.LLM.OpenAICompatible.StreamFirstChunkTimeoutMs
	if firstChunkTimeoutMs <= 0 {
		firstChunkTimeoutMs = openAITimeoutMs
	}
	idleTimeoutMs := cfg.LLM.OpenAICompatible.StreamIdleTimeoutMs
	if idleTimeoutMs <= 0 {
		idleTimeoutMs = firstChunkTimeoutMs
	}
	discordDeliveryMode := cfg.Connectors.Discord.DeliveryMode
	if discordDeliveryMode == "" {
		discordDeliveryMode = "gateway"
	}

	return ConfigResponse{
		Environment:    effectiveEnvironment(cfg),
		BindAddr:       cfg.BindAddr,
		DataDir:        cfg.DataDir,
		ConfigFilePath: config.ConfigFilePath(cfg.DataDir),
		LogLevel:       cfg.LogLevel,
		Version:        cfg.Version,
		LLM: ConfigLLMResponse{
			DefaultProvider:   cfg.LLM.DefaultProvider,
			DefaultModel:      cfg.LLM.DefaultModel,
			DefaultTimeoutMs:  defaultTimeoutMs,
			DefaultMaxRetries: cfg.LLM.DefaultMaxRetries,
			OpenAICompatible: ConfigOpenAICompatibleProviderResponse{
				Configured:                cfg.LLM.OpenAICompatible.BaseURL != "" || cfg.LLM.OpenAICompatible.APIKey != "" || cfg.LLM.OpenAICompatible.Model != "",
				BaseURL:                   cfg.LLM.OpenAICompatible.BaseURL,
				Model:                     cfg.LLM.OpenAICompatible.Model,
				TimeoutMs:                 openAITimeoutMs,
				StreamFirstChunkTimeoutMs: firstChunkTimeoutMs,
				StreamIdleTimeoutMs:       idleTimeoutMs,
				StreamMaxDurationMs:       cfg.LLM.OpenAICompatible.StreamMaxDurationMs,
				APIKeyConfigured:          cfg.LLM.OpenAICompatible.APIKey != "",
				APIKeyEnv:                 cfg.LLM.OpenAICompatible.APIKeyEnv,
			},
			Claude: ConfigManagedCLIProviderResponse{
				Configured:   cfg.LLM.Claude.CLIPath != "" || cfg.LLM.Claude.DefaultModel != "" || (cfg.LLM.Claude.WorkDir != "" && cfg.LLM.Claude.WorkDir != "~"),
				CLIPath:      cfg.LLM.Claude.CLIPath,
				DefaultModel: cfg.LLM.Claude.DefaultModel,
				WorkDir:      cfg.LLM.Claude.WorkDir,
				Sandbox:      buildManagedProviderConfigSandbox("claude_managed", cfg.LLM.Claude.WorkDir),
			},
			Codex: ConfigManagedCLIProviderResponse{
				Configured:   cfg.LLM.Codex.CLIPath != "" || cfg.LLM.Codex.DefaultModel != "" || (cfg.LLM.Codex.WorkDir != "" && cfg.LLM.Codex.WorkDir != "~"),
				CLIPath:      cfg.LLM.Codex.CLIPath,
				DefaultModel: cfg.LLM.Codex.DefaultModel,
				WorkDir:      cfg.LLM.Codex.WorkDir,
				Sandbox:      buildManagedProviderConfigSandbox("codex_managed", cfg.LLM.Codex.WorkDir),
			},
		},
		Connectors: ConfigConnectorsResponse{
			Discord: ConfigDiscordConnectorResponse{
				Enabled:            cfg.Connectors.Discord.Enabled,
				Configured:         cfg.Connectors.Discord.BotToken != "",
				ConnectorID:        cfg.Connectors.Discord.ConnectorID,
				DisplayName:        cfg.Connectors.Discord.DisplayName,
				DeliveryMode:       discordDeliveryMode,
				RequireMention:     cfg.Connectors.Discord.RequireMention,
				RespondInDM:        cfg.Connectors.Discord.RespondInDM,
				AllowedGuildIDs:    cloneStringSlice(cfg.Connectors.Discord.AllowedGuildIDs),
				AllowedChannelIDs:  cloneStringSlice(cfg.Connectors.Discord.AllowedChannelIDs),
				BotTokenConfigured: cfg.Connectors.Discord.BotToken != "",
				BotTokenEnv:        cfg.Connectors.Discord.BotTokenEnv,
				HostedReadiness:    cfg.Connectors.Discord.ProjectHostedReadiness(""),
			},
			Telegram: ConfigTelegramConnectorResponse{
				Enabled:              cfg.Connectors.Telegram.Enabled,
				Configured:           cfg.Connectors.Telegram.BotToken != "",
				ConnectorID:          cfg.Connectors.Telegram.ConnectorID,
				DisplayName:          cfg.Connectors.Telegram.DisplayName,
				BotTokenConfigured:   cfg.Connectors.Telegram.BotToken != "",
				BotTokenEnv:          cfg.Connectors.Telegram.BotTokenEnv,
				BotUsername:          cfg.Connectors.Telegram.BotUsername,
				AllowedUserIDs:       cloneStringSlice(cfg.Connectors.Telegram.AllowedUserIDs),
				AllowedDirectChatIDs: cloneStringSlice(cfg.Connectors.Telegram.AllowedDirectChatIDs),
				AllowedGroupIDs:      cloneStringSlice(cfg.Connectors.Telegram.AllowedGroupIDs),
				HostedReadiness:      cfg.Connectors.Telegram.ProjectHostedReadiness(""),
			},
		},
		MCP: ConfigMCPResponse{
			Servers:    listMCPServersForConfig(mcpManager),
			Catalog:    listMCPCatalogForConfig(mcpManager),
			Transports: listMCPTransportsForConfig(mcpManager),
		},
		Sandbox: ConfigSandboxResponse{
			Backends: listSandboxBackendsForConfig(sandboxManager),
		},
		RedactedFields: redactedFields,
	}
}

func listMCPServersForConfig(manager *mcp.Manager) []mcp.ServerResource {
	if manager == nil {
		return []mcp.ServerResource{}
	}
	return manager.ListServers()
}

func listMCPCatalogForConfig(manager *mcp.Manager) []mcp.CatalogEntry {
	if manager == nil {
		return []mcp.CatalogEntry{}
	}
	return manager.ListCatalog()
}

func listMCPTransportsForConfig(manager *mcp.Manager) []mcp.TransportCapability {
	if manager == nil {
		return []mcp.TransportCapability{}
	}
	return manager.ListTransportCapabilities()
}

func listSandboxBackendsForConfig(manager *sandbox.Manager) []sandbox.BackendCapabilityProfile {
	if manager == nil {
		return []sandbox.BackendCapabilityProfile{}
	}
	return manager.BackendCapabilities()
}

func buildSkillRegistryResponse(snapshot skills.Snapshot) SkillRegistryResponse {
	items := make([]SkillSummaryResponse, 0, len(snapshot.Skills))
	for _, skill := range snapshot.Skills {
		items = append(items, buildSkillSummaryResponse(skill))
	}
	overlays := make([]SkillOverlayResponse, 0, len(snapshot.Overlays))
	for _, overlay := range snapshot.Overlays {
		overlays = append(overlays, SkillOverlayResponse{
			OverlayID:  overlay.OverlayID,
			Source:     string(overlay.Source),
			Path:       overlay.Path,
			SizeBytes:  overlay.SizeBytes,
			ModifiedAt: overlay.ModifiedAt,
		})
	}
	return SkillRegistryResponse{
		LoadedAt: snapshot.LoadedAt,
		Items:    items,
		Overlays: overlays,
	}
}

func buildSkillSummaryResponse(skill skills.Skill) SkillSummaryResponse {
	files := make([]SkillFileResponse, 0, len(skill.Files))
	for _, file := range skill.Files {
		files = append(files, SkillFileResponse{
			Path:      file.Path,
			SizeBytes: file.SizeBytes,
		})
	}
	return SkillSummaryResponse{
		SkillID:            skill.SkillID,
		Name:               skill.Name,
		Description:        skill.Description,
		Source:             string(skill.Source),
		RootPath:           skill.RootPath,
		SkillPath:          skill.SkillPath,
		InstructionPath:    skill.InstructionPath,
		Files:              files,
		Frontmatter:        skill.Frontmatter,
		ExecutionManifest:  cloneExecutableManifest(skill.ExecutionManifest),
		AvailabilityStatus: string(skill.AvailabilityStatus),
		AvailabilityReason: skill.AvailabilityReason,
		Sandbox:            cloneSandboxConsumerView(skill.Sandbox),
	}
}

func buildSkillDetailResponse(skill skills.Skill) SkillDetailResponse {
	return SkillDetailResponse{
		SkillSummaryResponse: buildSkillSummaryResponse(skill),
		FrontmatterRaw:       skill.FrontmatterRaw,
		Body:                 skill.Body,
	}
}

func buildManagedProviderConfigSandbox(providerID, workDir string) map[string]any {
	readRoots := []string{}
	if strings.TrimSpace(workDir) != "" {
		readRoots = []string{strings.TrimSpace(workDir)}
	}
	view := &sandbox.ConsumerContractView{
		Declaration: &sandbox.ConsumerRequirementDeclaration{
			DeclarationID:               "managed_provider:" + strings.TrimSpace(providerID) + ":config",
			ConsumerKind:                sandbox.ConsumerKindManagedProvider,
			ConsumerID:                  strings.TrimSpace(providerID),
			OperationKind:               "config_inspect",
			ProfileID:                   sandbox.ProfileIDSubprocessDefault,
			ExecutionMode:               sandbox.ExecutionModeDeclarationOnly,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   readRoots,
			WriteRoots:                  []string{},
			NetworkMode:                 sandbox.NetworkModeDeny,
			SecretRefs:                  []string{},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
			Source:                      sandbox.SourceBuiltin,
		},
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return nil
	}
	var item map[string]any
	if err := json.Unmarshal(payload, &item); err != nil {
		return nil
	}
	return item
}

func cloneSandboxConsumerView(view map[string]any) map[string]any {
	if view == nil {
		return nil
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return view
	}
	var cloned map[string]any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return view
	}
	return cloned
}

func cloneSandboxConsumerViews(items []map[string]any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneSandboxConsumerView(item))
	}
	return cloned
}

func cloneExecutableManifest(manifest *skills.ExecutableManifest) *skills.ExecutableManifest {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	cloned.Args = append([]string(nil), manifest.Args...)
	cloned.ReadRoots = append([]string(nil), manifest.ReadRoots...)
	cloned.WriteRoots = append([]string(nil), manifest.WriteRoots...)
	cloned.AllowedHosts = append([]string(nil), manifest.AllowedHosts...)
	cloned.AllowedPorts = append([]int(nil), manifest.AllowedPorts...)
	cloned.SecretRefs = append([]string(nil), manifest.SecretRefs...)
	return &cloned
}

func effectiveEnvironment(cfg config.Config) string {
	switch cfg.Environment {
	case config.EnvironmentProd, config.EnvironmentTest:
		return string(cfg.Environment)
	default:
		return string(config.EnvironmentTest)
	}
}

func buildEventListResponse(items []events.Event) EventListResponse {
	response := EventListResponse{
		Items: items,
	}
	if len(items) > 0 {
		response.NextCursor = items[len(items)-1].Sequence
	}
	return response
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string(nil), items...)
}
