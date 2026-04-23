package calendar

import "time"

type OperationClass string

const (
	OperationClassListEvents  OperationClass = "list_events"
	OperationClassGetEvent    OperationClass = "get_event"
	OperationClassBusyFree    OperationClass = "busy_free"
	OperationClassCreateEvent OperationClass = "create_event"
	OperationClassUpdateEvent OperationClass = "update_event"
	OperationClassCancelEvent OperationClass = "cancel_event"
)

type OperationStatus string

const (
	OperationStatusRequested OperationStatus = "requested"
	OperationStatusCompleted OperationStatus = "completed"
	OperationStatusFailed    OperationStatus = "failed"
	OperationStatusBlocked   OperationStatus = "blocked"
	OperationStatusCancelled OperationStatus = "cancelled"
)

type ArtifactKind string

const (
	ArtifactKindEventSnapshot     ArtifactKind = "event_snapshot"
	ArtifactKindAvailabilityQuery ArtifactKind = "availability_query"
)

type EventLifecycleState string

const (
	EventLifecycleStateActive        EventLifecycleState = "active"
	EventLifecycleStateCancelled     EventLifecycleState = "cancelled"
	EventLifecycleStateStaleSnapshot EventLifecycleState = "stale_snapshot"
	EventLifecycleStateUnavailable   EventLifecycleState = "unavailable"
)

type AccountProjection struct {
	CalendarAccountID       string    `json:"calendarAccountId"`
	IntegrationID           string    `json:"integrationId"`
	DomainKind              string    `json:"domainKind"`
	EnvironmentScope        string    `json:"environmentScope"`
	AccountKey              string    `json:"accountKey,omitempty"`
	AccountLabel            string    `json:"accountLabel,omitempty"`
	ReadinessStatus         string    `json:"readinessStatus"`
	CanonicalDefault        bool      `json:"canonicalDefault"`
	SelectionMode           string    `json:"selectionMode,omitempty"`
	PrimaryCalendarRef      string    `json:"primaryCalendarRef"`
	PrimaryCalendarLabel    string    `json:"primaryCalendarLabel"`
	PrimaryTimezone         string    `json:"primaryTimezone"`
	SupportsEventInspection bool      `json:"supportsEventInspection"`
	SupportsBusyFree        bool      `json:"supportsBusyFree"`
	SupportsTimedMutation   bool      `json:"supportsTimedMutation"`
	LastSyncedAt            time.Time `json:"lastSyncedAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type Event struct {
	ExternalEventID         string              `json:"externalEventId"`
	IntegrationID           string              `json:"integrationId"`
	CalendarAccountID       string              `json:"calendarAccountId"`
	CalendarRef             string              `json:"calendarRef"`
	Title                   string              `json:"title"`
	Description             string              `json:"description,omitempty"`
	Location                string              `json:"location,omitempty"`
	StartsAt                time.Time           `json:"startsAt"`
	EndsAt                  time.Time           `json:"endsAt"`
	Timezone                string              `json:"timezone"`
	AllDay                  bool                `json:"allDay,omitempty"`
	Recurring               bool                `json:"recurring,omitempty"`
	RecurrenceSummary       string              `json:"recurrenceSummary,omitempty"`
	Attendees               []string            `json:"attendees,omitempty"`
	MutationEligibleInPhase bool                `json:"mutationEligibleInPhase"`
	LifecycleState          EventLifecycleState `json:"lifecycleState"`
	CreatedAt               time.Time           `json:"createdAt"`
	UpdatedAt               time.Time           `json:"updatedAt"`
	CancelledAt             *time.Time          `json:"cancelledAt,omitempty"`
}

type BusyInterval struct {
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}

type AvailabilityQuery struct {
	QueryID           string         `json:"queryId"`
	OperationID       string         `json:"operationId"`
	IntegrationID     string         `json:"integrationId"`
	CalendarAccountID string         `json:"calendarAccountId"`
	WindowStart       time.Time      `json:"windowStart"`
	WindowEnd         time.Time      `json:"windowEnd"`
	Timezone          string         `json:"timezone"`
	BusyIntervals     []BusyInterval `json:"busyIntervals,omitempty"`
	ConflictCount     int            `json:"conflictCount"`
	ResultSummary     string         `json:"resultSummary"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type Artifact struct {
	ArtifactID              string              `json:"artifactId"`
	OperationID             string              `json:"operationId"`
	Kind                    ArtifactKind        `json:"kind"`
	IntegrationID           string              `json:"integrationId"`
	EnvironmentScope        string              `json:"environmentScope"`
	ExternalEventID         string              `json:"externalEventId,omitempty"`
	CalendarRef             string              `json:"calendarRef,omitempty"`
	Title                   string              `json:"title,omitempty"`
	StartsAt                *time.Time          `json:"startsAt,omitempty"`
	EndsAt                  *time.Time          `json:"endsAt,omitempty"`
	Timezone                string              `json:"timezone,omitempty"`
	AllDay                  bool                `json:"allDay,omitempty"`
	RecurrenceSummary       string              `json:"recurrenceSummary,omitempty"`
	MutationEligibleInPhase bool                `json:"mutationEligibleInPhase,omitempty"`
	LifecycleState          EventLifecycleState `json:"lifecycleState,omitempty"`
	AvailabilityQuery       *AvailabilityQuery  `json:"availabilityQuery,omitempty"`
	CreatedAt               time.Time           `json:"createdAt"`
}

type Operation struct {
	OperationID         string          `json:"operationId"`
	OperationClass      OperationClass  `json:"operationClass"`
	Status              OperationStatus `json:"status"`
	IntegrationID       string          `json:"integrationId"`
	CalendarAccountID   string          `json:"calendarAccountId"`
	EnvironmentScope    string          `json:"environmentScope"`
	CalendarRef         string          `json:"calendarRef,omitempty"`
	SelectionMode       string          `json:"selectionMode,omitempty"`
	TimezoneUsed        string          `json:"timezoneUsed,omitempty"`
	RequestSummary      string          `json:"requestSummary,omitempty"`
	ExternalEventID     string          `json:"externalEventId,omitempty"`
	FailureClass        string          `json:"failureClass,omitempty"`
	FailureReason       string          `json:"failureReason,omitempty"`
	RunID               string          `json:"runId,omitempty"`
	StepID              string          `json:"stepId,omitempty"`
	ToolCallID          string          `json:"toolCallId,omitempty"`
	WorkflowID          string          `json:"workflowId,omitempty"`
	WorkflowStepID      string          `json:"workflowStepId,omitempty"`
	ScheduleID          string          `json:"scheduleId,omitempty"`
	ScheduleAttemptID   string          `json:"scheduleAttemptId,omitempty"`
	DeliveryID          string          `json:"deliveryId,omitempty"`
	ArtifactIDs         []string        `json:"artifactIds,omitempty"`
	AvailabilityQueryID string          `json:"availabilityQueryId,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	CompletedAt         *time.Time      `json:"completedAt,omitempty"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

type OperationSummary struct {
	OperationID     string          `json:"operationId"`
	OperationClass  OperationClass  `json:"operationClass"`
	IntegrationID   string          `json:"integrationId"`
	ExternalEventID string          `json:"externalEventId,omitempty"`
	Status          OperationStatus `json:"status"`
	TimezoneUsed    string          `json:"timezoneUsed,omitempty"`
	CapturedAt      time.Time       `json:"capturedAt"`
}

type Selection struct {
	IntegrationID string
}

type SourceLinkage struct {
	RunID             string
	StepID            string
	ToolCallID        string
	WorkflowID        string
	WorkflowStepID    string
	ScheduleID        string
	ScheduleAttemptID string
	DeliveryID        string
}

type Action struct {
	OperationClass  OperationClass `json:"operationClass"`
	IntegrationID   string         `json:"integrationId,omitempty"`
	ExternalEventID string         `json:"externalEventId,omitempty"`
	WindowStart     *time.Time     `json:"windowStart,omitempty"`
	WindowEnd       *time.Time     `json:"windowEnd,omitempty"`
	Title           string         `json:"title,omitempty"`
	Description     string         `json:"description,omitempty"`
	Location        string         `json:"location,omitempty"`
	StartsAt        *time.Time     `json:"startsAt,omitempty"`
	EndsAt          *time.Time     `json:"endsAt,omitempty"`
	Timezone        string         `json:"timezone,omitempty"`
	CalendarRef     string         `json:"calendarRef,omitempty"`
	AllDay          bool           `json:"allDay,omitempty"`
	Recurring       bool           `json:"recurring,omitempty"`
	Attendees       []string       `json:"attendees,omitempty"`
	Reason          string         `json:"reason,omitempty"`
}

type ListEventsInput struct {
	Selection Selection
	StartsAt  *time.Time
	EndsAt    *time.Time
	Source    SourceLinkage
}

type GetEventInput struct {
	Selection       Selection
	ExternalEventID string
	Source          SourceLinkage
}

type BusyFreeInput struct {
	Selection   Selection
	WindowStart time.Time
	WindowEnd   time.Time
	Timezone    string
	Source      SourceLinkage
}

type CreateEventInput struct {
	Selection   Selection
	Title       string
	Description string
	Location    string
	StartsAt    time.Time
	EndsAt      time.Time
	Timezone    string
	AllDay      bool
	Recurring   bool
	Attendees   []string
	Source      SourceLinkage
}

type UpdateEventInput struct {
	Selection       Selection
	ExternalEventID string
	Title           string
	Description     string
	Location        string
	StartsAt        time.Time
	EndsAt          time.Time
	Timezone        string
	AllDay          bool
	Recurring       bool
	Attendees       []string
	Source          SourceLinkage
}

type CancelEventInput struct {
	Selection       Selection
	ExternalEventID string
	Reason          string
	Source          SourceLinkage
}

type OperationFilter struct {
	IntegrationID   string
	RunID           string
	WorkflowID      string
	ScheduleID      string
	DeliveryID      string
	OperationClass  OperationClass
	Status          OperationStatus
	ExternalEventID string
}

func CloneOperationSummaries(items []OperationSummary) []OperationSummary {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]OperationSummary, len(items))
	copy(cloned, items)
	return cloned
}
