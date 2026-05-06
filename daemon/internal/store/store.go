package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
	_ "modernc.org/sqlite"
)

const (
	defaultDatabaseFile  = "daemon.sqlite"
	CurrentSchemaVersion = 40
)

func (s *SQLiteStore) ResolveActiveTenantBinding(ctx context.Context) any {
	if tenantContext, ok := tenantctx.FromContext(ctx); ok && strings.TrimSpace(tenantContext.TenantID) != "" {
		return strings.TrimSpace(tenantContext.TenantID)
	}
	return s.ResolveDefaultTenantBinding(ctx)
}

type schemaMigration struct {
	Version    int
	Name       string
	Statements []string
}

type SandboxExecutionRecord struct {
	ExecutionID string
	ProfileID   string
	BackendKind string
	Status      string
	ApprovalID  string
	RequestedAt time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Document    []byte
}

type ConsumerPolicyRecordRecord struct {
	PolicyRecordID      string
	ConsumerKind        string
	ConsumerID          string
	OperationKind       string
	DeclarationID       string
	Status              string
	Decision            string
	ApprovalStatus      string
	SecretResolution    string
	RequestedBy         string
	SandboxExecutionID  string
	ToolCallID          string
	ProviderOperationID string
	StartedAt           time.Time
	CompletedAt         *time.Time
	Document            []byte
}

type SecretScopeBindingRecord struct {
	BindingID        string
	ConsumerKind     string
	ConsumerID       string
	EnvironmentScope string
	SecretRef        string
	DefaultSource    string
	DeliveryKind     string
	Active           bool
	Document         []byte
}

type MCPServerRecord struct {
	ServerID  string
	Enabled   bool
	UpdatedAt time.Time
	Document  []byte
}

type MCPServerStateRecord struct {
	ServerID  string
	Status    string
	UpdatedAt time.Time
	Document  []byte
}

type MCPToolRecord struct {
	ServerID         string
	ToolName         string
	DiscoveryStatus  string
	UpdatedAt        time.Time
	LastDiscoveredAt *time.Time
	Document         []byte
}

type MCPToolExposureRuleRecord struct {
	ServerID       string
	ToolName       string
	RuntimeSurface string
	ExposureMode   string
	Active         bool
	UpdatedAt      time.Time
	Document       []byte
}

type WorkflowRecord struct {
	WorkflowID        string
	RunID             string
	ScheduleID        string
	ScheduleAttemptID string
	EnvironmentScope  string
	Goal              string
	Status            string
	PlanSummary       string
	FailureSummary    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	InterruptedAt     *time.Time
	Document          []byte
}

type IntegrationRecord struct {
	IntegrationID    string
	DomainKind       string
	EnvironmentScope string
	AccountKey       string
	BackendKind      string
	ReadinessStatus  string
	CanonicalDefault bool
	UpdatedAt        time.Time
	Document         []byte
}

type CalendarAccountRecord struct {
	CalendarAccountID string
	IntegrationID     string
	EnvironmentScope  string
	AccountKey        string
	ReadinessStatus   string
	CanonicalDefault  bool
	UpdatedAt         time.Time
	Document          []byte
}

type CalendarOperationRecord struct {
	OperationID       string
	IntegrationID     string
	CalendarAccountID string
	EnvironmentScope  string
	OperationClass    string
	Status            string
	ExternalEventID   string
	RunID             string
	WorkflowID        string
	ScheduleID        string
	DeliveryID        string
	UpdatedAt         time.Time
	Document          []byte
}

type CalendarArtifactRecord struct {
	ArtifactID       string
	OperationID      string
	IntegrationID    string
	EnvironmentScope string
	Kind             string
	ExternalEventID  string
	CreatedAt        time.Time
	Document         []byte
}

type CalendarOperationFilter struct {
	IntegrationID   string
	RunID           string
	WorkflowID      string
	ScheduleID      string
	DeliveryID      string
	OperationClass  string
	Status          string
	ExternalEventID string
}

type MailAccountRecord struct {
	MailAccountID    string
	IntegrationID    string
	EnvironmentScope string
	AccountKey       string
	ReadinessStatus  string
	CanonicalDefault bool
	UpdatedAt        time.Time
	Document         []byte
}

type MailOperationRecord struct {
	OperationID      string
	IntegrationID    string
	MailAccountID    string
	EnvironmentScope string
	OperationClass   string
	Status           string
	ResultMode       string
	ThreadID         string
	MessageID        string
	DraftID          string
	RunID            string
	WorkflowID       string
	ScheduleID       string
	DeliveryID       string
	UpdatedAt        time.Time
	Document         []byte
}

type MailArtifactRecord struct {
	ArtifactID       string
	OperationID      string
	IntegrationID    string
	EnvironmentScope string
	Kind             string
	ThreadID         string
	MessageID        string
	DraftID          string
	AttachmentRefID  string
	CreatedAt        time.Time
	Document         []byte
}

type MailOperationFilter struct {
	IntegrationID  string
	RunID          string
	WorkflowID     string
	ScheduleID     string
	DeliveryID     string
	OperationClass string
	Status         string
	ResultMode     string
	ThreadID       string
	MessageID      string
	DraftID        string
}

type ReminderRecord struct {
	ReminderID         string
	EnvironmentScope   string
	TenantID           string
	BehaviorMode       string
	CurrentState       string
	NextDueAt          *time.Time
	ActiveOccurrenceID string
	UpdatedAt          time.Time
	Document           []byte
}

type ReminderOccurrenceRecord struct {
	OccurrenceID         string
	ReminderID           string
	EnvironmentScope     string
	State                string
	ScheduledFor         time.Time
	RunID                string
	WorkflowID           string
	LatestDeliveryID     string
	LatestDeliveryStatus string
	UpdatedAt            time.Time
	Document             []byte
}

type ReminderActionRecord struct {
	ActionID     string
	ReminderID   string
	OccurrenceID string
	ActionKind   string
	NewState     string
	RunID        string
	WorkflowID   string
	DeliveryID   string
	CreatedAt    time.Time
	Document     []byte
}

type ReminderOccurrenceFilter struct {
	ReminderID      string
	State           string
	RunID           string
	WorkflowID      string
	DeliveryID      string
	ScheduledBefore *time.Time
	ScheduledAfter  *time.Time
}

type WorkflowStepRecord struct {
	WorkflowStepID   string
	WorkflowID       string
	Position         int
	Status           string
	RuntimeStepID    string
	ActiveToolCallID string
	AttemptCount     int
	MaxAttempts      int
	LastFailureClass string
	BlockedReason    string
	Document         []byte
}

type WorkflowDependencyRecord struct {
	DependencyID string
	WorkflowID   string
	Document     []byte
}

type WorkflowHandoffRecord struct {
	HandoffID  string
	WorkflowID string
	Status     string
	Document   []byte
}

type ScheduleRecord struct {
	ScheduleID       string
	EnvironmentScope string
	TenantID         string
	Kind             string
	Status           string
	TargetRefID      string
	Timezone         string
	NextDueAt        *time.Time
	LastAttemptAt    *time.Time
	LastOutcome      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	PausedAt         *time.Time
	CancelledAt      *time.Time
	CompletedAt      *time.Time
	Document         []byte
}

type ScheduleTargetRecord struct {
	TargetRefID string
	ScheduleID  string
	TargetKind  string
	Revision    int
	Active      bool
	UpdatedAt   time.Time
	Document    []byte
}

type ScheduleDispatchAttemptRecord struct {
	AttemptID              string
	ScheduleID             string
	DueAt                  time.Time
	TriggerSource          string
	DispatchStatus         string
	FailureClass           string
	FailureReason          string
	RetryCount             int
	RetryBudget            int
	NextRetryAt            *time.Time
	ResolvedTargetRevision int
	RunID                  string
	WorkflowID             string
	DownstreamStatus       string
	SkippedReason          string
	MissedCount            int
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Document               []byte
}

type DeliveryTargetRecord struct {
	TargetID         string
	EnvironmentScope string
	TargetKind       string
	Status           string
	UpdatedAt        time.Time
	Document         []byte
}

type DeliveryPreferenceRecord struct {
	PreferenceID     string
	EnvironmentScope string
	ScopeKind        string
	IntegrationID    string
	Active           bool
	UpdatedAt        time.Time
	Document         []byte
}

type DeliveryOutcomeRecord struct {
	DeliveryID       string
	EnvironmentScope string
	SourceKind       string
	SourceID         string
	RunID            string
	WorkflowID       string
	ScheduleID       string
	IntegrationID    string
	Status           string
	ChosenTargetID   string
	PreferenceID     string
	SummaryWindowID  string
	UpdatedAt        time.Time
	Document         []byte
}

type DeliveryAttemptRecord struct {
	AttemptID     string
	DeliveryID    string
	AttemptNumber int
	TargetID      string
	Status        string
	NextRetryAt   *time.Time
	Document      []byte
}

type DeliverySummaryWindowRecord struct {
	SummaryWindowID  string
	EnvironmentScope string
	TargetID         string
	PreferenceID     string
	Status           string
	WindowEndsAt     time.Time
	UpdatedAt        time.Time
	Document         []byte
}

type DeliveryOutcomeFilter struct {
	SourceKind    string
	SourceID      string
	RunID         string
	WorkflowID    string
	ScheduleID    string
	IntegrationID string
	Status        string
	TargetID      string
}

var schemaMigrations = []schemaMigration{
	{
		Version: 1,
		Name:    "baseline",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS sessions (
				session_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				channel TEXT NOT NULL,
				account_id TEXT,
				peer_id TEXT NOT NULL,
				thread_id TEXT,
				routing_key TEXT NOT NULL UNIQUE,
				generation INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_active_at TEXT NOT NULL,
				last_reset_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS auth_pairings (
				pairing_id TEXT PRIMARY KEY,
				mode TEXT NOT NULL,
				label TEXT NOT NULL,
				status TEXT NOT NULL,
				code_hash TEXT NOT NULL,
				code_preview TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				expires_at TEXT NOT NULL,
				completed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS auth_tokens (
				token_id TEXT PRIMARY KEY,
				label TEXT NOT NULL,
				mode TEXT NOT NULL,
				token_hash TEXT NOT NULL,
				token_preview TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_used_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS approvals (
				approval_id TEXT PRIMARY KEY,
				action TEXT NOT NULL,
				resource_kind TEXT,
				resource_id TEXT,
				reason TEXT NOT NULL,
				requested_by TEXT,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				resolved_at TEXT,
				resolution TEXT,
				comment TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS decisions (
				decision_id TEXT PRIMARY KEY,
				action TEXT NOT NULL,
				resource_kind TEXT,
				resource_id TEXT,
				outcome TEXT NOT NULL,
				reason TEXT NOT NULL,
				approval_id TEXT,
				created_at TEXT NOT NULL,
				FOREIGN KEY(approval_id) REFERENCES approvals(approval_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS runs (
				run_id TEXT PRIMARY KEY,
				session_id TEXT,
				entrypoint TEXT NOT NULL,
				status TEXT NOT NULL,
				goal TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS connectors (
				connector_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				failure_count INTEGER NOT NULL,
				restart_count INTEGER NOT NULL,
				backoff_seconds INTEGER NOT NULL,
				next_restart_at TEXT,
				last_restart_at TEXT,
				last_heartbeat_at TEXT,
				last_failure_reason TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS capabilities (
				capability_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				failure_count INTEGER NOT NULL,
				restart_count INTEGER NOT NULL,
				backoff_seconds INTEGER NOT NULL,
				next_restart_at TEXT,
				last_restart_at TEXT,
				last_heartbeat_at TEXT,
				last_failure_reason TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS steps (
				step_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				title TEXT NOT NULL,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				input_json TEXT,
				output_json TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS tool_calls (
				tool_call_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				step_id TEXT NOT NULL,
				capability_id TEXT NOT NULL,
				tool_name TEXT NOT NULL,
				status TEXT NOT NULL,
				input_json TEXT,
				output_json TEXT,
				error_text TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE,
				FOREIGN KEY(step_id) REFERENCES steps(step_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS llm_dispatches (
				dispatch_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				model TEXT NOT NULL,
				messages_json TEXT NOT NULL,
				stream INTEGER NOT NULL,
				status TEXT NOT NULL,
				output_text TEXT NOT NULL,
				finish_reason TEXT,
				usage_json TEXT NOT NULL,
				error_code TEXT,
				error_text TEXT,
				timeout_ms INTEGER NOT NULL,
				max_retries INTEGER NOT NULL,
				attempt_count INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS events (
				event_id TEXT PRIMARY KEY,
				category TEXT NOT NULL,
				name TEXT NOT NULL,
				occurred_at TEXT NOT NULL,
				session_id TEXT,
				run_id TEXT,
				step_id TEXT,
				connector_id TEXT,
				capability_id TEXT,
				resource_kind TEXT NOT NULL,
				resource_id TEXT NOT NULL,
				payload_json TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS checkpoints (
				checkpoint_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				captured_at TEXT NOT NULL,
				snapshot_json TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_steps_run_id ON steps(run_id);`,
			`CREATE INDEX IF NOT EXISTS idx_connectors_kind_status ON connectors(kind, status);`,
			`CREATE INDEX IF NOT EXISTS idx_capabilities_kind_status ON capabilities(kind, status);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_run_step ON tool_calls(run_id, step_id, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_llm_dispatches_provider_status ON llm_dispatches(provider, status, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_approvals_status_created ON approvals(status, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_channel_peer ON sessions(channel, peer_id, thread_id);`,
			`CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_category ON events(category, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id ON checkpoints(run_id, captured_at);`,
		},
	},
	{
		Version: 2,
		Name:    "operational_indexes",
		Statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_events_resource_scope ON events(resource_kind, resource_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_capability_created ON tool_calls(capability_id, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_tokens_last_used_at ON auth_tokens(last_used_at);`,
			`CREATE INDEX IF NOT EXISTS idx_decisions_approval_id ON decisions(approval_id, created_at);`,
		},
	},
	{
		Version: 3,
		Name:    "provider_checks",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS provider_checks (
				check_id TEXT PRIMARY KEY,
				provider_id TEXT NOT NULL,
				family TEXT NOT NULL,
				auth_mode TEXT NOT NULL,
				status TEXT NOT NULL,
				model TEXT NOT NULL,
				endpoint TEXT,
				error_class TEXT,
				error_code TEXT,
				error_message TEXT,
				usage_json TEXT NOT NULL,
				created_at TEXT NOT NULL,
				completed_at TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_provider_checks_provider_created ON provider_checks(provider_id, created_at DESC, check_id DESC);`,
		},
	},
	{
		Version: 4,
		Name:    "managed_provider_state",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS provider_auth_states (
				provider_id TEXT PRIMARY KEY,
				family TEXT NOT NULL,
				auth_mode TEXT NOT NULL,
				status TEXT NOT NULL,
				cli_path TEXT,
				cli_available INTEGER NOT NULL,
				account_label TEXT,
				account_id TEXT,
				plan TEXT,
				auth_method TEXT,
				login_command_json TEXT NOT NULL,
				logout_command_json TEXT NOT NULL,
				last_checked_at TEXT NOT NULL,
				last_authenticated_at TEXT,
				last_error TEXT,
				metadata_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS provider_models (
				provider_id TEXT NOT NULL,
				model_id TEXT NOT NULL,
				display_name TEXT NOT NULL,
				description TEXT,
				default_flag INTEGER NOT NULL,
				available_flag INTEGER NOT NULL,
				source TEXT NOT NULL,
				chat INTEGER NOT NULL,
				stream INTEGER NOT NULL,
				coding INTEGER NOT NULL,
				tool_use INTEGER NOT NULL,
				reasoning_levels_json TEXT NOT NULL,
				PRIMARY KEY (provider_id, model_id)
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS provider_preferences (
				provider_id TEXT PRIMARY KEY,
				default_model TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_provider_models_provider ON provider_models(provider_id, model_id);`,
		},
	},
	{
		Version: 5,
		Name:    "connector_messages",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS connector_messages (
				delivery_id TEXT PRIMARY KEY,
				connector_id TEXT NOT NULL,
				direction TEXT NOT NULL,
				external_message_id TEXT,
				session_id TEXT,
				run_id TEXT,
				channel_id TEXT NOT NULL,
				peer_id TEXT,
				thread_id TEXT,
				author_id TEXT,
				content TEXT NOT NULL,
				status TEXT NOT NULL,
				error_text TEXT,
				reply_to_external_message_id TEXT,
				response_to_delivery_id TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE SET NULL
			);
			`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_connector_messages_external ON connector_messages(connector_id, direction, external_message_id) WHERE external_message_id IS NOT NULL;`,
			`CREATE INDEX IF NOT EXISTS idx_connector_messages_connector_created ON connector_messages(connector_id, created_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_connector_messages_session_created ON connector_messages(session_id, created_at DESC, delivery_id DESC);`,
		},
	},
	{
		Version: 6,
		Name:    "sandbox_execution_plane",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS sandbox_executions (
				execution_id TEXT PRIMARY KEY,
				profile_id TEXT NOT NULL,
				backend_kind TEXT NOT NULL,
				status TEXT NOT NULL,
				approval_id TEXT,
				requested_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(approval_id) REFERENCES approvals(approval_id) ON DELETE SET NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_sandbox_executions_status_requested ON sandbox_executions(status, requested_at DESC, execution_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_sandbox_executions_profile_requested ON sandbox_executions(profile_id, requested_at DESC, execution_id DESC);`,
		},
	},
	{
		Version: 7,
		Name:    "sandbox_requirement_contract",
		Statements: []string{
			`ALTER TABLE provider_auth_states ADD COLUMN sandbox_json TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN sandbox_json TEXT;`,
			`
			CREATE TABLE IF NOT EXISTS consumer_policy_records (
				policy_record_id TEXT PRIMARY KEY,
				consumer_kind TEXT NOT NULL,
				consumer_id TEXT NOT NULL,
				operation_kind TEXT NOT NULL,
				declaration_id TEXT,
				status TEXT NOT NULL,
				decision TEXT NOT NULL,
				approval_status TEXT NOT NULL,
				secret_resolution TEXT NOT NULL,
				requested_by TEXT,
				sandbox_execution_id TEXT,
				tool_call_id TEXT,
				provider_operation_id TEXT,
				started_at TEXT NOT NULL,
				completed_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS secret_scope_bindings (
				binding_id TEXT PRIMARY KEY,
				consumer_kind TEXT NOT NULL,
				consumer_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				secret_ref TEXT NOT NULL,
				default_source TEXT NOT NULL,
				delivery_kind TEXT NOT NULL,
				active INTEGER NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_policy_records_consumer_started ON consumer_policy_records(consumer_kind, consumer_id, started_at DESC, policy_record_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_policy_records_status_started ON consumer_policy_records(status, started_at DESC, policy_record_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_secret_scope_bindings_consumer_secret ON secret_scope_bindings(consumer_kind, consumer_id, secret_ref);`,
		},
	},
	{
		Version: 8,
		Name:    "mcp_execution_plane",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS mcp_servers (
				server_id TEXT PRIMARY KEY,
				enabled INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS mcp_server_states (
				server_id TEXT PRIMARY KEY,
				status TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(server_id) REFERENCES mcp_servers(server_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS mcp_tools (
				server_id TEXT NOT NULL,
				tool_name TEXT NOT NULL,
				discovery_status TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_discovered_at TEXT,
				document_json TEXT NOT NULL,
				PRIMARY KEY (server_id, tool_name),
				FOREIGN KEY(server_id) REFERENCES mcp_servers(server_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS mcp_tool_exposure_rules (
				server_id TEXT NOT NULL,
				tool_name TEXT NOT NULL,
				runtime_surface TEXT NOT NULL,
				exposure_mode TEXT NOT NULL,
				active INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				PRIMARY KEY (server_id, tool_name, runtime_surface),
				FOREIGN KEY(server_id, tool_name) REFERENCES mcp_tools(server_id, tool_name) ON DELETE CASCADE
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_servers_enabled ON mcp_servers(enabled, updated_at DESC, server_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_server_states_status ON mcp_server_states(status, updated_at DESC, server_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_tools_server_status ON mcp_tools(server_id, discovery_status, tool_name);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_tool_exposure_surface ON mcp_tool_exposure_rules(runtime_surface, exposure_mode, server_id, tool_name);`,
		},
	},
	{
		Version: 9,
		Name:    "skill_tool_sandbox_execution",
		Statements: []string{
			`ALTER TABLE tool_calls ADD COLUMN invocation_kind TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN skill_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN sandbox_execution_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN failure_class TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_run_step_created ON tool_calls(run_id, step_id, created_at DESC, tool_call_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_skill_created ON tool_calls(skill_id, created_at DESC, tool_call_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_sandbox_execution ON tool_calls(sandbox_execution_id);`,
		},
	},
	{
		Version: 10,
		Name:    "mcp_runtime_catalog",
		Statements: []string{
			`ALTER TABLE tool_calls ADD COLUMN mcp_server_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN mcp_server_name TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN mcp_tool_name TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN mcp_transport_kind TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN mcp_session_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN authorization_result TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_mcp_server_created ON tool_calls(mcp_server_id, created_at DESC, tool_call_id DESC);`,
		},
	},
	{
		Version: 11,
		Name:    "workflow_orchestration",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS workflows (
				workflow_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				goal TEXT NOT NULL,
				status TEXT NOT NULL,
				plan_summary TEXT,
				failure_summary TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT,
				interrupted_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS workflow_steps (
				workflow_step_id TEXT PRIMARY KEY,
				workflow_id TEXT NOT NULL,
				position INTEGER NOT NULL,
				status TEXT NOT NULL,
				runtime_step_id TEXT,
				active_tool_call_id TEXT,
				attempt_count INTEGER NOT NULL,
				max_attempts INTEGER NOT NULL,
				last_failure_class TEXT,
				blocked_reason TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS workflow_dependencies (
				dependency_id TEXT PRIMARY KEY,
				workflow_id TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS workflow_handoffs (
				handoff_id TEXT PRIMARY KEY,
				workflow_id TEXT NOT NULL,
				status TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
			);
			`,
			`ALTER TABLE steps ADD COLUMN workflow_id TEXT;`,
			`ALTER TABLE steps ADD COLUMN workflow_step_id TEXT;`,
			`ALTER TABLE steps ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;`,
			`ALTER TABLE tool_calls ADD COLUMN workflow_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN workflow_step_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_run_env_created ON workflows(run_id, environment_scope, created_at DESC, workflow_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_position ON workflow_steps(workflow_id, position ASC, workflow_step_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_steps_workflow_linkage ON steps(workflow_id, workflow_step_id, attempt);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_workflow_linkage ON tool_calls(workflow_id, workflow_step_id, attempt, created_at DESC, tool_call_id DESC);`,
		},
	},
	{
		Version: 12,
		Name:    "scheduled_tasks_wakeups",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS schedules (
				schedule_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				target_ref_id TEXT NOT NULL,
				timezone TEXT,
				next_due_at TEXT,
				last_attempt_at TEXT,
				last_outcome TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				paused_at TEXT,
				cancelled_at TEXT,
				completed_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS schedule_targets (
				target_ref_id TEXT PRIMARY KEY,
				schedule_id TEXT NOT NULL,
				target_kind TEXT NOT NULL,
				revision INTEGER NOT NULL,
				active INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(schedule_id) REFERENCES schedules(schedule_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS schedule_dispatch_attempts (
				attempt_id TEXT PRIMARY KEY,
				schedule_id TEXT NOT NULL,
				due_at TEXT NOT NULL,
				trigger_source TEXT NOT NULL,
				dispatch_status TEXT NOT NULL,
				failure_class TEXT,
				failure_reason TEXT,
				retry_count INTEGER NOT NULL,
				retry_budget INTEGER NOT NULL,
				next_retry_at TEXT,
				resolved_target_revision INTEGER NOT NULL,
				run_id TEXT,
				workflow_id TEXT,
				downstream_status TEXT NOT NULL,
				skipped_reason TEXT,
				missed_count INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(schedule_id) REFERENCES schedules(schedule_id) ON DELETE CASCADE
			);
			`,
			`ALTER TABLE runs ADD COLUMN schedule_id TEXT;`,
			`ALTER TABLE runs ADD COLUMN schedule_attempt_id TEXT;`,
			`ALTER TABLE workflows ADD COLUMN schedule_id TEXT;`,
			`ALTER TABLE workflows ADD COLUMN schedule_attempt_id TEXT;`,
			`ALTER TABLE events ADD COLUMN workflow_id TEXT;`,
			`ALTER TABLE events ADD COLUMN workflow_step_id TEXT;`,
			`ALTER TABLE events ADD COLUMN schedule_id TEXT;`,
			`ALTER TABLE events ADD COLUMN schedule_attempt_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_schedules_env_status_due ON schedules(environment_scope, status, next_due_at, schedule_id);`,
			`CREATE INDEX IF NOT EXISTS idx_schedule_targets_schedule ON schedule_targets(schedule_id, updated_at DESC, target_ref_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_schedule_attempts_schedule_due ON schedule_dispatch_attempts(schedule_id, due_at DESC, attempt_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_schedule_attempts_retry_due ON schedule_dispatch_attempts(dispatch_status, next_retry_at, schedule_id);`,
			`CREATE INDEX IF NOT EXISTS idx_runs_schedule_linkage ON runs(schedule_id, schedule_attempt_id, created_at DESC, run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_schedule_linkage ON workflows(schedule_id, schedule_attempt_id, created_at DESC, workflow_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_events_schedule_id ON events(schedule_id, occurred_at);`,
		},
	},
	{
		Version: 13,
		Name:    "event_environment_scope",
		Statements: []string{
			`ALTER TABLE events ADD COLUMN environment_scope TEXT NOT NULL DEFAULT '';`,
			`CREATE INDEX IF NOT EXISTS idx_events_env_category ON events(environment_scope, category, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_env_schedule ON events(environment_scope, schedule_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_env_run ON events(environment_scope, run_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_env_session ON events(environment_scope, session_id, occurred_at);`,
		},
	},
	{
		Version: 14,
		Name:    "computer_use_capability_plane",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS computer_use_sessions (
				computer_use_session_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				run_id TEXT NOT NULL,
				workflow_id TEXT,
				workflow_step_id TEXT,
				status TEXT NOT NULL,
				driver_kind TEXT NOT NULL,
				trusted_page_scope_json TEXT,
				current_page_json TEXT,
				last_action_id TEXT,
				started_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				closed_at TEXT,
				interrupted_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS computer_use_actions (
				computer_use_action_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				computer_use_session_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				step_id TEXT,
				tool_call_id TEXT,
				workflow_id TEXT,
				workflow_step_id TEXT,
				action_kind TEXT NOT NULL,
				status TEXT NOT NULL,
				risk_level TEXT NOT NULL,
				approval_id TEXT,
				target_match_context_json TEXT,
				page_before_json TEXT,
				page_after_json TEXT,
				failure_class TEXT,
				failure_reason TEXT,
				requested_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				completed_at TEXT,
				input_json TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(computer_use_session_id) REFERENCES computer_use_sessions(computer_use_session_id) ON DELETE CASCADE,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS computer_use_artifacts (
				artifact_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				computer_use_session_id TEXT NOT NULL,
				computer_use_action_id TEXT NOT NULL,
				run_id TEXT NOT NULL,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				mime_type TEXT,
				file_name TEXT,
				byte_size INTEGER NOT NULL,
				storage_key TEXT,
				sha256 TEXT,
				capture_failure_reason TEXT,
				created_at TEXT NOT NULL,
				available_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(computer_use_session_id) REFERENCES computer_use_sessions(computer_use_session_id) ON DELETE CASCADE,
				FOREIGN KEY(computer_use_action_id) REFERENCES computer_use_actions(computer_use_action_id) ON DELETE CASCADE,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`ALTER TABLE tool_calls ADD COLUMN computer_use_session_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN computer_use_action_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_sessions_run ON computer_use_sessions(environment_scope, run_id, updated_at DESC, computer_use_session_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_actions_session ON computer_use_actions(environment_scope, computer_use_session_id, requested_at ASC, computer_use_action_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_actions_approval ON computer_use_actions(environment_scope, approval_id, requested_at DESC, computer_use_action_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_artifacts_action ON computer_use_artifacts(environment_scope, computer_use_action_id, created_at ASC, artifact_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_computer_use_session ON tool_calls(computer_use_session_id, created_at DESC, tool_call_id DESC);`,
		},
	},
	{
		Version: 15,
		Name:    "personal_integrations_platform",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS integrations (
				integration_id TEXT PRIMARY KEY,
				domain_kind TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				account_key TEXT,
				backend_kind TEXT NOT NULL,
				readiness_status TEXT NOT NULL,
				canonical_default INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`ALTER TABLE approvals ADD COLUMN integration_bindings_json TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN integration_bindings_json TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_integrations_env_domain_account ON integrations(environment_scope, domain_kind, account_key, canonical_default);`,
			`CREATE INDEX IF NOT EXISTS idx_integrations_readiness ON integrations(environment_scope, readiness_status, updated_at DESC, integration_id DESC);`,
		},
	},
	{
		Version: 16,
		Name:    "delivery_plane",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS delivery_targets (
				target_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				target_kind TEXT NOT NULL,
				status TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS delivery_preferences (
				preference_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				scope_kind TEXT NOT NULL,
				integration_id TEXT,
				active INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS delivery_outcomes (
				delivery_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				source_kind TEXT NOT NULL,
				source_id TEXT NOT NULL,
				run_id TEXT,
				workflow_id TEXT,
				schedule_id TEXT,
				integration_id TEXT,
				status TEXT NOT NULL,
				chosen_target_id TEXT,
				preference_id TEXT,
				summary_window_id TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS delivery_attempts (
				attempt_id TEXT PRIMARY KEY,
				delivery_id TEXT NOT NULL,
				attempt_number INTEGER NOT NULL,
				target_id TEXT NOT NULL,
				status TEXT NOT NULL,
				next_retry_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(delivery_id) REFERENCES delivery_outcomes(delivery_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS delivery_summary_windows (
				summary_window_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				target_id TEXT NOT NULL,
				preference_id TEXT NOT NULL,
				status TEXT NOT NULL,
				window_ends_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_targets_env_status ON delivery_targets(environment_scope, status, updated_at DESC, target_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_preferences_env_scope ON delivery_preferences(environment_scope, scope_kind, integration_id, active, updated_at DESC, preference_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_env_source ON delivery_outcomes(environment_scope, source_kind, source_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_env_run ON delivery_outcomes(environment_scope, run_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_env_workflow ON delivery_outcomes(environment_scope, workflow_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_env_schedule ON delivery_outcomes(environment_scope, schedule_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_env_target ON delivery_outcomes(environment_scope, chosen_target_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_attempts_delivery ON delivery_attempts(delivery_id, attempt_number ASC, attempt_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_summary_windows_env_status ON delivery_summary_windows(environment_scope, status, window_ends_at ASC, summary_window_id ASC);`,
		},
	},
	{
		Version: 17,
		Name:    "calendar_domain",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS calendar_accounts (
				calendar_account_id TEXT PRIMARY KEY,
				integration_id TEXT NOT NULL UNIQUE,
				environment_scope TEXT NOT NULL,
				account_key TEXT,
				readiness_status TEXT NOT NULL,
				canonical_default INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS calendar_operations (
				operation_id TEXT PRIMARY KEY,
				integration_id TEXT NOT NULL,
				calendar_account_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				operation_class TEXT NOT NULL,
				status TEXT NOT NULL,
				external_event_id TEXT,
				run_id TEXT,
				workflow_id TEXT,
				schedule_id TEXT,
				delivery_id TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS calendar_artifacts (
				artifact_id TEXT PRIMARY KEY,
				operation_id TEXT NOT NULL,
				integration_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				kind TEXT NOT NULL,
				external_event_id TEXT,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_accounts_env_readiness ON calendar_accounts(environment_scope, readiness_status, updated_at DESC, integration_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_accounts_env_default ON calendar_accounts(environment_scope, account_key, canonical_default, updated_at DESC, integration_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_env_class_status ON calendar_operations(environment_scope, operation_class, status, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_run ON calendar_operations(environment_scope, run_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_workflow ON calendar_operations(environment_scope, workflow_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_schedule ON calendar_operations(environment_scope, schedule_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_delivery ON calendar_operations(environment_scope, delivery_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_event ON calendar_operations(environment_scope, external_event_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_artifacts_operation ON calendar_artifacts(environment_scope, operation_id, created_at ASC, artifact_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_artifacts_event ON calendar_artifacts(environment_scope, external_event_id, created_at DESC, artifact_id DESC);`,
		},
	},
	{
		Version: 18,
		Name:    "mail_domain",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS mail_accounts (
				mail_account_id TEXT PRIMARY KEY,
				integration_id TEXT NOT NULL UNIQUE,
				environment_scope TEXT NOT NULL,
				account_key TEXT,
				readiness_status TEXT NOT NULL,
				canonical_default INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS mail_operations (
				operation_id TEXT PRIMARY KEY,
				integration_id TEXT NOT NULL,
				mail_account_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				operation_class TEXT NOT NULL,
				status TEXT NOT NULL,
				result_mode TEXT NOT NULL,
				thread_id TEXT,
				message_id TEXT,
				draft_id TEXT,
				run_id TEXT,
				workflow_id TEXT,
				schedule_id TEXT,
				delivery_id TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS mail_artifacts (
				artifact_id TEXT PRIMARY KEY,
				operation_id TEXT NOT NULL,
				integration_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				kind TEXT NOT NULL,
				thread_id TEXT,
				message_id TEXT,
				draft_id TEXT,
				attachment_ref_id TEXT,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_mail_accounts_env_readiness ON mail_accounts(environment_scope, readiness_status, updated_at DESC, integration_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_accounts_env_default ON mail_accounts(environment_scope, account_key, canonical_default, updated_at DESC, integration_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_env_class_status ON mail_operations(environment_scope, operation_class, status, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_env_result ON mail_operations(environment_scope, result_mode, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_run ON mail_operations(environment_scope, run_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_workflow ON mail_operations(environment_scope, workflow_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_schedule ON mail_operations(environment_scope, schedule_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_delivery ON mail_operations(environment_scope, delivery_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_thread ON mail_operations(environment_scope, thread_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_message ON mail_operations(environment_scope, message_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_draft ON mail_operations(environment_scope, draft_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_operation ON mail_artifacts(environment_scope, operation_id, created_at ASC, artifact_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_thread ON mail_artifacts(environment_scope, thread_id, created_at DESC, artifact_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_message ON mail_artifacts(environment_scope, message_id, created_at DESC, artifact_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_draft ON mail_artifacts(environment_scope, draft_id, created_at DESC, artifact_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_attachment ON mail_artifacts(environment_scope, attachment_ref_id, created_at DESC, artifact_id DESC);`,
		},
	},
	{
		Version: 19,
		Name:    "reminder_domain",
		Statements: []string{
			`ALTER TABLE runs ADD COLUMN reminder_id TEXT;`,
			`ALTER TABLE runs ADD COLUMN reminder_occurrence_id TEXT;`,
			`
			CREATE TABLE IF NOT EXISTS reminders (
				reminder_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				behavior_mode TEXT NOT NULL,
				current_state TEXT NOT NULL,
				next_due_at TEXT,
				active_occurrence_id TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS reminder_occurrences (
				occurrence_id TEXT PRIMARY KEY,
				reminder_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				state TEXT NOT NULL,
				scheduled_for TEXT NOT NULL,
				run_id TEXT,
				workflow_id TEXT,
				latest_delivery_id TEXT,
				latest_delivery_status TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS reminder_actions (
				action_id TEXT PRIMARY KEY,
				reminder_id TEXT NOT NULL,
				occurrence_id TEXT,
				action_kind TEXT NOT NULL,
				new_state TEXT,
				run_id TEXT,
				workflow_id TEXT,
				delivery_id TEXT,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_runs_reminder ON runs(reminder_id, reminder_occurrence_id, created_at DESC, run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_reminders_env_state ON reminders(environment_scope, current_state, updated_at DESC, reminder_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_reminders_env_due ON reminders(environment_scope, next_due_at, updated_at DESC, reminder_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_occurrences_reminder ON reminder_occurrences(environment_scope, reminder_id, scheduled_for DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_occurrences_state ON reminder_occurrences(environment_scope, state, scheduled_for DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_occurrences_run ON reminder_occurrences(environment_scope, run_id, updated_at DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_occurrences_workflow ON reminder_occurrences(environment_scope, workflow_id, updated_at DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_occurrences_delivery ON reminder_occurrences(environment_scope, latest_delivery_id, updated_at DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_actions_reminder ON reminder_actions(reminder_id, created_at DESC, action_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_rem_actions_occurrence ON reminder_actions(occurrence_id, created_at DESC, action_id DESC);`,
		},
	},
	{
		Version: 20,
		Name:    "evaluation_replay_harness",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS evaluation_replay_candidates (
				candidate_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				candidate_kind TEXT NOT NULL,
				source_kind TEXT NOT NULL,
				source_id TEXT NOT NULL,
				readiness_status TEXT NOT NULL,
				latest_attempt_id TEXT,
				latest_comparison_id TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_replay_attempts (
				attempt_id TEXT PRIMARY KEY,
				candidate_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				mode TEXT NOT NULL,
				status TEXT NOT NULL,
				change_window_label TEXT,
				baseline_attempt_id TEXT,
				started_at TEXT,
				completed_at TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(candidate_id) REFERENCES evaluation_replay_candidates(candidate_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_comparisons (
				comparison_id TEXT PRIMARY KEY,
				candidate_id TEXT NOT NULL,
				attempt_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				terminal_status TEXT NOT NULL,
				change_window_label TEXT,
				generated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(candidate_id) REFERENCES evaluation_replay_candidates(candidate_id) ON DELETE CASCADE,
				FOREIGN KEY(attempt_id) REFERENCES evaluation_replay_attempts(attempt_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_regression_fixtures (
				fixture_id TEXT PRIMARY KEY,
				environment_scope TEXT NOT NULL,
				domain_class TEXT NOT NULL,
				candidate_id TEXT,
				manifest_path TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_eval_candidates_env_ready ON evaluation_replay_candidates(environment_scope, readiness_status, updated_at DESC, candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_candidates_env_kind ON evaluation_replay_candidates(environment_scope, candidate_kind, source_kind, updated_at DESC, candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_attempts_env_candidate ON evaluation_replay_attempts(environment_scope, candidate_id, created_at DESC, attempt_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_attempts_env_status ON evaluation_replay_attempts(environment_scope, status, created_at DESC, attempt_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_comparisons_env_candidate ON evaluation_comparisons(environment_scope, candidate_id, generated_at DESC, comparison_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_comparisons_env_attempt ON evaluation_comparisons(environment_scope, attempt_id, generated_at DESC, comparison_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_fixtures_env_domain ON evaluation_regression_fixtures(environment_scope, domain_class, updated_at DESC, fixture_id DESC);`,
		},
	},
	{
		Version: 21,
		Name:    "tenant_identity_access_foundation",
		Statements: []string{
			`ALTER TABLE auth_tokens ADD COLUMN principal_id TEXT;`,
			`ALTER TABLE auth_tokens ADD COLUMN status TEXT NOT NULL DEFAULT 'active';`,
			`ALTER TABLE auth_tokens ADD COLUMN default_tenant_id TEXT;`,
			`ALTER TABLE auth_tokens ADD COLUMN expires_at TEXT;`,
			`ALTER TABLE auth_tokens ADD COLUMN revoked_at TEXT;`,
			`ALTER TABLE auth_tokens ADD COLUMN rotated_from_token_id TEXT;`,
			`ALTER TABLE auth_tokens ADD COLUMN rotated_to_token_id TEXT;`,
			`
			CREATE TABLE IF NOT EXISTS tenants (
				tenant_id TEXT PRIMARY KEY,
				tenant_kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				created_by_principal_id TEXT,
				default_owner_principal_id TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS principals (
				principal_id TEXT PRIMARY KEY,
				principal_kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				default_tenant_id TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				disabled_at TEXT,
				removed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS memberships (
				membership_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				principal_id TEXT NOT NULL,
				role TEXT NOT NULL,
				status TEXT NOT NULL,
				invitation_id TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				accepted_at TEXT,
				removed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS tenant_invitations (
				invitation_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				invited_principal_id TEXT NOT NULL,
				invited_by_principal_id TEXT NOT NULL,
				role TEXT NOT NULL,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				expires_at TEXT,
				decided_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS token_tenant_grants (
				grant_id TEXT PRIMARY KEY,
				token_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				is_default INTEGER NOT NULL,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				revoked_at TEXT,
				granted_by_principal_id TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS tenant_audit_events (
				audit_event_id TEXT PRIMARY KEY,
				event_kind TEXT NOT NULL,
				tenant_id TEXT,
				principal_id TEXT,
				target_principal_id TEXT,
				token_id TEXT,
				outcome TEXT NOT NULL,
				reason_code TEXT NOT NULL,
				created_at TEXT NOT NULL,
				document_json TEXT
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_tenants_kind_status ON tenants(tenant_kind, status, created_at DESC, tenant_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_principals_status ON principals(status, created_at DESC, principal_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_memberships_tenant_status ON memberships(tenant_id, status, role, principal_id);`,
			`CREATE INDEX IF NOT EXISTS idx_memberships_principal_status ON memberships(principal_id, status, tenant_id);`,
			`CREATE INDEX IF NOT EXISTS idx_invitations_tenant_status ON tenant_invitations(tenant_id, status, created_at DESC, invitation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_invitations_principal_status ON tenant_invitations(invited_principal_id, status, created_at DESC, invitation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_token_grants_token_status ON token_tenant_grants(token_id, status, tenant_id);`,
			`CREATE INDEX IF NOT EXISTS idx_token_grants_tenant_status ON token_tenant_grants(tenant_id, status, token_id);`,
			`CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_tenant_created ON tenant_audit_events(tenant_id, created_at DESC, audit_event_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_principal_created ON tenant_audit_events(principal_id, created_at DESC, audit_event_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_token_created ON tenant_audit_events(token_id, created_at DESC, audit_event_id DESC);`,
		},
	},
	{
		// Roadmap 35: durable per-step progress for the tenant-scoped data
		// migration. Backfill steps record `last_processed_key` after each
		// chunk commit so an interrupted migration resumes exactly once.
		Version: 22,
		Name:    "tenant_migration_progress",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS tenant_migration_progress (
				step_name           TEXT PRIMARY KEY,
				status              TEXT NOT NULL,
				started_at          INTEGER,
				completed_at        INTEGER,
				last_processed_key  TEXT,
				error               TEXT
			);
			`,
		},
	},
	{
		// Roadmap 35 (Phase 2 / T016): add NULLABLE tenant_id column and
		// tenant-aware indexes on the events table (step (a) of the
		// three-stage pattern). The actual backfill is owned by T077 in US2
		// and runs only after every parent table's tenant_id has been
		// populated. Step (c) for the events table (T077b) is a CHECK +
		// partial-index pair, not a NOT NULL swap, since events is a
		// mixed table carrying both tenant-owned and global categories.
		Version: 23,
		Name:    "tenant_scope_events_column",
		Statements: []string{
			`ALTER TABLE events ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_events_tenant_time ON events(tenant_id, occurred_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_events_tenant_category_time ON events(tenant_id, category, name, occurred_at DESC);`,
		},
	},
	{
		// Roadmap 35 (Phase 3 / T018): add NULLABLE tenant_id and tenant-aware
		// indexes on the runtime spine — sessions, runs, steps, tool_calls,
		// llm_dispatches, checkpoints. This is step (a) of the three-stage
		// per-domain pattern. Backfill (T067) and the NOT NULL shadow swap
		// (T077a) follow in US2; until then writers populate tenant_id
		// opportunistically when a TenantContext is present in the request.
		Version: 24,
		Name:    "tenant_scope_runtime_columns",
		Statements: []string{
			`ALTER TABLE sessions ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE runs ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE steps ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE tool_calls ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE llm_dispatches ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE checkpoints ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_tenant_created ON sessions(tenant_id, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_runs_tenant_created ON runs(tenant_id, created_at DESC, run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_steps_tenant_run_created ON steps(tenant_id, run_id, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_tenant_step ON tool_calls(tenant_id, step_id);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_tenant_status_created ON tool_calls(tenant_id, status, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_llm_dispatches_tenant_created ON llm_dispatches(tenant_id, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_llm_dispatches_tenant_provider_status ON llm_dispatches(tenant_id, provider, status, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_checkpoints_tenant_run_captured ON checkpoints(tenant_id, run_id, captured_at DESC);`,
		},
	},
	{
		// Roadmap 35 (Phase 3 / T019): schedules family — schedules,
		// schedule_targets, schedule_dispatch_attempts. Step (a) only:
		// NULLABLE column add + tenant-aware indexes. UNIQUE
		// (tenant_id, kind, target_ref_id) on schedules is added via
		// shadow swap in T077a.
		Version: 25,
		Name:    "tenant_scope_schedules_columns",
		Statements: []string{
			`ALTER TABLE schedules ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE schedule_targets ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE schedule_dispatch_attempts ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_schedules_tenant_status_due ON schedules(tenant_id, status, next_due_at, schedule_id);`,
			`CREATE INDEX IF NOT EXISTS idx_schedule_targets_tenant_schedule ON schedule_targets(tenant_id, schedule_id, target_ref_id);`,
			`CREATE INDEX IF NOT EXISTS idx_schedule_attempts_tenant_schedule_due ON schedule_dispatch_attempts(tenant_id, schedule_id, due_at DESC, attempt_id DESC);`,
		},
	},
	{
		// Roadmap 35 (T020): workflows family. workflows.tenant_id is
		// derived from runs.tenant_id via the workflows.run_id FK during
		// US2 backfill.
		Version: 26,
		Name:    "tenant_scope_workflows_columns",
		Statements: []string{
			`ALTER TABLE workflows ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE workflow_steps ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE workflow_dependencies ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE workflow_handoffs ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_workflows_tenant_updated ON workflows(tenant_id, updated_at DESC, workflow_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_steps_tenant_workflow_position ON workflow_steps(tenant_id, workflow_id, position);`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_dependencies_tenant_workflow ON workflow_dependencies(tenant_id, workflow_id);`,
			`CREATE INDEX IF NOT EXISTS idx_workflow_handoffs_tenant_workflow ON workflow_handoffs(tenant_id, workflow_id);`,
		},
	},
	{
		// Roadmap 35 (T021): integrations + delivery family. Step (a)
		// only — UNIQUE (tenant_id, domain_kind, account_key) on
		// integrations is added via shadow swap in T077a (partial index
		// filtered to non-NULL account_key).
		Version: 27,
		Name:    "tenant_scope_integrations_delivery_columns",
		Statements: []string{
			`ALTER TABLE integrations ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE delivery_targets ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE delivery_preferences ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE delivery_outcomes ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE delivery_attempts ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE delivery_summary_windows ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_integrations_tenant_domain_account ON integrations(tenant_id, domain_kind, account_key, canonical_default);`,
			`CREATE INDEX IF NOT EXISTS idx_integrations_tenant_readiness ON integrations(tenant_id, readiness_status, updated_at DESC, integration_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_targets_tenant_status ON delivery_targets(tenant_id, status, updated_at DESC, target_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_preferences_tenant_scope ON delivery_preferences(tenant_id, scope_kind, integration_id, active, updated_at DESC, preference_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_outcomes_tenant_updated ON delivery_outcomes(tenant_id, updated_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_attempts_tenant_delivery ON delivery_attempts(tenant_id, delivery_id, attempt_number ASC, attempt_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_delivery_summary_windows_tenant_status ON delivery_summary_windows(tenant_id, status, window_ends_at ASC, summary_window_id ASC);`,
		},
	},
	{
		// Roadmap 35 (T022): calendar family. Account-level uniqueness
		// (tenant_id, account_key) on calendar_accounts is added in
		// T077a via partial index filtered to non-NULL account_key.
		Version: 28,
		Name:    "tenant_scope_calendar_columns",
		Statements: []string{
			`ALTER TABLE calendar_accounts ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE calendar_operations ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE calendar_artifacts ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_accounts_tenant_readiness ON calendar_accounts(tenant_id, readiness_status, updated_at DESC, calendar_account_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_operations_tenant_account ON calendar_operations(tenant_id, calendar_account_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_calendar_artifacts_tenant_operation ON calendar_artifacts(tenant_id, operation_id, created_at ASC, artifact_id ASC);`,
		},
	},
	{
		// Roadmap 35 (T023): mail family. Account-level uniqueness
		// (tenant_id, account_key) on mail_accounts is added in T077a
		// via partial index filtered to non-NULL account_key.
		Version: 29,
		Name:    "tenant_scope_mail_columns",
		Statements: []string{
			`ALTER TABLE mail_accounts ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mail_operations ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mail_artifacts ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_mail_accounts_tenant_readiness ON mail_accounts(tenant_id, readiness_status, updated_at DESC, mail_account_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_operations_tenant_account ON mail_operations(tenant_id, mail_account_id, updated_at DESC, operation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mail_artifacts_tenant_operation ON mail_artifacts(tenant_id, operation_id, created_at ASC, artifact_id ASC);`,
		},
	},
	{
		// Roadmap 35 (T024): reminders family.
		Version: 30,
		Name:    "tenant_scope_reminders_columns",
		Statements: []string{
			`ALTER TABLE reminders ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE reminder_occurrences ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE reminder_actions ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_reminders_tenant_state ON reminders(tenant_id, current_state, updated_at DESC, reminder_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_reminders_tenant_due ON reminders(tenant_id, next_due_at, updated_at DESC, reminder_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_reminder_occurrences_tenant_reminder ON reminder_occurrences(tenant_id, reminder_id, scheduled_for DESC, occurrence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_reminder_actions_tenant_reminder ON reminder_actions(tenant_id, reminder_id, created_at DESC, action_id DESC);`,
		},
	},
	{
		// Roadmap 35 (T025): computer-use family. Tenant derived from
		// runs.tenant_id via the NOT NULL run_id FK during US2 backfill.
		Version: 31,
		Name:    "tenant_scope_computer_use_columns",
		Statements: []string{
			`ALTER TABLE computer_use_sessions ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE computer_use_actions ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE computer_use_artifacts ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_sessions_tenant_run ON computer_use_sessions(tenant_id, run_id, updated_at DESC, computer_use_session_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_actions_tenant_session ON computer_use_actions(tenant_id, computer_use_session_id, requested_at ASC, computer_use_action_id ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_computer_use_artifacts_tenant_action ON computer_use_artifacts(tenant_id, computer_use_action_id, created_at ASC, artifact_id ASC);`,
		},
	},
	{
		// Roadmap 35 (T026): evaluation + harness + remaining
		// tenant_owned tables (approvals, decisions, connector_messages,
		// sandbox_executions, consumer_policy_records, provider_preferences,
		// mcp_tool_exposure_rules, secret_scope_bindings).
		// secret_scope_bindings credential semantics remain owned by
		// Roadmap 37; this migration only attaches tenant ownership for
		// scoping. Per-tenant uniqueness on mcp_tool_exposure_rules
		// (extending its composite PK to include tenant_id) and on
		// evaluation_regression_fixtures.manifest_path is added via
		// shadow swap in T077a.
		Version: 32,
		Name:    "tenant_scope_evaluation_and_remaining_columns",
		Statements: []string{
			`ALTER TABLE evaluation_replay_candidates ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE evaluation_replay_attempts ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE evaluation_comparisons ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE evaluation_regression_fixtures ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE approvals ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE decisions ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE connector_messages ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE sandbox_executions ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE consumer_policy_records ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE provider_preferences ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mcp_tool_exposure_rules ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE secret_scope_bindings ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_eval_candidates_tenant_ready ON evaluation_replay_candidates(tenant_id, readiness_status, updated_at DESC, candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_attempts_tenant_candidate ON evaluation_replay_attempts(tenant_id, candidate_id, created_at DESC, attempt_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_comparisons_tenant_candidate ON evaluation_comparisons(tenant_id, candidate_id, generated_at DESC, comparison_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_fixtures_tenant_domain ON evaluation_regression_fixtures(tenant_id, domain_class, updated_at DESC, fixture_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_approvals_tenant_status_created ON approvals(tenant_id, status, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_decisions_tenant_created ON decisions(tenant_id, created_at DESC, decision_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_connector_messages_tenant_connector ON connector_messages(tenant_id, connector_id, created_at DESC, delivery_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_sandbox_executions_tenant_status ON sandbox_executions(tenant_id, status, requested_at DESC, execution_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_policy_records_tenant_started ON consumer_policy_records(tenant_id, started_at DESC, policy_record_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_provider_preferences_tenant ON provider_preferences(tenant_id, provider_id);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_tool_exposure_tenant_server_tool ON mcp_tool_exposure_rules(tenant_id, server_id, tool_name, runtime_surface);`,
			`CREATE INDEX IF NOT EXISTS idx_secret_scope_bindings_tenant_consumer ON secret_scope_bindings(tenant_id, consumer_kind, consumer_id, secret_ref);`,
		},
	},
	{
		// Roadmap 35 (T027): verified-only migration for the Roadmap 34
		// tenant tables. The tenant_id column and the basic per-tenant
		// list indexes were already added in v21 (Roadmap 34); this
		// migration adds the missing per-tenant uniqueness constraints
		// the inventory requires. No column adds, no backfill — every
		// row already has tenant_id from the R34 bootstrap path.
		Version: 33,
		Name:    "tenant_scope_r34_uniqueness",
		Statements: []string{
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_memberships_tenant_principal ON memberships(tenant_id, principal_id);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_token_grants_tenant_token ON token_tenant_grants(tenant_id, token_id);`,
		},
	},
	{
		// Roadmap 37: hosted credential metadata and explicit tenant
		// ownership for the R37 boundary resources. Backup-restore remains
		// the rollback path for these additive storage changes.
		Version: 34,
		Name:    "hosted_credentials_and_r37_boundary_tenancy",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS tenant_secrets (
				secret_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				secret_ref TEXT NOT NULL,
				display_name TEXT,
				status TEXT NOT NULL,
				active_version_id TEXT,
				disabled_reason TEXT,
				remediation_reason TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				rotated_at TEXT,
				disabled_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS tenant_secret_versions (
				secret_version_id TEXT PRIMARY KEY,
				secret_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				secret_ref TEXT NOT NULL,
				version_number INTEGER NOT NULL,
				status TEXT NOT NULL,
				value_backend_ref TEXT NOT NULL,
				created_at TEXT NOT NULL,
				activated_at TEXT,
				superseded_at TEXT,
				FOREIGN KEY(secret_id) REFERENCES tenant_secrets(secret_id) ON DELETE CASCADE
			);
			`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_secrets_ref ON tenant_secrets(tenant_id, secret_ref);`,
			`CREATE INDEX IF NOT EXISTS idx_tenant_secrets_tenant_status ON tenant_secrets(tenant_id, status, updated_at DESC, secret_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_tenant_secret_versions_secret ON tenant_secret_versions(tenant_id, secret_id, version_number DESC);`,
			`ALTER TABLE provider_auth_states ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE connectors ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mcp_servers ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mcp_server_states ADD COLUMN tenant_id TEXT;`,
			`ALTER TABLE mcp_tools ADD COLUMN tenant_id TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_provider_auth_states_tenant_provider ON provider_auth_states(tenant_id, provider_id);`,
			`CREATE INDEX IF NOT EXISTS idx_connectors_tenant_kind_status ON connectors(tenant_id, kind, status);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_servers_tenant_enabled ON mcp_servers(tenant_id, enabled, updated_at DESC, server_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_server_states_tenant_status ON mcp_server_states(tenant_id, status, updated_at DESC, server_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_mcp_tools_tenant_server_status ON mcp_tools(tenant_id, server_id, discovery_status, tool_name);`,
		},
	},
	{
		Version: 35,
		Name:    "r37_connector_disabled_metadata",
		Statements: []string{
			`ALTER TABLE connectors ADD COLUMN disabled_reason TEXT;`,
			`ALTER TABLE connectors ADD COLUMN secret_refs_json TEXT NOT NULL DEFAULT '[]';`,
		},
	},
	{
		Version: 36,
		Name:    "r38_billing_quotas_usage",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS billing_tenant_plans (
				plan_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				plan_key TEXT NOT NULL,
				status TEXT NOT NULL,
				enforcement_mode TEXT NOT NULL,
				effective_at TEXT NOT NULL,
				superseded_at TEXT,
				assigned_by_principal_id TEXT,
				assignment_reason TEXT,
				document_json TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_quota_definitions (
				quota_definition_id TEXT PRIMARY KEY,
				category TEXT NOT NULL UNIQUE,
				unit TEXT NOT NULL,
				period_kind TEXT NOT NULL,
				period_anchor TEXT NOT NULL,
				default_limit INTEGER NOT NULL,
				carryover_enabled INTEGER NOT NULL,
				carryover_max INTEGER NOT NULL,
				reservation_rule TEXT NOT NULL,
				commit_rule TEXT NOT NULL,
				refund_rule TEXT NOT NULL,
				denial_reason_code TEXT NOT NULL,
				active INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_quota_overrides (
				quota_override_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT NOT NULL,
				limit_amount INTEGER,
				carryover_enabled INTEGER,
				carryover_max INTEGER,
				effective_at TEXT NOT NULL,
				expires_at TEXT,
				reason TEXT NOT NULL,
				created_by_principal_id TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_quota_periods (
				quota_period_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT NOT NULL,
				period_kind TEXT NOT NULL,
				period_start TEXT NOT NULL,
				period_end TEXT NOT NULL,
				carryover_from_period_id TEXT,
				status TEXT NOT NULL,
				UNIQUE(tenant_id, category, period_start)
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_usage_counters (
				usage_counter_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT NOT NULL,
				quota_period_id TEXT NOT NULL,
				committed_amount INTEGER NOT NULL,
				reserved_amount INTEGER NOT NULL,
				adjusted_amount INTEGER NOT NULL,
				carryover_amount INTEGER NOT NULL,
				updated_at TEXT NOT NULL,
				UNIQUE(tenant_id, category, quota_period_id)
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_usage_reservations (
				reservation_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT NOT NULL,
				quota_period_id TEXT NOT NULL,
				operation_key TEXT NOT NULL,
				amount_reserved INTEGER NOT NULL,
				amount_committed INTEGER NOT NULL,
				amount_refunded INTEGER NOT NULL,
				status TEXT NOT NULL,
				reservation_point TEXT,
				commit_point TEXT,
				refund_point TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				expires_at TEXT,
				recovery_reason TEXT,
				UNIQUE(tenant_id, category, operation_key)
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_usage_events (
				usage_event_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT,
				quota_period_id TEXT,
				operation_key TEXT,
				event_kind TEXT NOT NULL,
				amount INTEGER NOT NULL,
				reason_code TEXT NOT NULL,
				reason TEXT,
				actor_principal_id TEXT,
				outcome TEXT NOT NULL,
				created_at TEXT NOT NULL,
				document_json TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_quota_denials (
				denial_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT,
				quota_period_id TEXT,
				operation_key TEXT NOT NULL,
				reason_code TEXT NOT NULL,
				requested_amount INTEGER NOT NULL,
				remaining_amount INTEGER NOT NULL,
				guarded_entry_point TEXT NOT NULL,
				created_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_manual_adjustments (
				adjustment_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				category TEXT NOT NULL,
				quota_period_id TEXT NOT NULL,
				amount_delta INTEGER NOT NULL,
				reason TEXT NOT NULL,
				created_by_principal_id TEXT,
				created_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS billing_audit_retention_policies (
				policy_id TEXT PRIMARY KEY,
				tenant_id TEXT,
				retention_mode TEXT NOT NULL,
				retention_period TEXT,
				created_by_principal_id TEXT,
				reason TEXT,
				created_at TEXT NOT NULL,
				expires_at TEXT
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_billing_tenant_plans_active ON billing_tenant_plans(tenant_id, status, effective_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_quota_overrides_tenant_category ON billing_quota_overrides(tenant_id, category, effective_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_quota_periods_tenant_category ON billing_quota_periods(tenant_id, category, period_start DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_usage_reservations_pending ON billing_usage_reservations(status, updated_at ASC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_usage_events_tenant_created ON billing_usage_events(tenant_id, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_quota_denials_tenant_created ON billing_quota_denials(tenant_id, created_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_billing_manual_adjustments_tenant_created ON billing_manual_adjustments(tenant_id, created_at DESC);`,
		},
	},
	{
		Version: 37,
		Name:    "r40_live_validation_replay",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS live_validation_attempts (
				validation_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				candidate_id TEXT NOT NULL,
				source_attempt_id TEXT,
				environment_scope TEXT NOT NULL,
				status TEXT NOT NULL,
				comparison_id TEXT,
				created_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_scopes (
				scope_id TEXT PRIMARY KEY,
				validation_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				approval_mode TEXT NOT NULL,
				declared_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(validation_id) REFERENCES live_validation_attempts(validation_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_approvals (
				approval_id TEXT PRIMARY KEY,
				validation_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				approval_target TEXT NOT NULL,
				tool_class TEXT NOT NULL,
				action_ref TEXT,
				status TEXT NOT NULL,
				requested_at TEXT NOT NULL,
				resolved_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(validation_id) REFERENCES live_validation_attempts(validation_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_support_matrix_snapshots (
				snapshot_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				validation_id TEXT,
				version TEXT NOT NULL,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_ledger_entries (
				ledger_entry_id TEXT PRIMARY KEY,
				validation_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				candidate_id TEXT NOT NULL,
				tool_class TEXT NOT NULL,
				safety_class TEXT NOT NULL,
				action_ref TEXT NOT NULL,
				outcome TEXT NOT NULL,
				attempted_at TEXT,
				completed_at TEXT,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(validation_id) REFERENCES live_validation_attempts(validation_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_kill_switches (
				kill_switch_id TEXT PRIMARY KEY,
				scope TEXT NOT NULL,
				tenant_id TEXT,
				enabled INTEGER NOT NULL,
				changed_at TEXT NOT NULL,
				expires_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_ambiguous_commits (
				ambiguous_commit_id TEXT PRIMARY KEY,
				ledger_entry_id TEXT NOT NULL,
				validation_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				cause TEXT NOT NULL,
				automatic_retry_stopped INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(ledger_entry_id) REFERENCES live_validation_ledger_entries(ledger_entry_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_reconciliation_resolutions (
				reconciliation_id TEXT PRIMARY KEY,
				ambiguous_commit_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				resolved_by TEXT NOT NULL,
				resolution TEXT NOT NULL,
				resolved_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(ambiguous_commit_id) REFERENCES live_validation_ambiguous_commits(ambiguous_commit_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_comparisons (
				comparison_id TEXT PRIMARY KEY,
				validation_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				candidate_id TEXT NOT NULL,
				terminal_status TEXT NOT NULL,
				generated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(validation_id) REFERENCES live_validation_attempts(validation_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS live_validation_retention_policies (
				policy_id TEXT PRIMARY KEY,
				tenant_id TEXT,
				applies_to TEXT NOT NULL,
				retention_mode TEXT NOT NULL,
				created_by_principal_id TEXT NOT NULL,
				created_at TEXT NOT NULL,
				expires_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_attempts_tenant_status ON live_validation_attempts(tenant_id, status, updated_at DESC, validation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_attempts_tenant_candidate ON live_validation_attempts(tenant_id, candidate_id, updated_at DESC, validation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_attempts_env_status ON live_validation_attempts(environment_scope, status, updated_at DESC, validation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_scopes_validation ON live_validation_scopes(tenant_id, validation_id);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_approvals_validation_status ON live_validation_approvals(tenant_id, validation_id, status, requested_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_matrix_tenant_created ON live_validation_support_matrix_snapshots(tenant_id, created_at DESC, snapshot_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_ledger_validation ON live_validation_ledger_entries(tenant_id, validation_id, updated_at DESC, ledger_entry_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_ledger_outcome ON live_validation_ledger_entries(tenant_id, outcome, updated_at DESC, ledger_entry_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_kill_switches_enabled ON live_validation_kill_switches(scope, tenant_id, enabled, changed_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_ambiguous_validation ON live_validation_ambiguous_commits(tenant_id, validation_id, updated_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_reconciliations_tenant ON live_validation_reconciliation_resolutions(tenant_id, resolved_at DESC, reconciliation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_comparisons_validation ON live_validation_comparisons(tenant_id, validation_id, generated_at DESC, comparison_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_live_validation_retention_tenant ON live_validation_retention_policies(tenant_id, applies_to, created_at DESC, policy_id DESC);`,
		},
	},
	{
		Version: 38,
		Name:    "r41_evaluation_product_expansion",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS evaluation_discovery_policies (
				policy_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				enabled INTEGER NOT NULL,
				window_start TEXT NOT NULL,
				window_end TEXT NOT NULL,
				max_inspected_records INTEGER NOT NULL,
				max_emitted_candidates INTEGER NOT NULL,
				cost_budget INTEGER NOT NULL,
				created_by TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_discovery_runs (
				discovery_run_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				policy_id TEXT,
				status TEXT NOT NULL,
				cursor TEXT,
				window_start TEXT NOT NULL,
				window_end TEXT NOT NULL,
				max_inspected_records INTEGER NOT NULL,
				max_emitted_candidates INTEGER NOT NULL,
				cost_budget INTEGER NOT NULL,
				inspected_records INTEGER NOT NULL,
				emitted_candidates INTEGER NOT NULL,
				started_by TEXT,
				started_at TEXT NOT NULL,
				completed_at TEXT,
				updated_at TEXT NOT NULL,
				idempotency_key TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(policy_id) REFERENCES evaluation_discovery_policies(policy_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_discovered_candidates (
				discovered_candidate_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				discovery_run_id TEXT NOT NULL,
				source_kind TEXT NOT NULL,
				source_id TEXT NOT NULL,
				score REAL NOT NULL,
				score_band TEXT NOT NULL,
				redaction_status TEXT NOT NULL,
				evidence_ref TEXT,
				readiness_status TEXT NOT NULL,
				suppression_state TEXT NOT NULL,
				retention_state TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				expires_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(discovery_run_id) REFERENCES evaluation_discovery_runs(discovery_run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_candidate_evidence (
				evidence_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				discovered_candidate_id TEXT NOT NULL,
				redaction_status TEXT NOT NULL,
				materialization_allowed INTEGER NOT NULL,
				retention_state TEXT NOT NULL,
				created_at TEXT NOT NULL,
				expires_at TEXT,
				document_json TEXT NOT NULL,
				FOREIGN KEY(discovered_candidate_id) REFERENCES evaluation_discovered_candidates(discovered_candidate_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_suppressions (
				suppression_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				target_kind TEXT NOT NULL,
				target_id TEXT,
				target_source_ref TEXT,
				reason_code TEXT NOT NULL,
				created_by TEXT,
				active INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				expires_at TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_product_fixtures (
				fixture_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				display_name TEXT NOT NULL,
				domain_class TEXT NOT NULL,
				source_kind TEXT,
				source_candidate_id TEXT,
				current_revision_id TEXT,
				review_state TEXT NOT NULL,
				suppression_state TEXT NOT NULL,
				retention_state TEXT NOT NULL,
				created_by TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_fixture_revisions (
				revision_id TEXT PRIMARY KEY,
				fixture_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				revision_number INTEGER NOT NULL,
				redaction_status TEXT NOT NULL,
				created_by TEXT,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(fixture_id) REFERENCES evaluation_product_fixtures(fixture_id) ON DELETE CASCADE,
				UNIQUE(fixture_id, revision_number)
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_campaigns (
				campaign_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT,
				published_at TEXT,
				retention_state TEXT NOT NULL,
				idempotency_key TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_campaign_items (
				campaign_item_id TEXT PRIMARY KEY,
				campaign_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				source_type TEXT NOT NULL,
				source_id TEXT NOT NULL,
				suppression_checked_at TEXT NOT NULL,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(campaign_id) REFERENCES evaluation_campaigns(campaign_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_campaign_attempt_groups (
				attempt_group_id TEXT PRIMARY KEY,
				campaign_id TEXT NOT NULL,
				campaign_item_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				status TEXT NOT NULL,
				drift_count INTEGER NOT NULL,
				failure_count INTEGER NOT NULL,
				unsupported_count INTEGER NOT NULL,
				operator_action_needed_count INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(campaign_id) REFERENCES evaluation_campaigns(campaign_id) ON DELETE CASCADE,
				FOREIGN KEY(campaign_item_id) REFERENCES evaluation_campaign_items(campaign_item_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_dashboard_projections (
				projection_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				window_start TEXT NOT NULL,
				window_end TEXT NOT NULL,
				generated_at TEXT NOT NULL,
				cursor TEXT,
				retention_state TEXT NOT NULL DEFAULT 'active',
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_tool_call_inspections (
				inspection_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				campaign_id TEXT NOT NULL,
				campaign_item_id TEXT NOT NULL,
				tool_call_ref TEXT NOT NULL,
				classification TEXT NOT NULL,
				redaction_status TEXT NOT NULL,
				retention_state TEXT NOT NULL DEFAULT 'active',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(campaign_id) REFERENCES evaluation_campaigns(campaign_id) ON DELETE CASCADE,
				FOREIGN KEY(campaign_item_id) REFERENCES evaluation_campaign_items(campaign_item_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS evaluation_retention_applications (
				application_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				resource_kind TEXT NOT NULL,
				resource_id TEXT,
				dry_run INTEGER NOT NULL,
				outcome TEXT NOT NULL,
				applied_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovery_policies_tenant_enabled ON evaluation_discovery_policies(tenant_id, enabled, updated_at DESC, policy_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovery_runs_tenant_status ON evaluation_discovery_runs(tenant_id, status, updated_at DESC, discovery_run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovery_runs_tenant_policy ON evaluation_discovery_runs(tenant_id, policy_id, updated_at DESC, discovery_run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovered_candidates_tenant_ready ON evaluation_discovered_candidates(tenant_id, readiness_status, updated_at DESC, discovered_candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovered_candidates_tenant_source ON evaluation_discovered_candidates(tenant_id, source_kind, source_id, updated_at DESC, discovered_candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_discovered_candidates_tenant_run ON evaluation_discovered_candidates(tenant_id, discovery_run_id, updated_at DESC, discovered_candidate_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_candidate_evidence_tenant_candidate ON evaluation_candidate_evidence(tenant_id, discovered_candidate_id, created_at DESC, evidence_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_suppressions_tenant_target ON evaluation_suppressions(tenant_id, target_kind, target_id, active, created_at DESC, suppression_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_product_fixtures_tenant_review ON evaluation_product_fixtures(tenant_id, review_state, updated_at DESC, fixture_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_product_fixtures_tenant_source ON evaluation_product_fixtures(tenant_id, source_kind, source_candidate_id, updated_at DESC, fixture_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_fixture_revisions_tenant_fixture ON evaluation_fixture_revisions(tenant_id, fixture_id, revision_number DESC, revision_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaigns_tenant_status ON evaluation_campaigns(tenant_id, status, created_at DESC, campaign_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaigns_tenant_idempotency ON evaluation_campaigns(tenant_id, idempotency_key, created_at DESC, campaign_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaign_items_tenant_campaign ON evaluation_campaign_items(tenant_id, campaign_id, created_at DESC, campaign_item_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaign_items_tenant_source ON evaluation_campaign_items(tenant_id, source_type, source_id, created_at DESC, campaign_item_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaign_attempt_groups_tenant_campaign ON evaluation_campaign_attempt_groups(tenant_id, campaign_id, updated_at DESC, attempt_group_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_campaign_attempt_groups_tenant_item ON evaluation_campaign_attempt_groups(tenant_id, campaign_item_id, updated_at DESC, attempt_group_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_dashboard_projections_tenant_generated ON evaluation_dashboard_projections(tenant_id, generated_at DESC, projection_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_tool_call_inspections_tenant_campaign ON evaluation_tool_call_inspections(tenant_id, campaign_id, updated_at DESC, inspection_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_tool_call_inspections_tenant_item ON evaluation_tool_call_inspections(tenant_id, campaign_item_id, updated_at DESC, inspection_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_eval_retention_applications_tenant_resource ON evaluation_retention_applications(tenant_id, resource_kind, resource_id, applied_at DESC, application_id DESC);`,
		},
	},
	{
		Version: 39,
		Name:    "r42_integration_diagnostics",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS integration_diagnostic_runs (
				diagnostic_run_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				integration_id TEXT NOT NULL,
				integration_account_id TEXT,
				domain_kind TEXT,
				provider_kind TEXT,
				requested_by TEXT NOT NULL,
				trigger TEXT NOT NULL,
				status TEXT NOT NULL,
				started_at TEXT NOT NULL,
				completed_at TEXT,
				failure_reason_code TEXT,
				redaction_status TEXT NOT NULL,
				retention_expires_at TEXT NOT NULL,
				idempotency_key TEXT,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS integration_diagnostic_results (
				diagnostic_result_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				integration_id TEXT NOT NULL,
				integration_account_id TEXT,
				domain_kind TEXT NOT NULL,
				provider_kind TEXT NOT NULL,
				capability TEXT NOT NULL,
				status TEXT NOT NULL,
				reason_code TEXT NOT NULL,
				remediation_owner TEXT NOT NULL,
				retry_safety TEXT NOT NULL,
				checked_at TEXT NOT NULL,
				stale_after TEXT NOT NULL,
				freshness_state TEXT NOT NULL,
				run_id TEXT,
				redaction_status TEXT NOT NULL,
				retention_expires_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES integration_diagnostic_runs(diagnostic_run_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS integration_provider_classifications (
				classification_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				provider_kind TEXT NOT NULL,
				domain_kind TEXT NOT NULL,
				integration_id TEXT,
				operation_class TEXT,
				reason_code TEXT NOT NULL,
				retry_safety TEXT NOT NULL,
				remediation_owner TEXT NOT NULL,
				redaction_status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS integration_smoke_reports (
				smoke_report_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				report_kind TEXT NOT NULL,
				requested_by TEXT NOT NULL,
				status TEXT NOT NULL,
				started_at TEXT NOT NULL,
				completed_at TEXT,
				published_at TEXT,
				retention_expires_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS integration_smoke_probe_outcomes (
				probe_outcome_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				smoke_report_id TEXT NOT NULL,
				integration_id TEXT NOT NULL,
				integration_account_id TEXT,
				domain_kind TEXT NOT NULL,
				provider_kind TEXT NOT NULL,
				probe_action TEXT NOT NULL,
				result TEXT NOT NULL,
				reason_code TEXT NOT NULL,
				retry_safety TEXT NOT NULL,
				checked_at TEXT NOT NULL,
				redaction_status TEXT NOT NULL,
				retention_expires_at TEXT NOT NULL,
				document_json TEXT NOT NULL,
				FOREIGN KEY(smoke_report_id) REFERENCES integration_smoke_reports(smoke_report_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS integration_diagnostic_retention (
				retention_record_id TEXT PRIMARY KEY,
				tenant_id TEXT NOT NULL,
				target_kind TEXT NOT NULL,
				target_id TEXT NOT NULL,
				policy_ref TEXT,
				default_expires_at TEXT NOT NULL,
				effective_expires_at TEXT NOT NULL,
				retention_state TEXT NOT NULL,
				applied_at TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				document_json TEXT NOT NULL
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_runs_tenant_status ON integration_diagnostic_runs(tenant_id, status, started_at DESC, diagnostic_run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_runs_tenant_integration ON integration_diagnostic_runs(tenant_id, integration_id, started_at DESC, diagnostic_run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_runs_tenant_idempotency ON integration_diagnostic_runs(tenant_id, idempotency_key, started_at DESC, diagnostic_run_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_results_tenant_latest ON integration_diagnostic_results(tenant_id, integration_id, domain_kind, capability, checked_at DESC, diagnostic_result_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_results_tenant_reason ON integration_diagnostic_results(tenant_id, reason_code, checked_at DESC, diagnostic_result_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_provider_classifications_tenant_reason ON integration_provider_classifications(tenant_id, reason_code, created_at DESC, classification_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_smoke_reports_tenant_status ON integration_smoke_reports(tenant_id, status, started_at DESC, smoke_report_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_smoke_outcomes_tenant_report ON integration_smoke_probe_outcomes(tenant_id, smoke_report_id, checked_at DESC, probe_outcome_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_integration_diagnostic_retention_tenant_target ON integration_diagnostic_retention(tenant_id, target_kind, target_id, effective_expires_at DESC);`,
		},
	},
	{
		Version: 40,
		Name:    activationStatesMigrationName,
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS activation_states (
				activation_id TEXT PRIMARY KEY,
				principal_id TEXT NOT NULL,
				tenant_id TEXT NOT NULL,
				environment_scope TEXT NOT NULL,
				status TEXT NOT NULL,
				current_step_id TEXT NOT NULL,
				completed_step_ids_json TEXT NOT NULL,
				blocking_reason_codes_json TEXT NOT NULL,
				readiness_items_json TEXT NOT NULL,
				quota_baseline_json TEXT,
				first_action_json TEXT NOT NULL,
				test_chat_json TEXT,
				failure_reason_json TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				first_action_completed_at TEXT,
				last_evaluated_at TEXT NOT NULL,
				last_transition_audit_event_id TEXT,
				metadata_json TEXT,
				UNIQUE(principal_id, tenant_id)
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_activation_states_tenant_status ON activation_states(tenant_id, status, updated_at DESC, activation_id DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_activation_states_principal_updated ON activation_states(principal_id, updated_at DESC, activation_id DESC);`,
		},
	},
}

type SQLiteStore struct {
	DataDir string
	DBPath  string
	db      *sql.DB
	// defaultTenantCache memoizes the bootstrapped personal tenant id
	// so the legacy Upsert helpers can auto-bind tenant_id without
	// requiring callers to plumb a TenantContext (see
	// default_tenant.go for the Pass B rationale).
	defaultTenantCache defaultTenantCache
	// coldPathLogger is the sink for the C4 cold-path fail-closed
	// warning. nil means use slog.Default(). Override via
	// SetColdPathLogger from tests that need to capture the warning.
	coldPathLogger *slog.Logger
}

// SetColdPathLogger overrides the slog.Logger used by the C4 cold-path
// fail-closed warning. Test-only: production callers MUST leave it
// unset so warnings reach the operator's structured-log pipeline.
func (s *SQLiteStore) SetColdPathLogger(l *slog.Logger) {
	if s == nil {
		return
	}
	s.coldPathLogger = l
}

func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	resolvedDir, err := resolveDataDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(resolvedDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{
		DataDir: resolvedDir,
		DBPath:  dbPath,
		db:      db,
	}

	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Roadmap 35 (T016): register the events backfill + step (c) progress
	// rows as `pending` so operators can observe the migration plan via
	// `tenant_migration_progress`. Idempotent — re-running on an existing
	// store is a no-op. Per-domain backfill steps register themselves
	// when their migrations land in US1/US2.
	if err := store.RegisterEventsMigrationSteps(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Roadmap 35 (T067 / T018): register runtime spine backfill rows so the
	// migration plan is observable as soon as the v24 schema lands. The
	// driver itself is owned by US2 and is a no-op until invoked.
	if err := store.RegisterRuntimeMigrationSteps(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Roadmap 35 (T068+T069+T070+T071): register approvals/decisions,
	// schedules+children, workflows+children, and integrations+delivery
	// progress rows. Idempotent.
	if err := store.RegisterBatch2MigrationSteps(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Roadmap 35 (T072–T076b): calendar/mail/reminders/computer-use/
	// evaluation/harness/connector_messages.
	if err := store.RegisterBatch3MigrationSteps(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Roadmap 35 (T077a + T077b): step (c) enforcement progress rows.
	if err := store.RegisterEnforcementMigrationSteps(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB handle. Exposed so the tenancy
// subpackage (daemon/internal/store/tenancy) can issue tenant-scoped
// queries directly without duplicating connection/lifecycle management.
// Callers MUST NOT close the handle.
func (s *SQLiteStore) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteStore) SchemaVersion(ctx context.Context) (int, error) {
	if s == nil {
		return 0, nil
	}

	version, err := currentSchemaVersion(ctx, s.db)
	if err != nil {
		return 0, fmt.Errorf("load schema version: %w", err)
	}
	return version, nil
}

func (s *SQLiteStore) UpsertRun(ctx context.Context, run runtime.Run) error {
	if s == nil {
		return nil
	}
	// Roadmap 35 Pass B: bind tenant_id by delegating to the
	// tenant-aware safe variant when a personal tenant is bootstrapped.
	// Pre-bootstrap callers (NewSQLiteStore, early app boot) fall back
	// to the legacy tenantless write so the migration progress + auth
	// bootstrap paths still work before the personal tenant exists.
	if tenantID, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && tenantID != "" {
		return s.UpsertRunForTenantSafe(ctx, run, tenantID)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (
			run_id,
			session_id,
			schedule_id,
			schedule_attempt_id,
			reminder_id,
			reminder_occurrence_id,
			entrypoint,
			status,
			goal,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			session_id = excluded.session_id,
			schedule_id = excluded.schedule_id,
			schedule_attempt_id = excluded.schedule_attempt_id,
			reminder_id = excluded.reminder_id,
			reminder_occurrence_id = excluded.reminder_occurrence_id,
			entrypoint = excluded.entrypoint,
			status = excluded.status,
			goal = excluded.goal,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		run.RunID,
		nullString(run.SessionID),
		nullString(run.ScheduleID),
		nullString(run.ScheduleAttemptID),
		nullString(run.ReminderID),
		nullString(run.ReminderOccurrenceID),
		run.Entrypoint,
		string(run.Status),
		run.Goal,
		run.CreatedAt.UTC().Format(time.RFC3339Nano),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert run %s: %w", run.RunID, err)
	}

	return nil
}

func (s *SQLiteStore) RunTenantID(ctx context.Context, runID string) (string, bool, error) {
	if s == nil {
		return "", false, nil
	}
	var tenantID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id
		FROM runs
		WHERE run_id = ?
	`, strings.TrimSpace(runID)).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get run tenant %s: %w", runID, err)
	}
	return strings.TrimSpace(tenantID.String), tenantID.Valid && strings.TrimSpace(tenantID.String) != "", nil
}

func (s *SQLiteStore) UpsertSession(ctx context.Context, session router.Session) error {
	if s == nil {
		return nil
	}
	if tenantID, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && tenantID != "" {
		return s.UpsertSessionForTenantSafe(ctx, session, tenantID)
	}

	var lastResetAt sql.NullString
	if session.LastResetAt != nil {
		lastResetAt = sql.NullString{String: session.LastResetAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (
			session_id,
			kind,
			status,
			channel,
			account_id,
			peer_id,
			thread_id,
			routing_key,
			generation,
			created_at,
			updated_at,
			last_active_at,
			last_reset_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			kind = excluded.kind,
			status = excluded.status,
			channel = excluded.channel,
			account_id = excluded.account_id,
			peer_id = excluded.peer_id,
			thread_id = excluded.thread_id,
			routing_key = excluded.routing_key,
			generation = excluded.generation,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			last_active_at = excluded.last_active_at,
			last_reset_at = excluded.last_reset_at
	`,
		session.SessionID,
		string(session.Kind),
		string(session.Status),
		session.Channel,
		nullString(session.AccountID),
		session.PeerID,
		nullString(session.ThreadID),
		session.RoutingKey,
		session.Generation,
		session.CreatedAt.UTC().Format(time.RFC3339Nano),
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		session.LastActiveAt.UTC().Format(time.RFC3339Nano),
		lastResetAt,
	)
	if err != nil {
		return fmt.Errorf("upsert session %s: %w", session.SessionID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertPairing(ctx context.Context, pairing auth.Pairing) error {
	if s == nil {
		return nil
	}

	var completedAt sql.NullString
	if pairing.CompletedAt != nil {
		completedAt = sql.NullString{String: pairing.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_pairings (
			pairing_id,
			mode,
			label,
			status,
			code_hash,
			code_preview,
			created_at,
			updated_at,
			expires_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pairing_id) DO UPDATE SET
			mode = excluded.mode,
			label = excluded.label,
			status = excluded.status,
			code_hash = excluded.code_hash,
			code_preview = excluded.code_preview,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			completed_at = excluded.completed_at
	`,
		pairing.PairingID,
		string(pairing.Mode),
		pairing.Label,
		string(pairing.Status),
		pairing.CodeHash,
		nullString(pairing.CodePreview),
		pairing.CreatedAt.UTC().Format(time.RFC3339Nano),
		pairing.UpdatedAt.UTC().Format(time.RFC3339Nano),
		pairing.ExpiresAt.UTC().Format(time.RFC3339Nano),
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert pairing %s: %w", pairing.PairingID, err)
	}

	return nil
}

func (s *SQLiteStore) ListPairings(ctx context.Context) ([]auth.Pairing, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT pairing_id, mode, label, status, code_hash, code_preview, created_at, updated_at, expires_at, completed_at
		FROM auth_pairings
		ORDER BY created_at ASC, pairing_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pairings: %w", err)
	}
	defer rows.Close()

	items := make([]auth.Pairing, 0)
	for rows.Next() {
		pairing, err := scanPairing(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, pairing)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertAccessToken(ctx context.Context, token auth.AccessToken) error {
	if s == nil {
		return nil
	}

	status := token.Status
	if status == "" {
		status = "active"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_tokens (
			token_id,
			principal_id,
			label,
			mode,
			token_hash,
			token_preview,
			status,
			default_tenant_id,
			created_at,
			updated_at,
			last_used_at,
			expires_at,
			revoked_at,
			rotated_from_token_id,
			rotated_to_token_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token_id) DO UPDATE SET
			principal_id = excluded.principal_id,
			label = excluded.label,
			mode = excluded.mode,
			token_hash = excluded.token_hash,
			token_preview = excluded.token_preview,
			status = excluded.status,
			default_tenant_id = excluded.default_tenant_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			last_used_at = excluded.last_used_at,
			expires_at = excluded.expires_at,
			revoked_at = excluded.revoked_at,
			rotated_from_token_id = excluded.rotated_from_token_id,
			rotated_to_token_id = excluded.rotated_to_token_id
	`,
		token.TokenID,
		nullString(token.PrincipalID),
		token.Label,
		string(token.Mode),
		token.TokenHash,
		token.TokenPreview,
		status,
		nullString(token.DefaultTenantID),
		token.CreatedAt.UTC().Format(time.RFC3339Nano),
		token.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(token.LastUsedAt),
		nullableTimeString(token.ExpiresAt),
		nullableTimeString(token.RevokedAt),
		nullString(token.RotatedFromTokenID),
		nullString(token.RotatedToTokenID),
	)
	if err != nil {
		return fmt.Errorf("upsert access token %s: %w", token.TokenID, err)
	}

	return nil
}

func (s *SQLiteStore) ListAccessTokens(ctx context.Context) ([]auth.AccessToken, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT token_id, principal_id, label, mode, token_hash, token_preview, status, default_tenant_id, created_at, updated_at, last_used_at, expires_at, revoked_at, rotated_from_token_id, rotated_to_token_id
		FROM auth_tokens
		ORDER BY created_at ASC, token_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list access tokens: %w", err)
	}
	defer rows.Close()

	items := make([]auth.AccessToken, 0)
	for rows.Next() {
		token, err := scanAccessToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, token)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) ListTokenAuthorities(ctx context.Context) ([]identity.TokenAuthority, error) {
	tokens, err := s.ListAccessTokens(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]identity.TokenAuthority, 0, len(tokens))
	for _, token := range tokens {
		status := identity.LifecycleStatus(token.Status)
		if status == "" {
			status = identity.StatusActive
		}
		items = append(items, identity.TokenAuthority{
			TokenID:         token.TokenID,
			PrincipalID:     token.PrincipalID,
			DefaultTenantID: token.DefaultTenantID,
			Status:          status,
			ExpiresAt:       token.ExpiresAt,
		})
	}
	return items, nil
}

func (s *SQLiteStore) UpsertTenant(ctx context.Context, tenant identity.Tenant) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (
			tenant_id,
			tenant_kind,
			display_name,
			status,
			created_at,
			updated_at,
			created_by_principal_id,
			default_owner_principal_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET
			tenant_kind = excluded.tenant_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			created_by_principal_id = excluded.created_by_principal_id,
			default_owner_principal_id = excluded.default_owner_principal_id
	`,
		tenant.TenantID,
		string(tenant.TenantKind),
		tenant.DisplayName,
		string(tenant.Status),
		tenant.CreatedAt.UTC().Format(time.RFC3339Nano),
		tenant.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullString(tenant.CreatedByPrincipalID),
		nullString(tenant.DefaultOwnerPrincipalID),
	)
	if err != nil {
		return fmt.Errorf("upsert tenant %s: %w", tenant.TenantID, err)
	}
	return nil
}

func (s *SQLiteStore) GetTenant(ctx context.Context, tenantID string) (identity.Tenant, bool, error) {
	if s == nil {
		return identity.Tenant{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, tenant_kind, display_name, status, created_at, updated_at, created_by_principal_id, default_owner_principal_id
		FROM tenants
		WHERE tenant_id = ?
	`, tenantID)
	tenant, err := scanTenant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.Tenant{}, false, nil
	}
	if err != nil {
		return identity.Tenant{}, false, err
	}
	return tenant, true, nil
}

func (s *SQLiteStore) ListTenants(ctx context.Context, filter identity.TenantFilter) ([]identity.Tenant, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT tenant_id, tenant_kind, display_name, status, created_at, updated_at, created_by_principal_id, default_owner_principal_id
		FROM tenants
		WHERE 1 = 1
	`
	args := make([]any, 0, 4)
	if filter.TenantKind != "" {
		query += ` AND tenant_kind = ?`
		args = append(args, string(filter.TenantKind))
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	query += ` ORDER BY created_at ASC, tenant_id ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()
	items := make([]identity.Tenant, 0)
	for rows.Next() {
		item, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertPrincipal(ctx context.Context, principal identity.Principal) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO principals (
			principal_id,
			principal_kind,
			display_name,
			status,
			default_tenant_id,
			created_at,
			updated_at,
			disabled_at,
			removed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(principal_id) DO UPDATE SET
			principal_kind = excluded.principal_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			default_tenant_id = excluded.default_tenant_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			disabled_at = excluded.disabled_at,
			removed_at = excluded.removed_at
	`,
		principal.PrincipalID,
		string(principal.PrincipalKind),
		principal.DisplayName,
		string(principal.Status),
		principal.DefaultTenantID,
		principal.CreatedAt.UTC().Format(time.RFC3339Nano),
		principal.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(principal.DisabledAt),
		nullableTimeString(principal.RemovedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert principal %s: %w", principal.PrincipalID, err)
	}
	return nil
}

func (s *SQLiteStore) GetPrincipal(ctx context.Context, principalID string) (identity.Principal, bool, error) {
	if s == nil {
		return identity.Principal{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT principal_id, principal_kind, display_name, status, default_tenant_id, created_at, updated_at, disabled_at, removed_at
		FROM principals
		WHERE principal_id = ?
	`, principalID)
	principal, err := scanPrincipal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.Principal{}, false, nil
	}
	if err != nil {
		return identity.Principal{}, false, err
	}
	return principal, true, nil
}

func (s *SQLiteStore) ListPrincipals(ctx context.Context, filter identity.PrincipalFilter) ([]identity.Principal, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT p.principal_id, p.principal_kind, p.display_name, p.status, p.default_tenant_id, p.created_at, p.updated_at, p.disabled_at, p.removed_at
		FROM principals p
	`
	args := make([]any, 0, 4)
	if filter.TenantID != "" {
		query += ` JOIN memberships m ON m.principal_id = p.principal_id`
	}
	query += ` WHERE 1 = 1`
	if filter.TenantID != "" {
		query += ` AND m.tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.Status != "" {
		query += ` AND p.status = ?`
		args = append(args, string(filter.Status))
	}
	query += ` ORDER BY p.created_at ASC, p.principal_id ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list principals: %w", err)
	}
	defer rows.Close()
	items := make([]identity.Principal, 0)
	for rows.Next() {
		item, err := scanPrincipal(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMembership(ctx context.Context, membership identity.Membership) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memberships (
			membership_id,
			tenant_id,
			principal_id,
			role,
			status,
			invitation_id,
			created_at,
			updated_at,
			accepted_at,
			removed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(membership_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			principal_id = excluded.principal_id,
			role = excluded.role,
			status = excluded.status,
			invitation_id = excluded.invitation_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			accepted_at = excluded.accepted_at,
			removed_at = excluded.removed_at
	`,
		membership.MembershipID,
		membership.TenantID,
		membership.PrincipalID,
		string(membership.Role),
		string(membership.Status),
		nullString(membership.InvitationID),
		membership.CreatedAt.UTC().Format(time.RFC3339Nano),
		membership.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(membership.AcceptedAt),
		nullableTimeString(membership.RemovedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert membership %s: %w", membership.MembershipID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMemberships(ctx context.Context, filter identity.MembershipFilter) ([]identity.Membership, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT membership_id, tenant_id, principal_id, role, status, invitation_id, created_at, updated_at, accepted_at, removed_at
		FROM memberships
		WHERE 1 = 1
	`
	args := make([]any, 0, 5)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	if filter.Role != "" {
		query += ` AND role = ?`
		args = append(args, string(filter.Role))
	}
	query += ` ORDER BY created_at ASC, membership_id ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()
	items := make([]identity.Membership, 0)
	for rows.Next() {
		item, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertTenantInvitation(ctx context.Context, invitation identity.TenantInvitation) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_invitations (
			invitation_id,
			tenant_id,
			invited_principal_id,
			invited_by_principal_id,
			role,
			status,
			created_at,
			updated_at,
			expires_at,
			decided_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(invitation_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			invited_principal_id = excluded.invited_principal_id,
			invited_by_principal_id = excluded.invited_by_principal_id,
			role = excluded.role,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			decided_at = excluded.decided_at
	`,
		invitation.InvitationID,
		invitation.TenantID,
		invitation.InvitedPrincipalID,
		invitation.InvitedByPrincipalID,
		string(invitation.Role),
		string(invitation.Status),
		invitation.CreatedAt.UTC().Format(time.RFC3339Nano),
		invitation.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(invitation.ExpiresAt),
		nullableTimeString(invitation.DecidedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert tenant invitation %s: %w", invitation.InvitationID, err)
	}
	return nil
}

func (s *SQLiteStore) ListTenantInvitations(ctx context.Context, filter identity.InvitationFilter) ([]identity.TenantInvitation, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT invitation_id, tenant_id, invited_principal_id, invited_by_principal_id, role, status, created_at, updated_at, expires_at, decided_at
		FROM tenant_invitations
		WHERE 1 = 1
	`
	args := make([]any, 0, 5)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.PrincipalID != "" {
		query += ` AND invited_principal_id = ?`
		args = append(args, filter.PrincipalID)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	query += ` ORDER BY created_at ASC, invitation_id ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenant invitations: %w", err)
	}
	defer rows.Close()
	items := make([]identity.TenantInvitation, 0)
	for rows.Next() {
		item, err := scanTenantInvitation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertTokenTenantGrant(ctx context.Context, grant identity.TokenTenantGrant) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO token_tenant_grants (
			grant_id,
			token_id,
			tenant_id,
			is_default,
			status,
			created_at,
			updated_at,
			revoked_at,
			granted_by_principal_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(grant_id) DO UPDATE SET
			token_id = excluded.token_id,
			tenant_id = excluded.tenant_id,
			is_default = excluded.is_default,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			revoked_at = excluded.revoked_at,
			granted_by_principal_id = excluded.granted_by_principal_id
	`,
		grant.GrantID,
		grant.TokenID,
		grant.TenantID,
		boolToInt(grant.IsDefault),
		string(grant.Status),
		grant.CreatedAt.UTC().Format(time.RFC3339Nano),
		grant.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(grant.RevokedAt),
		nullString(grant.GrantedByPrincipalID),
	)
	if err != nil {
		return fmt.Errorf("upsert token tenant grant %s: %w", grant.GrantID, err)
	}
	return nil
}

func (s *SQLiteStore) ListTokenTenantGrants(ctx context.Context, tokenID string) ([]identity.TokenTenantGrant, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT grant_id, token_id, tenant_id, is_default, status, created_at, updated_at, revoked_at, granted_by_principal_id
		FROM token_tenant_grants
		WHERE token_id = ?
		ORDER BY created_at ASC, grant_id ASC
	`, tokenID)
	if err != nil {
		return nil, fmt.Errorf("list token tenant grants: %w", err)
	}
	defer rows.Close()
	items := make([]identity.TokenTenantGrant, 0)
	for rows.Next() {
		item, err := scanTokenTenantGrant(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) AppendTenantAuditEvent(ctx context.Context, event identity.TenantAuditEvent) (identity.TenantAuditEvent, error) {
	if s == nil {
		return event, nil
	}
	if event.AuditEventID == "" {
		event.AuditEventID = newTenantAuditEventID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	documentJSON, err := marshalJSON(event.Document)
	if err != nil {
		return identity.TenantAuditEvent{}, fmt.Errorf("encode tenant audit event %s document: %w", event.AuditEventID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tenant_audit_events (
			audit_event_id,
			event_kind,
			tenant_id,
			principal_id,
			target_principal_id,
			token_id,
			outcome,
			reason_code,
			created_at,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.AuditEventID,
		event.EventKind,
		nullString(event.TenantID),
		nullString(event.PrincipalID),
		nullString(event.TargetPrincipalID),
		nullString(event.TokenID),
		event.Outcome,
		event.ReasonCode,
		event.CreatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
	)
	if err != nil {
		return identity.TenantAuditEvent{}, fmt.Errorf("append tenant audit event %s: %w", event.AuditEventID, err)
	}
	return event, nil
}

func (s *SQLiteStore) ListTenantAuditEvents(ctx context.Context, filter identity.AuditEventFilter) ([]identity.TenantAuditEvent, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT audit_event_id, event_kind, tenant_id, principal_id, target_principal_id, token_id, outcome, reason_code, created_at, document_json
		FROM tenant_audit_events
		WHERE 1 = 1
	`
	args := make([]any, 0, 6)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.PrincipalID != "" {
		query += ` AND principal_id = ?`
		args = append(args, filter.PrincipalID)
	}
	if filter.TokenID != "" {
		query += ` AND token_id = ?`
		args = append(args, filter.TokenID)
	}
	if filter.EventKind != "" {
		query += ` AND event_kind = ?`
		args = append(args, filter.EventKind)
	}
	if filter.Outcome != "" {
		query += ` AND outcome = ?`
		args = append(args, filter.Outcome)
	}
	query += ` ORDER BY created_at DESC, audit_event_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenant audit events: %w", err)
	}
	defer rows.Close()
	items := make([]identity.TenantAuditEvent, 0)
	for rows.Next() {
		item, err := scanTenantAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertApproval(ctx context.Context, approval policy.Approval) error {
	if s == nil {
		return nil
	}

	var resolvedAt sql.NullString
	if approval.ResolvedAt != nil {
		resolvedAt = sql.NullString{String: approval.ResolvedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	integrationBindingsJSON, err := marshalJSON(approval.IntegrationBindings)
	if err != nil {
		return fmt.Errorf("marshal approval integration bindings: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO approvals (
			approval_id,
			action,
			resource_kind,
			resource_id,
			reason,
			requested_by,
			status,
			created_at,
			updated_at,
			resolved_at,
			resolution,
			comment,
			integration_bindings_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(approval_id) DO UPDATE SET
			action = excluded.action,
			resource_kind = excluded.resource_kind,
			resource_id = excluded.resource_id,
			reason = excluded.reason,
			requested_by = excluded.requested_by,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			resolved_at = excluded.resolved_at,
			resolution = excluded.resolution,
			comment = excluded.comment,
			integration_bindings_json = excluded.integration_bindings_json,
			tenant_id = COALESCE(approvals.tenant_id, excluded.tenant_id)
	`,
		approval.ApprovalID,
		approval.Action,
		nullString(approval.ResourceKind),
		nullString(approval.ResourceID),
		approval.Reason,
		nullString(approval.RequestedBy),
		string(approval.Status),
		approval.CreatedAt.UTC().Format(time.RFC3339Nano),
		approval.UpdatedAt.UTC().Format(time.RFC3339Nano),
		resolvedAt,
		nullString(approval.Resolution),
		nullString(approval.Comment),
		integrationBindingsJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert approval %s: %w", approval.ApprovalID, err)
	}

	return nil
}

func (s *SQLiteStore) ListApprovals(ctx context.Context) ([]policy.Approval, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT approval_id, action, resource_kind, resource_id, reason, requested_by, status, created_at, updated_at, resolved_at, resolution, comment, integration_bindings_json
		FROM approvals
		ORDER BY created_at ASC, approval_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	items := make([]policy.Approval, 0)
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, approval)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertDecision(ctx context.Context, decision policy.Decision) error {
	if s == nil {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO decisions (
			decision_id,
			action,
			resource_kind,
			resource_id,
			outcome,
			reason,
			approval_id,
			created_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(decision_id) DO UPDATE SET
			action = excluded.action,
			resource_kind = excluded.resource_kind,
			resource_id = excluded.resource_id,
			outcome = excluded.outcome,
			reason = excluded.reason,
			approval_id = excluded.approval_id,
			created_at = excluded.created_at,
			tenant_id = COALESCE(decisions.tenant_id, excluded.tenant_id)
	`,
		decision.DecisionID,
		decision.Action,
		nullString(decision.ResourceKind),
		nullString(decision.ResourceID),
		string(decision.Outcome),
		decision.Reason,
		nullString(decision.ApprovalID),
		decision.CreatedAt.UTC().Format(time.RFC3339Nano),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert decision %s: %w", decision.DecisionID, err)
	}

	return nil
}

func (s *SQLiteStore) ListDecisions(ctx context.Context) ([]policy.Decision, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT decision_id, action, resource_kind, resource_id, outcome, reason, approval_id, created_at
		FROM decisions
		ORDER BY created_at ASC, decision_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()

	items := make([]policy.Decision, 0)
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, decision)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertConnector(ctx context.Context, connector connectors.Connector) error {
	if s == nil {
		return nil
	}

	var nextRestartAt sql.NullString
	if connector.NextRestartAt != nil {
		nextRestartAt = sql.NullString{String: connector.NextRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastRestartAt sql.NullString
	if connector.LastRestartAt != nil {
		lastRestartAt = sql.NullString{String: connector.LastRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastHeartbeatAt sql.NullString
	if connector.LastHeartbeatAt != nil {
		lastHeartbeatAt = sql.NullString{String: connector.LastHeartbeatAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	secretRefsJSON, err := json.Marshal(connector.SecretRefs)
	if err != nil {
		return fmt.Errorf("marshal connector secret refs %s: %w", connector.ConnectorID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO connectors (
			connector_id,
			tenant_id,
			kind,
			display_name,
			status,
			disabled_reason,
			secret_refs_json,
			failure_count,
			restart_count,
			backoff_seconds,
			next_restart_at,
			last_restart_at,
			last_heartbeat_at,
			last_failure_reason,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(connector_id) DO UPDATE SET
			tenant_id = COALESCE(connectors.tenant_id, excluded.tenant_id),
			kind = excluded.kind,
			display_name = excluded.display_name,
			status = excluded.status,
			disabled_reason = excluded.disabled_reason,
			secret_refs_json = excluded.secret_refs_json,
			failure_count = excluded.failure_count,
			restart_count = excluded.restart_count,
			backoff_seconds = excluded.backoff_seconds,
			next_restart_at = excluded.next_restart_at,
			last_restart_at = excluded.last_restart_at,
			last_heartbeat_at = excluded.last_heartbeat_at,
			last_failure_reason = excluded.last_failure_reason,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		connector.ConnectorID,
		coalesceString(connector.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx))),
		connector.Kind,
		connector.DisplayName,
		string(connector.Status),
		nullString(connector.DisabledReason),
		string(secretRefsJSON),
		connector.FailureCount,
		connector.RestartCount,
		connector.BackoffSeconds,
		nextRestartAt,
		lastRestartAt,
		lastHeartbeatAt,
		nullString(connector.LastFailureReason),
		connector.CreatedAt.UTC().Format(time.RFC3339Nano),
		connector.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert connector %s: %w", connector.ConnectorID, err)
	}

	return nil
}

func (s *SQLiteStore) ListConnectors(ctx context.Context) ([]connectors.Connector, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, tenant_id, kind, display_name, status, disabled_reason, secret_refs_json, failure_count, restart_count, backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at, last_failure_reason, created_at, updated_at
		FROM connectors
		ORDER BY created_at ASC, connector_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}
	defer rows.Close()

	items := make([]connectors.Connector, 0)
	for rows.Next() {
		connector, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, connector)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) CreateConnectorMessageIfAbsent(ctx context.Context, message imtypes.MessageRecord) (imtypes.MessageRecord, bool, error) {
	if s == nil {
		return message, true, nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_messages (
			delivery_id,
			connector_id,
			direction,
			external_message_id,
			session_id,
			run_id,
			channel_id,
			peer_id,
			thread_id,
			author_id,
			content,
			status,
			error_text,
			reply_to_external_message_id,
			response_to_delivery_id,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		message.DeliveryID,
		message.ConnectorID,
		string(message.Direction),
		nullString(message.ExternalMessageID),
		nullString(message.SessionID),
		nullString(message.RunID),
		message.ChannelID,
		nullString(message.PeerID),
		nullString(message.ThreadID),
		nullString(message.AuthorID),
		message.Content,
		string(message.Status),
		nullString(message.Error),
		nullString(message.ReplyToExternalMessageID),
		nullString(message.ResponseToDeliveryID),
		message.CreatedAt.UTC().Format(time.RFC3339Nano),
		message.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.TrimSpace(message.ExternalMessageID) != "" && isUniqueConstraintError(err) {
			existing, ok, lookupErr := s.GetConnectorMessageByExternalID(ctx, message.ConnectorID, message.Direction, message.ExternalMessageID)
			if lookupErr != nil {
				return imtypes.MessageRecord{}, false, lookupErr
			}
			if ok {
				return existing, false, nil
			}
		}
		return imtypes.MessageRecord{}, false, fmt.Errorf("insert connector message %s: %w", message.DeliveryID, err)
	}

	if existing, ok, err := s.GetConnectorMessageByExternalID(ctx, message.ConnectorID, message.Direction, message.ExternalMessageID); err != nil {
		return imtypes.MessageRecord{}, false, err
	} else if ok {
		return existing, existing.DeliveryID == message.DeliveryID, nil
	}

	return imtypes.MessageRecord{}, false, fmt.Errorf("load connector message %s after insert", message.DeliveryID)
}

func (s *SQLiteStore) UpsertConnectorMessage(ctx context.Context, message imtypes.MessageRecord) error {
	if s == nil {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connector_messages (
			delivery_id,
			connector_id,
			direction,
			external_message_id,
			session_id,
			run_id,
			channel_id,
			peer_id,
			thread_id,
			author_id,
			content,
			status,
			error_text,
			reply_to_external_message_id,
			response_to_delivery_id,
			created_at,
			updated_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(delivery_id) DO UPDATE SET
			connector_id = excluded.connector_id,
			direction = excluded.direction,
			external_message_id = excluded.external_message_id,
			session_id = excluded.session_id,
			run_id = excluded.run_id,
			channel_id = excluded.channel_id,
			peer_id = excluded.peer_id,
			thread_id = excluded.thread_id,
			author_id = excluded.author_id,
			content = excluded.content,
			status = excluded.status,
			error_text = excluded.error_text,
			reply_to_external_message_id = excluded.reply_to_external_message_id,
			response_to_delivery_id = excluded.response_to_delivery_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			tenant_id = COALESCE(connector_messages.tenant_id, excluded.tenant_id)
	`,
		message.DeliveryID,
		message.ConnectorID,
		string(message.Direction),
		nullString(message.ExternalMessageID),
		nullString(message.SessionID),
		nullString(message.RunID),
		message.ChannelID,
		nullString(message.PeerID),
		nullString(message.ThreadID),
		nullString(message.AuthorID),
		message.Content,
		string(message.Status),
		nullString(message.Error),
		nullString(message.ReplyToExternalMessageID),
		nullString(message.ResponseToDeliveryID),
		message.CreatedAt.UTC().Format(time.RFC3339Nano),
		message.UpdatedAt.UTC().Format(time.RFC3339Nano),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert connector message %s: %w", message.DeliveryID, err)
	}

	return nil
}

func (s *SQLiteStore) GetConnectorMessageByExternalID(ctx context.Context, connectorID string, direction imtypes.DeliveryDirection, externalMessageID string) (imtypes.MessageRecord, bool, error) {
	if s == nil {
		return imtypes.MessageRecord{}, false, nil
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT delivery_id, connector_id, direction, external_message_id, session_id, run_id, channel_id, peer_id, thread_id, author_id, content, status, error_text, reply_to_external_message_id, response_to_delivery_id, created_at, updated_at
		FROM connector_messages
		WHERE connector_id = ? AND direction = ? AND external_message_id = ?
	`,
		connectorID,
		string(direction),
		externalMessageID,
	)

	item, err := scanConnectorMessage(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return imtypes.MessageRecord{}, false, nil
		}
		return imtypes.MessageRecord{}, false, err
	}
	return item, true, nil
}

func (s *SQLiteStore) UpsertCapability(ctx context.Context, capability capabilities.Capability) error {
	if s == nil {
		return nil
	}

	var nextRestartAt sql.NullString
	if capability.NextRestartAt != nil {
		nextRestartAt = sql.NullString{String: capability.NextRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastRestartAt sql.NullString
	if capability.LastRestartAt != nil {
		lastRestartAt = sql.NullString{String: capability.LastRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastHeartbeatAt sql.NullString
	if capability.LastHeartbeatAt != nil {
		lastHeartbeatAt = sql.NullString{String: capability.LastHeartbeatAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO capabilities (
			capability_id,
			kind,
			display_name,
			status,
			failure_count,
			restart_count,
			backoff_seconds,
			next_restart_at,
			last_restart_at,
			last_heartbeat_at,
			last_failure_reason,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(capability_id) DO UPDATE SET
			kind = excluded.kind,
			display_name = excluded.display_name,
			status = excluded.status,
			failure_count = excluded.failure_count,
			restart_count = excluded.restart_count,
			backoff_seconds = excluded.backoff_seconds,
			next_restart_at = excluded.next_restart_at,
			last_restart_at = excluded.last_restart_at,
			last_heartbeat_at = excluded.last_heartbeat_at,
			last_failure_reason = excluded.last_failure_reason,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		capability.CapabilityID,
		capability.Kind,
		capability.DisplayName,
		string(capability.Status),
		capability.FailureCount,
		capability.RestartCount,
		capability.BackoffSeconds,
		nextRestartAt,
		lastRestartAt,
		lastHeartbeatAt,
		nullString(capability.LastFailureReason),
		capability.CreatedAt.UTC().Format(time.RFC3339Nano),
		capability.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert capability %s: %w", capability.CapabilityID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertLLMDispatch(ctx context.Context, dispatch llm.Dispatch) error {
	if s == nil {
		return nil
	}
	if tenantID, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && tenantID != "" {
		return s.UpsertLLMDispatchForTenantSafe(ctx, dispatch, tenantID)
	}

	messagesJSON, err := marshalJSON(dispatch.Messages)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch messages: %w", err)
	}
	usageJSON, err := marshalJSON(dispatch.Usage)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch usage: %w", err)
	}

	var startedAt sql.NullString
	if dispatch.StartedAt != nil {
		startedAt = sql.NullString{String: dispatch.StartedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var completedAt sql.NullString
	if dispatch.CompletedAt != nil {
		completedAt = sql.NullString{String: dispatch.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO llm_dispatches (
			dispatch_id,
			provider,
			model,
			messages_json,
			stream,
			status,
			output_text,
			finish_reason,
			usage_json,
			error_code,
			error_text,
			timeout_ms,
			max_retries,
			attempt_count,
			created_at,
			updated_at,
			started_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dispatch_id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			messages_json = excluded.messages_json,
			stream = excluded.stream,
			status = excluded.status,
			output_text = excluded.output_text,
			finish_reason = excluded.finish_reason,
			usage_json = excluded.usage_json,
			error_code = excluded.error_code,
			error_text = excluded.error_text,
			timeout_ms = excluded.timeout_ms,
			max_retries = excluded.max_retries,
			attempt_count = excluded.attempt_count,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at
	`,
		dispatch.DispatchID,
		dispatch.Provider,
		dispatch.Model,
		messagesJSON,
		dispatch.Stream,
		string(dispatch.Status),
		dispatch.Output,
		nullString(dispatch.FinishReason),
		usageJSON,
		nullString(dispatch.ErrorCode),
		nullString(dispatch.Error),
		dispatch.TimeoutMs,
		dispatch.MaxRetries,
		dispatch.AttemptCount,
		dispatch.CreatedAt.UTC().Format(time.RFC3339Nano),
		dispatch.UpdatedAt.UTC().Format(time.RFC3339Nano),
		startedAt,
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert llm dispatch %s: %w", dispatch.DispatchID, err)
	}

	return nil
}

func (s *SQLiteStore) ListLLMDispatches(ctx context.Context) ([]llm.Dispatch, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT dispatch_id, provider, model, messages_json, stream, status, output_text, finish_reason, usage_json, error_code, error_text, timeout_ms, max_retries, attempt_count, created_at, updated_at, started_at, completed_at
		FROM llm_dispatches
		ORDER BY created_at ASC, dispatch_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list llm dispatches: %w", err)
	}
	defer rows.Close()

	items := make([]llm.Dispatch, 0)
	for rows.Next() {
		dispatch, err := scanLLMDispatch(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, dispatch)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) GetLLMDispatch(ctx context.Context, dispatchID string) (llm.Dispatch, bool, error) {
	if s == nil {
		return llm.Dispatch{}, false, nil
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT dispatch_id, provider, model, messages_json, stream, status, output_text, finish_reason, usage_json, error_code, error_text, timeout_ms, max_retries, attempt_count, created_at, updated_at, started_at, completed_at
		FROM llm_dispatches
		WHERE dispatch_id = ?
	`, dispatchID)

	dispatch, err := scanLLMDispatch(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return llm.Dispatch{}, false, nil
		}
		return llm.Dispatch{}, false, err
	}

	return dispatch, true, nil
}

func (s *SQLiteStore) UpsertProviderCheck(ctx context.Context, check providers.Check) error {
	if s == nil {
		return nil
	}

	usageJSON, err := marshalJSON(check.Usage)
	if err != nil {
		return fmt.Errorf("marshal provider check usage: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO provider_checks (
			check_id,
			provider_id,
			family,
			auth_mode,
			status,
			model,
			endpoint,
			error_class,
			error_code,
			error_message,
			usage_json,
			created_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(check_id) DO UPDATE SET
			provider_id = excluded.provider_id,
			family = excluded.family,
			auth_mode = excluded.auth_mode,
			status = excluded.status,
			model = excluded.model,
			endpoint = excluded.endpoint,
			error_class = excluded.error_class,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			usage_json = excluded.usage_json,
			created_at = excluded.created_at,
			completed_at = excluded.completed_at
	`,
		check.CheckID,
		check.ProviderID,
		string(check.Family),
		string(check.AuthMode),
		string(check.Status),
		check.Model,
		nullString(check.Endpoint),
		nullString(string(check.ErrorClass)),
		nullString(check.ErrorCode),
		nullString(check.ErrorMessage),
		usageJSON,
		check.CreatedAt.UTC().Format(time.RFC3339Nano),
		check.CompletedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert provider check %s: %w", check.CheckID, err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderChecks(ctx context.Context, providerID string) ([]providers.Check, error) {
	if s == nil {
		return []providers.Check{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT check_id, provider_id, family, auth_mode, status, model, endpoint, error_class, error_code, error_message, usage_json, created_at, completed_at
		FROM provider_checks
		WHERE provider_id = ?
		ORDER BY created_at DESC, check_id DESC
	`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list provider checks for %s: %w", providerID, err)
	}
	defer rows.Close()

	items := make([]providers.Check, 0)
	for rows.Next() {
		item, err := scanProviderCheck(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) GetProviderCheck(ctx context.Context, providerID, checkID string) (providers.Check, bool, error) {
	if s == nil {
		return providers.Check{}, false, nil
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT check_id, provider_id, family, auth_mode, status, model, endpoint, error_class, error_code, error_message, usage_json, created_at, completed_at
		FROM provider_checks
		WHERE provider_id = ? AND check_id = ?
	`, providerID, checkID)

	item, err := scanProviderCheck(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return providers.Check{}, false, nil
		}
		return providers.Check{}, false, err
	}
	return item, true, nil
}

func (s *SQLiteStore) UpsertProviderAuthState(ctx context.Context, state providers.AuthState) error {
	if s == nil {
		return nil
	}

	loginCommandJSON, err := marshalJSON(state.LoginCommand)
	if err != nil {
		return fmt.Errorf("marshal provider auth login command: %w", err)
	}
	logoutCommandJSON, err := marshalJSON(state.LogoutCommand)
	if err != nil {
		return fmt.Errorf("marshal provider auth logout command: %w", err)
	}
	metadataJSON, err := marshalJSON(defaultStringMap(state.Metadata))
	if err != nil {
		return fmt.Errorf("marshal provider auth metadata: %w", err)
	}
	sandboxJSON, err := marshalJSON(state.Sandbox)
	if err != nil {
		return fmt.Errorf("marshal provider auth sandbox metadata: %w", err)
	}

	lastCheckedAt := state.LastCheckedAt.UTC()
	if lastCheckedAt.IsZero() {
		lastCheckedAt = time.Now().UTC()
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO provider_auth_states (
			provider_id,
			tenant_id,
			family,
			auth_mode,
			status,
			cli_path,
			cli_available,
			account_label,
			account_id,
			plan,
			auth_method,
			login_command_json,
			logout_command_json,
			last_checked_at,
			last_authenticated_at,
			last_error,
			metadata_json,
			sandbox_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			family = excluded.family,
			auth_mode = excluded.auth_mode,
			status = excluded.status,
			cli_path = excluded.cli_path,
			cli_available = excluded.cli_available,
			account_label = excluded.account_label,
			account_id = excluded.account_id,
			plan = excluded.plan,
			auth_method = excluded.auth_method,
			login_command_json = excluded.login_command_json,
			logout_command_json = excluded.logout_command_json,
			last_checked_at = excluded.last_checked_at,
			last_authenticated_at = excluded.last_authenticated_at,
			last_error = excluded.last_error,
			metadata_json = excluded.metadata_json,
			sandbox_json = excluded.sandbox_json
	`,
		state.ProviderID,
		nullString(state.TenantID),
		string(state.Family),
		string(state.AuthMode),
		string(state.Status),
		nullString(state.CLIPath),
		boolToInt(state.CLIAvailable),
		nullString(state.AccountLabel),
		nullString(state.AccountID),
		nullString(state.Plan),
		nullString(state.AuthMethod),
		loginCommandJSON,
		logoutCommandJSON,
		lastCheckedAt.Format(time.RFC3339Nano),
		nullableTimeString(state.LastAuthenticatedAt),
		nullString(state.LastError),
		metadataJSON,
		sandboxJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert provider auth state %s: %w", state.ProviderID, err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderAuthStates(ctx context.Context) ([]providers.AuthState, error) {
	if s == nil {
		return []providers.AuthState{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, tenant_id, family, auth_mode, status, cli_path, cli_available, account_label, account_id, plan, auth_method, login_command_json, logout_command_json, last_checked_at, last_authenticated_at, last_error, metadata_json, sandbox_json
		FROM provider_auth_states
		ORDER BY provider_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list provider auth states: %w", err)
	}
	defer rows.Close()

	items := make([]providers.AuthState, 0)
	for rows.Next() {
		item, err := scanProviderAuthState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ReplaceProviderModels(ctx context.Context, providerID string, models []providers.Model) error {
	if s == nil {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provider model replace transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_models WHERE provider_id = ?`, providerID); err != nil {
		return fmt.Errorf("delete provider models for %s: %w", providerID, err)
	}

	for _, model := range models {
		reasoningLevelsJSON, err := marshalJSON(model.ReasoningLevels)
		if err != nil {
			return fmt.Errorf("marshal reasoning levels for %s/%s: %w", providerID, model.ModelID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO provider_models (
				provider_id,
				model_id,
				display_name,
				description,
				default_flag,
				available_flag,
				source,
				chat,
				stream,
				coding,
				tool_use,
				reasoning_levels_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			model.ProviderID,
			model.ModelID,
			model.DisplayName,
			nullString(model.Description),
			boolToInt(model.Default),
			boolToInt(model.Available),
			model.Source,
			boolToInt(model.Chat),
			boolToInt(model.Stream),
			boolToInt(model.Coding),
			boolToInt(model.ToolUse),
			reasoningLevelsJSON,
		); err != nil {
			return fmt.Errorf("insert provider model %s/%s: %w", providerID, model.ModelID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider model replace transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderModels(ctx context.Context) ([]providers.Model, error) {
	if s == nil {
		return []providers.Model{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, model_id, display_name, description, default_flag, available_flag, source, chat, stream, coding, tool_use, reasoning_levels_json
		FROM provider_models
		ORDER BY provider_id ASC, model_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list provider models: %w", err)
	}
	defer rows.Close()

	items := make([]providers.Model, 0)
	for rows.Next() {
		item, err := scanProviderModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ListProviderModelsByProvider(ctx context.Context, providerID string) ([]providers.Model, error) {
	if s == nil {
		return []providers.Model{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, model_id, display_name, description, default_flag, available_flag, source, chat, stream, coding, tool_use, reasoning_levels_json
		FROM provider_models
		WHERE provider_id = ?
		ORDER BY model_id ASC
	`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list provider models for %s: %w", providerID, err)
	}
	defer rows.Close()

	items := make([]providers.Model, 0)
	for rows.Next() {
		item, err := scanProviderModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertProviderPreference(ctx context.Context, preference providers.Preference) error {
	if s == nil {
		return nil
	}

	updatedAt := preference.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_preferences (provider_id, default_model, updated_at, tenant_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			default_model = excluded.default_model,
			updated_at = excluded.updated_at,
			tenant_id = COALESCE(provider_preferences.tenant_id, excluded.tenant_id)
	`, preference.ProviderID, preference.DefaultModel, updatedAt.Format(time.RFC3339Nano), s.ResolveDefaultTenantBinding(ctx))
	if err != nil {
		return fmt.Errorf("upsert provider preference %s: %w", preference.ProviderID, err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderPreferences(ctx context.Context) ([]providers.Preference, error) {
	if s == nil {
		return []providers.Preference{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provider_id, default_model, updated_at
		FROM provider_preferences
		ORDER BY provider_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list provider preferences: %w", err)
	}
	defer rows.Close()

	items := make([]providers.Preference, 0)
	for rows.Next() {
		item, err := scanProviderPreference(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ListCapabilities(ctx context.Context) ([]capabilities.Capability, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT capability_id, kind, display_name, status, failure_count, restart_count, backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at, last_failure_reason, created_at, updated_at
		FROM capabilities
		ORDER BY created_at ASC, capability_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list capabilities: %w", err)
	}
	defer rows.Close()

	items := make([]capabilities.Capability, 0)
	for rows.Next() {
		capability, err := scanCapability(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, capability)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) ListSessions(ctx context.Context) ([]router.Session, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, kind, status, channel, account_id, peer_id, thread_id, routing_key, generation, created_at, updated_at, last_active_at, last_reset_at
		FROM sessions
		ORDER BY created_at ASC, session_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]router.Session, 0)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// Store.ListRuns was removed by Roadmap 35 (T028). The tenantless scan
// violated FR-006 (cross-tenant isolation). Use
// ListRunsForTenantRaw / tenancy.Runtime.ListRunsForTenant instead.

func (s *SQLiteStore) UpsertWorkflow(ctx context.Context, workflow orchestration.Workflow) error {
	if s == nil {
		return nil
	}

	document, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("marshal workflow %s: %w", workflow.WorkflowID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workflows (
			workflow_id,
			run_id,
			schedule_id,
			schedule_attempt_id,
			environment_scope,
			goal,
			status,
			plan_summary,
			failure_summary,
			created_at,
			updated_at,
			started_at,
			completed_at,
			interrupted_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id) DO UPDATE SET
			run_id = excluded.run_id,
			schedule_id = excluded.schedule_id,
			schedule_attempt_id = excluded.schedule_attempt_id,
			environment_scope = excluded.environment_scope,
			goal = excluded.goal,
			status = excluded.status,
			plan_summary = excluded.plan_summary,
			failure_summary = excluded.failure_summary,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			interrupted_at = excluded.interrupted_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(workflows.tenant_id, excluded.tenant_id)
	`,
		workflow.WorkflowID,
		workflow.RunID,
		nullString(workflow.ScheduleID),
		nullString(workflow.ScheduleAttemptID),
		workflow.EnvironmentScope,
		workflow.Goal,
		string(workflow.Status),
		nullString(workflow.PlanSummary),
		nullString(workflow.FailureSummary),
		workflow.CreatedAt.UTC().Format(time.RFC3339Nano),
		workflow.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(workflow.StartedAt),
		nullableTimeString(workflow.CompletedAt),
		nullableTimeString(workflow.InterruptedAt),
		string(document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert workflow %s: %w", workflow.WorkflowID, err)
	}
	return nil
}

func (s *SQLiteStore) UpsertIntegration(ctx context.Context, item integrations.Resource) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal integration %s: %w", item.IntegrationID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integrations (
			integration_id,
			domain_kind,
			environment_scope,
			account_key,
			backend_kind,
			readiness_status,
			canonical_default,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(integration_id) DO UPDATE SET
			domain_kind = excluded.domain_kind,
			environment_scope = excluded.environment_scope,
			account_key = excluded.account_key,
			backend_kind = excluded.backend_kind,
			readiness_status = excluded.readiness_status,
			canonical_default = excluded.canonical_default,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(integrations.tenant_id, excluded.tenant_id)
	`,
		item.IntegrationID,
		item.DomainKind,
		item.EnvironmentScope,
		nullString(item.AccountBinding.AccountKey),
		string(item.BackendBinding.BackendKind),
		string(item.ReadinessStatus),
		boolToInt(item.CanonicalDefault),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx))),
	)
	if err != nil {
		return fmt.Errorf("upsert integration %s: %w", item.IntegrationID, err)
	}
	return nil
}

func (s *SQLiteStore) ListIntegrations(ctx context.Context, environmentScope string) ([]integrations.Resource, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT integration_id, domain_kind, environment_scope, account_key, backend_kind, readiness_status, canonical_default, updated_at, document_json
		FROM integrations
		WHERE environment_scope = ?
		ORDER BY updated_at ASC, integration_id ASC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list integrations for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]integrations.Resource, 0)
	for rows.Next() {
		record, err := scanIntegrationRecord(rows)
		if err != nil {
			return nil, err
		}
		var item integrations.Resource
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode integration %s: %w", record.IntegrationID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertCalendarAccount(ctx context.Context, item calendar.AccountProjection) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal calendar account %s: %w", item.CalendarAccountID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO calendar_accounts (
			calendar_account_id,
			integration_id,
			environment_scope,
			account_key,
			readiness_status,
			canonical_default,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(calendar_account_id) DO UPDATE SET
			integration_id = excluded.integration_id,
			environment_scope = excluded.environment_scope,
			account_key = excluded.account_key,
			readiness_status = excluded.readiness_status,
			canonical_default = excluded.canonical_default,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(calendar_accounts.tenant_id, excluded.tenant_id)
	`,
		item.CalendarAccountID,
		item.IntegrationID,
		item.EnvironmentScope,
		nullString(item.AccountKey),
		item.ReadinessStatus,
		boolToInt(item.CanonicalDefault),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert calendar account %s: %w", item.CalendarAccountID, err)
	}
	return nil
}

func (s *SQLiteStore) ListCalendarAccounts(ctx context.Context, environmentScope string) ([]calendar.AccountProjection, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT calendar_account_id, integration_id, environment_scope, account_key, readiness_status, canonical_default, updated_at, document_json
		FROM calendar_accounts
		WHERE environment_scope = ?
		ORDER BY updated_at ASC, calendar_account_id ASC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list calendar accounts for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]calendar.AccountProjection, 0)
	for rows.Next() {
		record, err := scanCalendarAccountRecord(rows)
		if err != nil {
			return nil, err
		}
		var item calendar.AccountProjection
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode calendar account %s: %w", record.CalendarAccountID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertCalendarOperation(ctx context.Context, item calendar.Operation) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal calendar operation %s: %w", item.OperationID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO calendar_operations (
			operation_id,
			integration_id,
			calendar_account_id,
			environment_scope,
			operation_class,
			status,
			external_event_id,
			run_id,
			workflow_id,
			schedule_id,
			delivery_id,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET
			integration_id = excluded.integration_id,
			calendar_account_id = excluded.calendar_account_id,
			environment_scope = excluded.environment_scope,
			operation_class = excluded.operation_class,
			status = excluded.status,
			external_event_id = excluded.external_event_id,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			schedule_id = excluded.schedule_id,
			delivery_id = excluded.delivery_id,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(calendar_operations.tenant_id, excluded.tenant_id)
	`,
		item.OperationID,
		item.IntegrationID,
		item.CalendarAccountID,
		item.EnvironmentScope,
		string(item.OperationClass),
		string(item.Status),
		nullString(item.ExternalEventID),
		nullString(item.RunID),
		nullString(item.WorkflowID),
		nullString(item.ScheduleID),
		nullString(item.DeliveryID),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert calendar operation %s: %w", item.OperationID, err)
	}
	return nil
}

func (s *SQLiteStore) ListCalendarOperations(ctx context.Context, environmentScope string, filter CalendarOperationFilter) ([]calendar.Operation, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT operation_id, integration_id, calendar_account_id, environment_scope, operation_class, status, external_event_id, run_id, workflow_id, schedule_id, delivery_id, updated_at, document_json
		FROM calendar_operations
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if trimmed := strings.TrimSpace(filter.IntegrationID); trimmed != "" {
		query += ` AND integration_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.RunID); trimmed != "" {
		query += ` AND run_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.WorkflowID); trimmed != "" {
		query += ` AND workflow_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ScheduleID); trimmed != "" {
		query += ` AND schedule_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.DeliveryID); trimmed != "" {
		query += ` AND delivery_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.OperationClass); trimmed != "" {
		query += ` AND operation_class = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.Status); trimmed != "" {
		query += ` AND status = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ExternalEventID); trimmed != "" {
		query += ` AND external_event_id = ?`
		args = append(args, trimmed)
	}
	query += ` ORDER BY updated_at DESC, operation_id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list calendar operations for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]calendar.Operation, 0)
	for rows.Next() {
		record, err := scanCalendarOperationRecord(rows)
		if err != nil {
			return nil, err
		}
		var item calendar.Operation
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode calendar operation %s: %w", record.OperationID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetCalendarOperationByID(ctx context.Context, environmentScope, operationID string) (calendar.Operation, bool, error) {
	items, err := s.ListCalendarOperations(ctx, environmentScope, CalendarOperationFilter{})
	if err != nil {
		return calendar.Operation{}, false, err
	}
	for _, item := range items {
		if item.OperationID == strings.TrimSpace(operationID) {
			return item, true, nil
		}
	}
	return calendar.Operation{}, false, nil
}

func (s *SQLiteStore) UpsertCalendarArtifact(ctx context.Context, item calendar.Artifact) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal calendar artifact %s: %w", item.ArtifactID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO calendar_artifacts (
			artifact_id,
			operation_id,
			integration_id,
			environment_scope,
			kind,
			external_event_id,
			created_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artifact_id) DO UPDATE SET
			operation_id = excluded.operation_id,
			integration_id = excluded.integration_id,
			environment_scope = excluded.environment_scope,
			kind = excluded.kind,
			external_event_id = excluded.external_event_id,
			created_at = excluded.created_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(calendar_artifacts.tenant_id, excluded.tenant_id)
	`,
		item.ArtifactID,
		item.OperationID,
		item.IntegrationID,
		item.EnvironmentScope,
		string(item.Kind),
		nullString(item.ExternalEventID),
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert calendar artifact %s: %w", item.ArtifactID, err)
	}
	return nil
}

func (s *SQLiteStore) ListCalendarArtifacts(ctx context.Context, environmentScope, operationID string) ([]calendar.Artifact, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT artifact_id, operation_id, integration_id, environment_scope, kind, external_event_id, created_at, document_json
		FROM calendar_artifacts
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if trimmed := strings.TrimSpace(operationID); trimmed != "" {
		query += ` AND operation_id = ?`
		args = append(args, trimmed)
	}
	query += ` ORDER BY created_at ASC, artifact_id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list calendar artifacts for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]calendar.Artifact, 0)
	for rows.Next() {
		record, err := scanCalendarArtifactRecord(rows)
		if err != nil {
			return nil, err
		}
		var item calendar.Artifact
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode calendar artifact %s: %w", record.ArtifactID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMailAccount(ctx context.Context, item mail.AccountProjection) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal mail account %s: %w", item.MailAccountID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mail_accounts (
			mail_account_id,
			integration_id,
			environment_scope,
			account_key,
			readiness_status,
			canonical_default,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(mail_account_id) DO UPDATE SET
			integration_id = excluded.integration_id,
			environment_scope = excluded.environment_scope,
			account_key = excluded.account_key,
			readiness_status = excluded.readiness_status,
			canonical_default = excluded.canonical_default,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mail_accounts.tenant_id, excluded.tenant_id)
	`,
		item.MailAccountID,
		item.IntegrationID,
		item.EnvironmentScope,
		nullString(item.AccountKey),
		item.ReadinessStatus,
		boolToInt(item.CanonicalDefault),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mail account %s: %w", item.MailAccountID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMailAccounts(ctx context.Context, environmentScope string) ([]mail.AccountProjection, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT mail_account_id, integration_id, environment_scope, account_key, readiness_status, canonical_default, updated_at, document_json
		FROM mail_accounts
		WHERE environment_scope = ?
		ORDER BY updated_at ASC, mail_account_id ASC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list mail accounts for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]mail.AccountProjection, 0)
	for rows.Next() {
		record, err := scanMailAccountRecord(rows)
		if err != nil {
			return nil, err
		}
		var item mail.AccountProjection
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode mail account %s: %w", record.MailAccountID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMailOperation(ctx context.Context, item mail.Operation) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal mail operation %s: %w", item.OperationID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mail_operations (
			operation_id,
			integration_id,
			mail_account_id,
			environment_scope,
			operation_class,
			status,
			result_mode,
			thread_id,
			message_id,
			draft_id,
			run_id,
			workflow_id,
			schedule_id,
			delivery_id,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET
			integration_id = excluded.integration_id,
			mail_account_id = excluded.mail_account_id,
			environment_scope = excluded.environment_scope,
			operation_class = excluded.operation_class,
			status = excluded.status,
			result_mode = excluded.result_mode,
			thread_id = excluded.thread_id,
			message_id = excluded.message_id,
			draft_id = excluded.draft_id,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			schedule_id = excluded.schedule_id,
			delivery_id = excluded.delivery_id,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mail_operations.tenant_id, excluded.tenant_id)
	`,
		item.OperationID,
		item.IntegrationID,
		item.MailAccountID,
		item.EnvironmentScope,
		string(item.OperationClass),
		string(item.Status),
		string(item.ResultMode),
		nullString(item.ThreadID),
		nullString(item.MessageID),
		nullString(item.DraftID),
		nullString(item.RunID),
		nullString(item.WorkflowID),
		nullString(item.ScheduleID),
		nullString(item.DeliveryID),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mail operation %s: %w", item.OperationID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMailOperations(ctx context.Context, environmentScope string, filter MailOperationFilter) ([]mail.Operation, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT operation_id, integration_id, mail_account_id, environment_scope, operation_class, status, result_mode, thread_id, message_id, draft_id, run_id, workflow_id, schedule_id, delivery_id, updated_at, document_json
		FROM mail_operations
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if trimmed := strings.TrimSpace(filter.IntegrationID); trimmed != "" {
		query += ` AND integration_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.RunID); trimmed != "" {
		query += ` AND run_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.WorkflowID); trimmed != "" {
		query += ` AND workflow_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ScheduleID); trimmed != "" {
		query += ` AND schedule_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.DeliveryID); trimmed != "" {
		query += ` AND delivery_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.OperationClass); trimmed != "" {
		query += ` AND operation_class = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.Status); trimmed != "" {
		query += ` AND status = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ResultMode); trimmed != "" {
		query += ` AND result_mode = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ThreadID); trimmed != "" {
		query += ` AND thread_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.MessageID); trimmed != "" {
		query += ` AND message_id = ?`
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.DraftID); trimmed != "" {
		query += ` AND draft_id = ?`
		args = append(args, trimmed)
	}
	query += ` ORDER BY updated_at DESC, operation_id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mail operations for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]mail.Operation, 0)
	for rows.Next() {
		record, err := scanMailOperationRecord(rows)
		if err != nil {
			return nil, err
		}
		var item mail.Operation
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode mail operation %s: %w", record.OperationID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetMailOperationByID(ctx context.Context, environmentScope, operationID string) (mail.Operation, bool, error) {
	items, err := s.ListMailOperations(ctx, environmentScope, MailOperationFilter{})
	if err != nil {
		return mail.Operation{}, false, err
	}
	for _, item := range items {
		if item.OperationID == strings.TrimSpace(operationID) {
			return item, true, nil
		}
	}
	return mail.Operation{}, false, nil
}

func (s *SQLiteStore) UpsertMailArtifact(ctx context.Context, item mail.Artifact) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(item)
	if err != nil {
		return fmt.Errorf("marshal mail artifact %s: %w", item.ArtifactID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mail_artifacts (
			artifact_id,
			operation_id,
			integration_id,
			environment_scope,
			kind,
			thread_id,
			message_id,
			draft_id,
			attachment_ref_id,
			created_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artifact_id) DO UPDATE SET
			operation_id = excluded.operation_id,
			integration_id = excluded.integration_id,
			environment_scope = excluded.environment_scope,
			kind = excluded.kind,
			thread_id = excluded.thread_id,
			message_id = excluded.message_id,
			draft_id = excluded.draft_id,
			attachment_ref_id = excluded.attachment_ref_id,
			created_at = excluded.created_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mail_artifacts.tenant_id, excluded.tenant_id)
	`,
		item.ArtifactID,
		item.OperationID,
		item.IntegrationID,
		item.EnvironmentScope,
		string(item.Kind),
		nullString(item.ThreadID),
		nullString(item.MessageID),
		nullString(item.DraftID),
		nullString(item.AttachmentRefID),
		item.CreatedAt.UTC().Format(time.RFC3339Nano),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mail artifact %s: %w", item.ArtifactID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMailArtifacts(ctx context.Context, environmentScope, operationID string) ([]mail.Artifact, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT artifact_id, operation_id, integration_id, environment_scope, kind, thread_id, message_id, draft_id, attachment_ref_id, created_at, document_json
		FROM mail_artifacts
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if trimmed := strings.TrimSpace(operationID); trimmed != "" {
		query += ` AND operation_id = ?`
		args = append(args, trimmed)
	}
	query += ` ORDER BY created_at ASC, artifact_id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mail artifacts for %s: %w", environmentScope, err)
	}
	defer rows.Close()
	items := make([]mail.Artifact, 0)
	for rows.Next() {
		record, err := scanMailArtifactRecord(rows)
		if err != nil {
			return nil, err
		}
		var item mail.Artifact
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode mail artifact %s: %w", record.ArtifactID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertReminder(ctx context.Context, record ReminderRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reminders (
			reminder_id,
			environment_scope,
			behavior_mode,
			current_state,
			next_due_at,
			active_occurrence_id,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(reminder_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			behavior_mode = excluded.behavior_mode,
			current_state = excluded.current_state,
			next_due_at = excluded.next_due_at,
			active_occurrence_id = excluded.active_occurrence_id,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(reminders.tenant_id, excluded.tenant_id)
	`,
		record.ReminderID,
		record.EnvironmentScope,
		record.BehaviorMode,
		record.CurrentState,
		nullableTimeString(record.NextDueAt),
		nullString(record.ActiveOccurrenceID),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		coalesceTenantBinding(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx))),
	)
	if err != nil {
		return fmt.Errorf("upsert reminder %s: %w", record.ReminderID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReminders(ctx context.Context, environmentScope string) ([]ReminderRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
			SELECT reminder_id, environment_scope, tenant_id, behavior_mode, current_state, next_due_at, active_occurrence_id, updated_at, document_json
		FROM reminders
		WHERE environment_scope = ?
		ORDER BY updated_at DESC, reminder_id DESC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()
	items := make([]ReminderRecord, 0)
	for rows.Next() {
		record, scanErr := scanReminderRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetReminder(ctx context.Context, environmentScope, reminderID string) (ReminderRecord, bool, error) {
	if s == nil {
		return ReminderRecord{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
			SELECT reminder_id, environment_scope, tenant_id, behavior_mode, current_state, next_due_at, active_occurrence_id, updated_at, document_json
		FROM reminders
		WHERE environment_scope = ? AND reminder_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(reminderID))
	record, err := scanReminderRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return ReminderRecord{}, false, nil
		}
		return ReminderRecord{}, false, err
	}
	return record, true, nil
}

func (s *SQLiteStore) UpsertReminderOccurrence(ctx context.Context, record ReminderOccurrenceRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reminder_occurrences (
			occurrence_id,
			reminder_id,
			environment_scope,
			state,
			scheduled_for,
			run_id,
			workflow_id,
			latest_delivery_id,
			latest_delivery_status,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(occurrence_id) DO UPDATE SET
			reminder_id = excluded.reminder_id,
			environment_scope = excluded.environment_scope,
			state = excluded.state,
			scheduled_for = excluded.scheduled_for,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			latest_delivery_id = excluded.latest_delivery_id,
			latest_delivery_status = excluded.latest_delivery_status,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(reminder_occurrences.tenant_id, excluded.tenant_id)
	`,
		record.OccurrenceID,
		record.ReminderID,
		record.EnvironmentScope,
		record.State,
		record.ScheduledFor.UTC().Format(time.RFC3339Nano),
		nullString(record.RunID),
		nullString(record.WorkflowID),
		nullString(record.LatestDeliveryID),
		nullString(record.LatestDeliveryStatus),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert reminder occurrence %s: %w", record.OccurrenceID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReminderOccurrences(ctx context.Context, environmentScope string, filter ReminderOccurrenceFilter) ([]ReminderOccurrenceRecord, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT occurrence_id, reminder_id, environment_scope, state, scheduled_for, run_id, workflow_id, latest_delivery_id, latest_delivery_status, updated_at, document_json
		FROM reminder_occurrences
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if strings.TrimSpace(filter.ReminderID) != "" {
		query += ` AND reminder_id = ?`
		args = append(args, strings.TrimSpace(filter.ReminderID))
	}
	if strings.TrimSpace(filter.State) != "" {
		query += ` AND state = ?`
		args = append(args, strings.TrimSpace(filter.State))
	}
	if strings.TrimSpace(filter.RunID) != "" {
		query += ` AND run_id = ?`
		args = append(args, strings.TrimSpace(filter.RunID))
	}
	if strings.TrimSpace(filter.WorkflowID) != "" {
		query += ` AND workflow_id = ?`
		args = append(args, strings.TrimSpace(filter.WorkflowID))
	}
	if strings.TrimSpace(filter.DeliveryID) != "" {
		query += ` AND latest_delivery_id = ?`
		args = append(args, strings.TrimSpace(filter.DeliveryID))
	}
	if filter.ScheduledBefore != nil {
		query += ` AND scheduled_for <= ?`
		args = append(args, filter.ScheduledBefore.UTC().Format(time.RFC3339Nano))
	}
	if filter.ScheduledAfter != nil {
		query += ` AND scheduled_for >= ?`
		args = append(args, filter.ScheduledAfter.UTC().Format(time.RFC3339Nano))
	}
	query += ` ORDER BY scheduled_for DESC, occurrence_id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list reminder occurrences: %w", err)
	}
	defer rows.Close()
	items := make([]ReminderOccurrenceRecord, 0)
	for rows.Next() {
		record, scanErr := scanReminderOccurrenceRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetReminderOccurrence(ctx context.Context, environmentScope, occurrenceID string) (ReminderOccurrenceRecord, bool, error) {
	if s == nil {
		return ReminderOccurrenceRecord{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT occurrence_id, reminder_id, environment_scope, state, scheduled_for, run_id, workflow_id, latest_delivery_id, latest_delivery_status, updated_at, document_json
		FROM reminder_occurrences
		WHERE environment_scope = ? AND occurrence_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(occurrenceID))
	record, err := scanReminderOccurrenceRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return ReminderOccurrenceRecord{}, false, nil
		}
		return ReminderOccurrenceRecord{}, false, err
	}
	return record, true, nil
}

func (s *SQLiteStore) AppendReminderAction(ctx context.Context, record ReminderActionRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO reminder_actions (
			action_id,
			reminder_id,
			occurrence_id,
			action_kind,
			new_state,
			run_id,
			workflow_id,
			delivery_id,
			created_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ActionID,
		record.ReminderID,
		nullString(record.OccurrenceID),
		record.ActionKind,
		nullString(record.NewState),
		nullString(record.RunID),
		nullString(record.WorkflowID),
		nullString(record.DeliveryID),
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("append reminder action %s: %w", record.ActionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReminderActions(ctx context.Context, environmentScope, reminderID string) ([]ReminderActionRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.action_id, a.reminder_id, a.occurrence_id, a.action_kind, a.new_state, a.run_id, a.workflow_id, a.delivery_id, a.created_at, a.document_json
		FROM reminder_actions a
		INNER JOIN reminders r ON r.reminder_id = a.reminder_id
		WHERE a.reminder_id = ? AND r.environment_scope = ?
		ORDER BY a.created_at DESC, a.action_id DESC
	`, strings.TrimSpace(reminderID), strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list reminder actions: %w", err)
	}
	defer rows.Close()
	items := make([]ReminderActionRecord, 0)
	for rows.Next() {
		record, scanErr := scanReminderActionRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) ReplaceWorkflowSteps(ctx context.Context, workflowID string, steps []orchestration.WorkflowStep) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace workflow steps %s: %w", workflowID, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_steps WHERE workflow_id = ?`, workflowID); err != nil {
		return fmt.Errorf("delete workflow steps %s: %w", workflowID, err)
	}
	for _, step := range steps {
		document, marshalErr := json.Marshal(step)
		if marshalErr != nil {
			err = fmt.Errorf("marshal workflow step %s: %w", step.WorkflowStepID, marshalErr)
			return err
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO workflow_steps (
				workflow_step_id,
				workflow_id,
				position,
				status,
				runtime_step_id,
				active_tool_call_id,
				attempt_count,
				max_attempts,
				last_failure_class,
				blocked_reason,
				document_json,
				tenant_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			step.WorkflowStepID,
			workflowID,
			step.Position,
			string(step.Status),
			nullString(step.RuntimeStepID),
			nullString(step.ActiveToolCallID),
			step.AttemptCount,
			step.MaxAttempts,
			nullString(step.LastFailureClass),
			nullString(step.BlockedReason),
			string(document),
			s.ResolveDefaultTenantBinding(ctx),
		); err != nil {
			return fmt.Errorf("insert workflow step %s: %w", step.WorkflowStepID, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit replace workflow steps %s: %w", workflowID, err)
	}
	return nil
}

func (s *SQLiteStore) ReplaceWorkflowDependencies(ctx context.Context, workflowID string, items []orchestration.Dependency) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace workflow dependencies %s: %w", workflowID, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_dependencies WHERE workflow_id = ?`, workflowID); err != nil {
		return fmt.Errorf("delete workflow dependencies %s: %w", workflowID, err)
	}
	for _, item := range items {
		document, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			err = fmt.Errorf("marshal workflow dependency %s: %w", item.DependencyID, marshalErr)
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO workflow_dependencies (dependency_id, workflow_id, document_json, tenant_id) VALUES (?, ?, ?, ?)`, item.DependencyID, workflowID, string(document), s.ResolveDefaultTenantBinding(ctx)); err != nil {
			return fmt.Errorf("insert workflow dependency %s: %w", item.DependencyID, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit replace workflow dependencies %s: %w", workflowID, err)
	}
	return nil
}

func (s *SQLiteStore) ReplaceWorkflowHandoffs(ctx context.Context, workflowID string, items []orchestration.Handoff) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace workflow handoffs %s: %w", workflowID, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_handoffs WHERE workflow_id = ?`, workflowID); err != nil {
		return fmt.Errorf("delete workflow handoffs %s: %w", workflowID, err)
	}
	for _, item := range items {
		document, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			err = fmt.Errorf("marshal workflow handoff %s: %w", item.HandoffID, marshalErr)
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO workflow_handoffs (handoff_id, workflow_id, status, document_json, tenant_id) VALUES (?, ?, ?, ?, ?)`, item.HandoffID, workflowID, string(item.Status), string(document), s.ResolveDefaultTenantBinding(ctx)); err != nil {
			return fmt.Errorf("insert workflow handoff %s: %w", item.HandoffID, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit replace workflow handoffs %s: %w", workflowID, err)
	}
	return nil
}

func (s *SQLiteStore) ListWorkflows(ctx context.Context, environmentScope, runID string) ([]orchestration.Workflow, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT workflow_id, run_id, schedule_id, schedule_attempt_id, environment_scope, goal, status, plan_summary, failure_summary, created_at, updated_at, started_at, completed_at, interrupted_at, document_json
		FROM workflows
		WHERE environment_scope = ? AND run_id = ?
		ORDER BY created_at ASC, workflow_id ASC
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID))
	if err != nil {
		return nil, fmt.Errorf("list workflows for run %s: %w", runID, err)
	}
	defer rows.Close()

	records := make([]WorkflowRecord, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]orchestration.Workflow, 0, len(records))
	for _, record := range records {
		workflow, decodeErr := s.decodeWorkflowRecord(ctx, record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, workflow)
	}
	return items, nil
}

func (s *SQLiteStore) GetWorkflow(ctx context.Context, environmentScope, runID, workflowID string) (orchestration.Workflow, bool, error) {
	if s == nil {
		return orchestration.Workflow{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT workflow_id, run_id, schedule_id, schedule_attempt_id, environment_scope, goal, status, plan_summary, failure_summary, created_at, updated_at, started_at, completed_at, interrupted_at, document_json
		FROM workflows
		WHERE environment_scope = ? AND run_id = ? AND workflow_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID), strings.TrimSpace(workflowID))
	record, err := scanWorkflowRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return orchestration.Workflow{}, false, nil
		}
		return orchestration.Workflow{}, false, err
	}
	workflow, err := s.decodeWorkflowRecord(ctx, record)
	if err != nil {
		return orchestration.Workflow{}, false, err
	}
	return workflow, true, nil
}

func (s *SQLiteStore) GetWorkflowByID(ctx context.Context, environmentScope, workflowID string) (orchestration.Workflow, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT workflow_id, run_id, schedule_id, schedule_attempt_id, environment_scope, goal, status, plan_summary, failure_summary, created_at, updated_at, started_at, completed_at, interrupted_at, document_json
		FROM workflows
		WHERE environment_scope = ? AND workflow_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(workflowID))
	record, err := scanWorkflowRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return orchestration.Workflow{}, false, nil
		}
		return orchestration.Workflow{}, false, err
	}
	item, err := s.decodeWorkflowRecord(ctx, record)
	if err != nil {
		return orchestration.Workflow{}, false, err
	}
	return item, true, nil
}

func (s *SQLiteStore) MarkInFlightWorkflowsInterrupted(ctx context.Context, environmentScope string, interruptedAt time.Time) ([]orchestration.Workflow, error) {
	if s == nil {
		return nil, nil
	}
	items, err := s.listInterruptibleWorkflows(ctx, environmentScope)
	if err != nil {
		return nil, err
	}
	updated := make([]orchestration.Workflow, 0, len(items))
	for _, workflow := range items {
		timestamp := interruptedAt.UTC()
		workflow.Status = orchestration.WorkflowStatusInterrupted
		workflow.UpdatedAt = timestamp
		workflow.InterruptedAt = &timestamp
		for idx := range workflow.Steps {
			switch workflow.Steps[idx].Status {
			case orchestration.StepStatusRunning, orchestration.StepStatusReady, orchestration.StepStatusWaitingDependency:
				workflow.Steps[idx].Status = orchestration.StepStatusInterrupted
				workflow.Steps[idx].UpdatedAt = timestamp
			}
		}
		for idx := range workflow.Handoffs {
			if workflow.Handoffs[idx].Status == orchestration.HandoffStatusPending {
				workflow.Handoffs[idx].Status = orchestration.HandoffStatusInvalid
				workflow.Handoffs[idx].InvalidReason = "daemon_restart_interrupted_workflow"
			}
		}
		if err := s.UpsertWorkflow(ctx, workflow); err != nil {
			return nil, err
		}
		if err := s.ReplaceWorkflowSteps(ctx, workflow.WorkflowID, workflow.Steps); err != nil {
			return nil, err
		}
		if err := s.ReplaceWorkflowHandoffs(ctx, workflow.WorkflowID, workflow.Handoffs); err != nil {
			return nil, err
		}
		updated = append(updated, workflow)
	}
	return updated, nil
}

func (s *SQLiteStore) listInterruptibleWorkflows(ctx context.Context, environmentScope string) ([]orchestration.Workflow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workflow_id, run_id, schedule_id, schedule_attempt_id, environment_scope, goal, status, plan_summary, failure_summary, created_at, updated_at, started_at, completed_at, interrupted_at, document_json
		FROM workflows
		WHERE environment_scope = ?
		  AND status IN (?, ?, ?)
		ORDER BY created_at ASC, workflow_id ASC
	`,
		strings.TrimSpace(environmentScope),
		string(orchestration.WorkflowStatusPlanned),
		string(orchestration.WorkflowStatusRunning),
		string(orchestration.WorkflowStatusBlocked),
	)
	if err != nil {
		return nil, fmt.Errorf("list interruptible workflows: %w", err)
	}
	defer rows.Close()
	records := make([]WorkflowRecord, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]orchestration.Workflow, 0, len(records))
	for _, record := range records {
		workflow, decodeErr := s.decodeWorkflowRecord(ctx, record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, workflow)
	}
	return items, nil
}

func (s *SQLiteStore) decodeWorkflowRecord(ctx context.Context, record WorkflowRecord) (orchestration.Workflow, error) {
	var workflow orchestration.Workflow
	if len(record.Document) > 0 {
		if err := json.Unmarshal(record.Document, &workflow); err != nil {
			return orchestration.Workflow{}, fmt.Errorf("decode workflow %s: %w", record.WorkflowID, err)
		}
	}
	if workflow.WorkflowID == "" {
		workflow = orchestration.Workflow{
			WorkflowID:        record.WorkflowID,
			RunID:             record.RunID,
			ScheduleID:        record.ScheduleID,
			ScheduleAttemptID: record.ScheduleAttemptID,
			EnvironmentScope:  record.EnvironmentScope,
			Goal:              record.Goal,
			Status:            orchestration.WorkflowStatus(record.Status),
			PlanSummary:       record.PlanSummary,
			FailureSummary:    record.FailureSummary,
			CreatedAt:         record.CreatedAt,
			UpdatedAt:         record.UpdatedAt,
			StartedAt:         record.StartedAt,
			CompletedAt:       record.CompletedAt,
			InterruptedAt:     record.InterruptedAt,
		}
	}
	workflow.ScheduleID = record.ScheduleID
	workflow.ScheduleAttemptID = record.ScheduleAttemptID
	workflow.EnvironmentScope = record.EnvironmentScope
	workflow.Status = orchestration.WorkflowStatus(record.Status)
	workflow.PlanSummary = record.PlanSummary
	workflow.FailureSummary = record.FailureSummary
	workflow.CreatedAt = record.CreatedAt
	workflow.UpdatedAt = record.UpdatedAt
	workflow.StartedAt = record.StartedAt
	workflow.CompletedAt = record.CompletedAt
	workflow.InterruptedAt = record.InterruptedAt

	steps, err := s.listWorkflowSteps(ctx, record.WorkflowID)
	if err != nil {
		return orchestration.Workflow{}, err
	}
	dependencies, err := s.listWorkflowDependencies(ctx, record.WorkflowID)
	if err != nil {
		return orchestration.Workflow{}, err
	}
	handoffs, err := s.listWorkflowHandoffs(ctx, record.WorkflowID)
	if err != nil {
		return orchestration.Workflow{}, err
	}
	workflow.Steps = steps
	workflow.Dependencies = dependencies
	workflow.Handoffs = handoffs
	return workflow, nil
}

func (s *SQLiteStore) listWorkflowSteps(ctx context.Context, workflowID string) ([]orchestration.WorkflowStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workflow_step_id, workflow_id, position, status, runtime_step_id, active_tool_call_id, attempt_count, max_attempts, last_failure_class, blocked_reason, document_json
		FROM workflow_steps
		WHERE workflow_id = ?
		ORDER BY position ASC, workflow_step_id ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list workflow steps %s: %w", workflowID, err)
	}
	defer rows.Close()
	items := make([]orchestration.WorkflowStep, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowStepRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		var item orchestration.WorkflowStep
		if len(record.Document) > 0 {
			if err := json.Unmarshal(record.Document, &item); err != nil {
				return nil, fmt.Errorf("decode workflow step %s: %w", record.WorkflowStepID, err)
			}
		}
		item.WorkflowStepID = record.WorkflowStepID
		item.WorkflowID = record.WorkflowID
		item.Position = record.Position
		item.Status = orchestration.StepStatus(record.Status)
		item.RuntimeStepID = record.RuntimeStepID
		item.ActiveToolCallID = record.ActiveToolCallID
		item.AttemptCount = record.AttemptCount
		item.MaxAttempts = record.MaxAttempts
		item.LastFailureClass = record.LastFailureClass
		item.BlockedReason = record.BlockedReason
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) listWorkflowDependencies(ctx context.Context, workflowID string) ([]orchestration.Dependency, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT dependency_id, workflow_id, document_json
		FROM workflow_dependencies
		WHERE workflow_id = ?
		ORDER BY dependency_id ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list workflow dependencies %s: %w", workflowID, err)
	}
	defer rows.Close()
	items := make([]orchestration.Dependency, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowDependencyRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		var item orchestration.Dependency
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode workflow dependency %s: %w", record.DependencyID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) listWorkflowHandoffs(ctx context.Context, workflowID string) ([]orchestration.Handoff, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT handoff_id, workflow_id, status, document_json
		FROM workflow_handoffs
		WHERE workflow_id = ?
		ORDER BY handoff_id ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list workflow handoffs %s: %w", workflowID, err)
	}
	defer rows.Close()
	items := make([]orchestration.Handoff, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowHandoffRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		var item orchestration.Handoff
		if err := json.Unmarshal(record.Document, &item); err != nil {
			return nil, fmt.Errorf("decode workflow handoff %s: %w", record.HandoffID, err)
		}
		item.Status = orchestration.HandoffStatus(record.Status)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertSchedule(ctx context.Context, record ScheduleRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO schedules (
			schedule_id,
			environment_scope,
			kind,
			status,
			target_ref_id,
			timezone,
			next_due_at,
			last_attempt_at,
			last_outcome,
			created_at,
			updated_at,
			paused_at,
			cancelled_at,
			completed_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schedule_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			kind = excluded.kind,
			status = excluded.status,
			target_ref_id = excluded.target_ref_id,
			timezone = excluded.timezone,
			next_due_at = excluded.next_due_at,
			last_attempt_at = excluded.last_attempt_at,
			last_outcome = excluded.last_outcome,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			paused_at = excluded.paused_at,
			cancelled_at = excluded.cancelled_at,
			completed_at = excluded.completed_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(schedules.tenant_id, excluded.tenant_id)
	`,
		record.ScheduleID,
		record.EnvironmentScope,
		record.Kind,
		record.Status,
		record.TargetRefID,
		nullString(record.Timezone),
		nullableTimeString(record.NextDueAt),
		nullableTimeString(record.LastAttemptAt),
		nullString(record.LastOutcome),
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(record.PausedAt),
		nullableTimeString(record.CancelledAt),
		nullableTimeString(record.CompletedAt),
		string(record.Document),
		coalesceTenantBinding(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx))),
	)
	if err != nil {
		return fmt.Errorf("upsert schedule %s: %w", record.ScheduleID, err)
	}
	return nil
}

func (s *SQLiteStore) GetSchedule(ctx context.Context, environmentScope, scheduleID string) (ScheduleRecord, bool, error) {
	if s == nil {
		return ScheduleRecord{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
			SELECT schedule_id, environment_scope, tenant_id, kind, status, target_ref_id, timezone, next_due_at, last_attempt_at, last_outcome, created_at, updated_at, paused_at, cancelled_at, completed_at, document_json
		FROM schedules
		WHERE environment_scope = ? AND schedule_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(scheduleID))
	record, err := scanScheduleRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return ScheduleRecord{}, false, nil
		}
		return ScheduleRecord{}, false, err
	}
	return record, true, nil
}

func (s *SQLiteStore) ListSchedules(ctx context.Context, environmentScope string) ([]ScheduleRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
			SELECT schedule_id, environment_scope, tenant_id, kind, status, target_ref_id, timezone, next_due_at, last_attempt_at, last_outcome, created_at, updated_at, paused_at, cancelled_at, completed_at, document_json
		FROM schedules
		WHERE environment_scope = ?
		ORDER BY created_at ASC, schedule_id ASC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	items := make([]ScheduleRecord, 0)
	for rows.Next() {
		record, scanErr := scanScheduleRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertScheduleTarget(ctx context.Context, record ScheduleTargetRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO schedule_targets (
			target_ref_id,
			schedule_id,
			target_kind,
			revision,
			active,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_ref_id) DO UPDATE SET
			schedule_id = excluded.schedule_id,
			target_kind = excluded.target_kind,
			revision = excluded.revision,
			active = excluded.active,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(schedule_targets.tenant_id, excluded.tenant_id)
	`,
		record.TargetRefID,
		record.ScheduleID,
		record.TargetKind,
		record.Revision,
		boolToInt(record.Active),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert schedule target %s: %w", record.TargetRefID, err)
	}
	return nil
}

func (s *SQLiteStore) GetScheduleTarget(ctx context.Context, scheduleID, targetRefID string) (ScheduleTargetRecord, bool, error) {
	if s == nil {
		return ScheduleTargetRecord{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT target_ref_id, schedule_id, target_kind, revision, active, updated_at, document_json
		FROM schedule_targets
		WHERE schedule_id = ? AND target_ref_id = ?
	`, strings.TrimSpace(scheduleID), strings.TrimSpace(targetRefID))
	record, err := scanScheduleTargetRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			return ScheduleTargetRecord{}, false, nil
		}
		return ScheduleTargetRecord{}, false, err
	}
	return record, true, nil
}

func (s *SQLiteStore) UpsertScheduleDispatchAttempt(ctx context.Context, record ScheduleDispatchAttemptRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO schedule_dispatch_attempts (
			attempt_id,
			schedule_id,
			due_at,
			trigger_source,
			dispatch_status,
			failure_class,
			failure_reason,
			retry_count,
			retry_budget,
			next_retry_at,
			resolved_target_revision,
			run_id,
			workflow_id,
			downstream_status,
			skipped_reason,
			missed_count,
			created_at,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(attempt_id) DO UPDATE SET
			schedule_id = excluded.schedule_id,
			due_at = excluded.due_at,
			trigger_source = excluded.trigger_source,
			dispatch_status = excluded.dispatch_status,
			failure_class = excluded.failure_class,
			failure_reason = excluded.failure_reason,
			retry_count = excluded.retry_count,
			retry_budget = excluded.retry_budget,
			next_retry_at = excluded.next_retry_at,
			resolved_target_revision = excluded.resolved_target_revision,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			downstream_status = excluded.downstream_status,
			skipped_reason = excluded.skipped_reason,
			missed_count = excluded.missed_count,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(schedule_dispatch_attempts.tenant_id, excluded.tenant_id)
	`,
		record.AttemptID,
		record.ScheduleID,
		record.DueAt.UTC().Format(time.RFC3339Nano),
		record.TriggerSource,
		record.DispatchStatus,
		nullString(record.FailureClass),
		nullString(record.FailureReason),
		record.RetryCount,
		record.RetryBudget,
		nullableTimeString(record.NextRetryAt),
		record.ResolvedTargetRevision,
		nullString(record.RunID),
		nullString(record.WorkflowID),
		record.DownstreamStatus,
		nullString(record.SkippedReason),
		record.MissedCount,
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert schedule attempt %s: %w", record.AttemptID, err)
	}
	return nil
}

func (s *SQLiteStore) ListScheduleDispatchAttempts(ctx context.Context, scheduleID string) ([]ScheduleDispatchAttemptRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT attempt_id, schedule_id, due_at, trigger_source, dispatch_status, failure_class, failure_reason, retry_count, retry_budget, next_retry_at, resolved_target_revision, run_id, workflow_id, downstream_status, skipped_reason, missed_count, created_at, updated_at, document_json
		FROM schedule_dispatch_attempts
		WHERE schedule_id = ?
		ORDER BY due_at DESC, created_at DESC, attempt_id DESC
	`, strings.TrimSpace(scheduleID))
	if err != nil {
		return nil, fmt.Errorf("list schedule attempts %s: %w", scheduleID, err)
	}
	defer rows.Close()

	items := make([]ScheduleDispatchAttemptRecord, 0)
	for rows.Next() {
		record, scanErr := scanScheduleDispatchAttemptRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertDeliveryTarget(ctx context.Context, record DeliveryTargetRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO delivery_targets (
			target_id,
			environment_scope,
			target_kind,
			status,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			target_kind = excluded.target_kind,
			status = excluded.status,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(delivery_targets.tenant_id, excluded.tenant_id)
	`,
		record.TargetID,
		record.EnvironmentScope,
		record.TargetKind,
		record.Status,
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert delivery target %s: %w", record.TargetID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDeliveryTargets(ctx context.Context, environmentScope string) ([]DeliveryTargetRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT target_id, environment_scope, target_kind, status, updated_at, document_json
		FROM delivery_targets
		WHERE environment_scope = ?
		ORDER BY updated_at DESC, target_id DESC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list delivery targets: %w", err)
	}
	defer rows.Close()
	items := make([]DeliveryTargetRecord, 0)
	for rows.Next() {
		var item DeliveryTargetRecord
		var updatedAt string
		var document string
		if err := rows.Scan(&item.TargetID, &item.EnvironmentScope, &item.TargetKind, &item.Status, &updatedAt, &document); err != nil {
			return nil, fmt.Errorf("scan delivery target: %w", err)
		}
		item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse delivery target updated_at: %w", err)
		}
		item.Document = []byte(document)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetDeliveryTarget(ctx context.Context, environmentScope, targetID string) (DeliveryTargetRecord, bool, error) {
	if s == nil {
		return DeliveryTargetRecord{}, false, nil
	}
	var item DeliveryTargetRecord
	var updatedAt string
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT target_id, environment_scope, target_kind, status, updated_at, document_json
		FROM delivery_targets
		WHERE environment_scope = ? AND target_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(targetID)).Scan(&item.TargetID, &item.EnvironmentScope, &item.TargetKind, &item.Status, &updatedAt, &document)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryTargetRecord{}, false, nil
	}
	if err != nil {
		return DeliveryTargetRecord{}, false, fmt.Errorf("get delivery target %s: %w", targetID, err)
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return DeliveryTargetRecord{}, false, fmt.Errorf("parse delivery target updated_at: %w", err)
	}
	item.Document = []byte(document)
	return item, true, nil
}

func (s *SQLiteStore) UpsertDeliveryPreference(ctx context.Context, record DeliveryPreferenceRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO delivery_preferences (
			preference_id,
			environment_scope,
			scope_kind,
			integration_id,
			active,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(preference_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			scope_kind = excluded.scope_kind,
			integration_id = excluded.integration_id,
			active = excluded.active,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(delivery_preferences.tenant_id, excluded.tenant_id)
	`,
		record.PreferenceID,
		record.EnvironmentScope,
		record.ScopeKind,
		nullString(record.IntegrationID),
		boolToInt(record.Active),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert delivery preference %s: %w", record.PreferenceID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDeliveryPreferences(ctx context.Context, environmentScope string) ([]DeliveryPreferenceRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT preference_id, environment_scope, scope_kind, integration_id, active, updated_at, document_json
		FROM delivery_preferences
		WHERE environment_scope = ?
		ORDER BY updated_at DESC, preference_id DESC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list delivery preferences: %w", err)
	}
	defer rows.Close()
	items := make([]DeliveryPreferenceRecord, 0)
	for rows.Next() {
		var item DeliveryPreferenceRecord
		var integrationID sql.NullString
		var active int
		var updatedAt string
		var document string
		if err := rows.Scan(&item.PreferenceID, &item.EnvironmentScope, &item.ScopeKind, &integrationID, &active, &updatedAt, &document); err != nil {
			return nil, fmt.Errorf("scan delivery preference: %w", err)
		}
		item.IntegrationID = integrationID.String
		item.Active = active != 0
		item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse delivery preference updated_at: %w", err)
		}
		item.Document = []byte(document)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetDeliveryPreference(ctx context.Context, environmentScope, preferenceID string) (DeliveryPreferenceRecord, bool, error) {
	if s == nil {
		return DeliveryPreferenceRecord{}, false, nil
	}
	var item DeliveryPreferenceRecord
	var integrationID sql.NullString
	var active int
	var updatedAt string
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT preference_id, environment_scope, scope_kind, integration_id, active, updated_at, document_json
		FROM delivery_preferences
		WHERE environment_scope = ? AND preference_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(preferenceID)).Scan(&item.PreferenceID, &item.EnvironmentScope, &item.ScopeKind, &integrationID, &active, &updatedAt, &document)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryPreferenceRecord{}, false, nil
	}
	if err != nil {
		return DeliveryPreferenceRecord{}, false, fmt.Errorf("get delivery preference %s: %w", preferenceID, err)
	}
	item.IntegrationID = integrationID.String
	item.Active = active != 0
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return DeliveryPreferenceRecord{}, false, fmt.Errorf("parse delivery preference updated_at: %w", err)
	}
	item.Document = []byte(document)
	return item, true, nil
}

func (s *SQLiteStore) UpsertDeliveryOutcome(ctx context.Context, record DeliveryOutcomeRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO delivery_outcomes (
			delivery_id,
			environment_scope,
			source_kind,
			source_id,
			run_id,
			workflow_id,
			schedule_id,
			integration_id,
			status,
			chosen_target_id,
			preference_id,
			summary_window_id,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(delivery_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			source_kind = excluded.source_kind,
			source_id = excluded.source_id,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			schedule_id = excluded.schedule_id,
			integration_id = excluded.integration_id,
			status = excluded.status,
			chosen_target_id = excluded.chosen_target_id,
			preference_id = excluded.preference_id,
			summary_window_id = excluded.summary_window_id,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(delivery_outcomes.tenant_id, excluded.tenant_id)
	`,
		record.DeliveryID,
		record.EnvironmentScope,
		record.SourceKind,
		record.SourceID,
		nullString(record.RunID),
		nullString(record.WorkflowID),
		nullString(record.ScheduleID),
		nullString(record.IntegrationID),
		record.Status,
		nullString(record.ChosenTargetID),
		nullString(record.PreferenceID),
		nullString(record.SummaryWindowID),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert delivery outcome %s: %w", record.DeliveryID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDeliveryOutcomes(ctx context.Context, environmentScope string, filter DeliveryOutcomeFilter) ([]DeliveryOutcomeRecord, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT delivery_id, environment_scope, source_kind, source_id, run_id, workflow_id, schedule_id, integration_id, status, chosen_target_id, preference_id, summary_window_id, updated_at, document_json
		FROM delivery_outcomes
		WHERE environment_scope = ?
	`
	args := []any{strings.TrimSpace(environmentScope)}
	if strings.TrimSpace(filter.SourceKind) != "" {
		query += ` AND source_kind = ?`
		args = append(args, strings.TrimSpace(filter.SourceKind))
	}
	if strings.TrimSpace(filter.SourceID) != "" {
		query += ` AND source_id = ?`
		args = append(args, strings.TrimSpace(filter.SourceID))
	}
	if strings.TrimSpace(filter.RunID) != "" {
		query += ` AND run_id = ?`
		args = append(args, strings.TrimSpace(filter.RunID))
	}
	if strings.TrimSpace(filter.WorkflowID) != "" {
		query += ` AND workflow_id = ?`
		args = append(args, strings.TrimSpace(filter.WorkflowID))
	}
	if strings.TrimSpace(filter.ScheduleID) != "" {
		query += ` AND schedule_id = ?`
		args = append(args, strings.TrimSpace(filter.ScheduleID))
	}
	if strings.TrimSpace(filter.IntegrationID) != "" {
		query += ` AND integration_id = ?`
		args = append(args, strings.TrimSpace(filter.IntegrationID))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query += ` AND status = ?`
		args = append(args, strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.TargetID) != "" {
		query += ` AND chosen_target_id = ?`
		args = append(args, strings.TrimSpace(filter.TargetID))
	}
	query += ` ORDER BY updated_at DESC, delivery_id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list delivery outcomes: %w", err)
	}
	defer rows.Close()
	items := make([]DeliveryOutcomeRecord, 0)
	for rows.Next() {
		var item DeliveryOutcomeRecord
		var runID, workflowID, scheduleID, integrationID, chosenTargetID, preferenceID, summaryWindowID sql.NullString
		var updatedAt string
		var document string
		if err := rows.Scan(&item.DeliveryID, &item.EnvironmentScope, &item.SourceKind, &item.SourceID, &runID, &workflowID, &scheduleID, &integrationID, &item.Status, &chosenTargetID, &preferenceID, &summaryWindowID, &updatedAt, &document); err != nil {
			return nil, fmt.Errorf("scan delivery outcome: %w", err)
		}
		item.RunID = runID.String
		item.WorkflowID = workflowID.String
		item.ScheduleID = scheduleID.String
		item.IntegrationID = integrationID.String
		item.ChosenTargetID = chosenTargetID.String
		item.PreferenceID = preferenceID.String
		item.SummaryWindowID = summaryWindowID.String
		item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse delivery outcome updated_at: %w", err)
		}
		item.Document = []byte(document)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetDeliveryOutcome(ctx context.Context, environmentScope, deliveryID string) (DeliveryOutcomeRecord, bool, error) {
	if s == nil {
		return DeliveryOutcomeRecord{}, false, nil
	}
	var item DeliveryOutcomeRecord
	var runID, workflowID, scheduleID, integrationID, chosenTargetID, preferenceID, summaryWindowID sql.NullString
	var updatedAt string
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT delivery_id, environment_scope, source_kind, source_id, run_id, workflow_id, schedule_id, integration_id, status, chosen_target_id, preference_id, summary_window_id, updated_at, document_json
		FROM delivery_outcomes
		WHERE environment_scope = ? AND delivery_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(deliveryID)).Scan(&item.DeliveryID, &item.EnvironmentScope, &item.SourceKind, &item.SourceID, &runID, &workflowID, &scheduleID, &integrationID, &item.Status, &chosenTargetID, &preferenceID, &summaryWindowID, &updatedAt, &document)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryOutcomeRecord{}, false, nil
	}
	if err != nil {
		return DeliveryOutcomeRecord{}, false, fmt.Errorf("get delivery outcome %s: %w", deliveryID, err)
	}
	item.RunID = runID.String
	item.WorkflowID = workflowID.String
	item.ScheduleID = scheduleID.String
	item.IntegrationID = integrationID.String
	item.ChosenTargetID = chosenTargetID.String
	item.PreferenceID = preferenceID.String
	item.SummaryWindowID = summaryWindowID.String
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return DeliveryOutcomeRecord{}, false, fmt.Errorf("parse delivery outcome updated_at: %w", err)
	}
	item.Document = []byte(document)
	return item, true, nil
}

func (s *SQLiteStore) UpsertDeliveryAttempt(ctx context.Context, record DeliveryAttemptRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO delivery_attempts (
			attempt_id,
			delivery_id,
			attempt_number,
			target_id,
			status,
			next_retry_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(attempt_id) DO UPDATE SET
			delivery_id = excluded.delivery_id,
			attempt_number = excluded.attempt_number,
			target_id = excluded.target_id,
			status = excluded.status,
			next_retry_at = excluded.next_retry_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(delivery_attempts.tenant_id, excluded.tenant_id)
	`,
		record.AttemptID,
		record.DeliveryID,
		record.AttemptNumber,
		record.TargetID,
		record.Status,
		nullableTimeString(record.NextRetryAt),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert delivery attempt %s: %w", record.AttemptID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDeliveryAttempts(ctx context.Context, deliveryID string) ([]DeliveryAttemptRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT attempt_id, delivery_id, attempt_number, target_id, status, next_retry_at, document_json
		FROM delivery_attempts
		WHERE delivery_id = ?
		ORDER BY attempt_number ASC, attempt_id ASC
	`, strings.TrimSpace(deliveryID))
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer rows.Close()
	items := make([]DeliveryAttemptRecord, 0)
	for rows.Next() {
		var item DeliveryAttemptRecord
		var nextRetryAt sql.NullString
		var document string
		if err := rows.Scan(&item.AttemptID, &item.DeliveryID, &item.AttemptNumber, &item.TargetID, &item.Status, &nextRetryAt, &document); err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		if nextRetryAt.Valid {
			parsed, err := time.Parse(time.RFC3339Nano, nextRetryAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse delivery attempt next_retry_at: %w", err)
			}
			item.NextRetryAt = &parsed
		}
		item.Document = []byte(document)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertDeliverySummaryWindow(ctx context.Context, record DeliverySummaryWindowRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO delivery_summary_windows (
			summary_window_id,
			environment_scope,
			target_id,
			preference_id,
			status,
			window_ends_at,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(summary_window_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			target_id = excluded.target_id,
			preference_id = excluded.preference_id,
			status = excluded.status,
			window_ends_at = excluded.window_ends_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(delivery_summary_windows.tenant_id, excluded.tenant_id)
	`,
		record.SummaryWindowID,
		record.EnvironmentScope,
		record.TargetID,
		record.PreferenceID,
		record.Status,
		record.WindowEndsAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert delivery summary window %s: %w", record.SummaryWindowID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDeliverySummaryWindows(ctx context.Context, environmentScope string) ([]DeliverySummaryWindowRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT summary_window_id, environment_scope, target_id, preference_id, status, window_ends_at, updated_at, document_json
		FROM delivery_summary_windows
		WHERE environment_scope = ?
		ORDER BY updated_at DESC, summary_window_id DESC
	`, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list delivery summary windows: %w", err)
	}
	defer rows.Close()
	items := make([]DeliverySummaryWindowRecord, 0)
	for rows.Next() {
		var item DeliverySummaryWindowRecord
		var windowEndsAt, updatedAt string
		var document string
		if err := rows.Scan(&item.SummaryWindowID, &item.EnvironmentScope, &item.TargetID, &item.PreferenceID, &item.Status, &windowEndsAt, &updatedAt, &document); err != nil {
			return nil, fmt.Errorf("scan delivery summary window: %w", err)
		}
		item.WindowEndsAt, err = time.Parse(time.RFC3339Nano, windowEndsAt)
		if err != nil {
			return nil, fmt.Errorf("parse delivery summary window window_ends_at: %w", err)
		}
		item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse delivery summary window updated_at: %w", err)
		}
		item.Document = []byte(document)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetDeliverySummaryWindow(ctx context.Context, environmentScope, summaryWindowID string) (DeliverySummaryWindowRecord, bool, error) {
	if s == nil {
		return DeliverySummaryWindowRecord{}, false, nil
	}
	var item DeliverySummaryWindowRecord
	var windowEndsAt, updatedAt string
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT summary_window_id, environment_scope, target_id, preference_id, status, window_ends_at, updated_at, document_json
		FROM delivery_summary_windows
		WHERE environment_scope = ? AND summary_window_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(summaryWindowID)).Scan(&item.SummaryWindowID, &item.EnvironmentScope, &item.TargetID, &item.PreferenceID, &item.Status, &windowEndsAt, &updatedAt, &document)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliverySummaryWindowRecord{}, false, nil
	}
	if err != nil {
		return DeliverySummaryWindowRecord{}, false, fmt.Errorf("get delivery summary window %s: %w", summaryWindowID, err)
	}
	item.WindowEndsAt, err = time.Parse(time.RFC3339Nano, windowEndsAt)
	if err != nil {
		return DeliverySummaryWindowRecord{}, false, fmt.Errorf("parse delivery summary window window_ends_at: %w", err)
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return DeliverySummaryWindowRecord{}, false, fmt.Errorf("parse delivery summary window updated_at: %w", err)
	}
	item.Document = []byte(document)
	return item, true, nil
}

func (s *SQLiteStore) UpsertStep(ctx context.Context, step runtime.Step) error {
	if s == nil {
		return nil
	}
	if tenantID, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && tenantID != "" {
		return s.UpsertStepForTenantSafe(ctx, step, tenantID)
	}

	inputJSON, err := marshalJSON(step.Input)
	if err != nil {
		return fmt.Errorf("marshal step input: %w", err)
	}
	outputJSON, err := marshalJSON(step.Output)
	if err != nil {
		return fmt.Errorf("marshal step output: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO steps (
			step_id,
			run_id,
			workflow_id,
			workflow_step_id,
			attempt,
			title,
			kind,
			status,
			input_json,
			output_json,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(step_id) DO UPDATE SET
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			attempt = excluded.attempt,
			title = excluded.title,
			kind = excluded.kind,
			status = excluded.status,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		step.StepID,
		step.RunID,
		nullString(step.WorkflowID),
		nullString(step.WorkflowStepID),
		step.Attempt,
		step.Title,
		step.Kind,
		string(step.Status),
		inputJSON,
		outputJSON,
		step.CreatedAt.UTC().Format(time.RFC3339Nano),
		step.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert step %s: %w", step.StepID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertToolCall(ctx context.Context, toolCall runtime.ToolCall) error {
	if s == nil {
		return nil
	}
	if tenantID, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && tenantID != "" {
		return s.UpsertToolCallForTenantSafe(ctx, toolCall, tenantID)
	}

	inputJSON, err := marshalJSON(toolCall.Input)
	if err != nil {
		return fmt.Errorf("marshal tool call input: %w", err)
	}
	outputJSON, err := marshalJSON(toolCall.Output)
	if err != nil {
		return fmt.Errorf("marshal tool call output: %w", err)
	}
	sandboxJSON, err := marshalJSON(toolCall.Sandbox)
	if err != nil {
		return fmt.Errorf("marshal tool call sandbox metadata: %w", err)
	}
	integrationBindingsJSON, err := marshalJSON(toolCall.IntegrationBindings)
	if err != nil {
		return fmt.Errorf("marshal tool call integration bindings: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tool_calls (
			tool_call_id,
			run_id,
			step_id,
			workflow_id,
			workflow_step_id,
			attempt,
			computer_use_session_id,
			computer_use_action_id,
			invocation_kind,
			capability_id,
			skill_id,
			mcp_server_id,
			mcp_server_name,
			mcp_tool_name,
			mcp_transport_kind,
			mcp_session_id,
			authorization_result,
			tool_name,
			status,
			sandbox_execution_id,
			failure_class,
			input_json,
			output_json,
			sandbox_json,
			integration_bindings_json,
			error_text,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_call_id) DO UPDATE SET
			run_id = excluded.run_id,
			step_id = excluded.step_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			attempt = excluded.attempt,
			computer_use_session_id = excluded.computer_use_session_id,
			computer_use_action_id = excluded.computer_use_action_id,
			invocation_kind = excluded.invocation_kind,
			capability_id = excluded.capability_id,
			skill_id = excluded.skill_id,
			mcp_server_id = excluded.mcp_server_id,
			mcp_server_name = excluded.mcp_server_name,
			mcp_tool_name = excluded.mcp_tool_name,
			mcp_transport_kind = excluded.mcp_transport_kind,
			mcp_session_id = excluded.mcp_session_id,
			authorization_result = excluded.authorization_result,
			tool_name = excluded.tool_name,
			status = excluded.status,
			sandbox_execution_id = excluded.sandbox_execution_id,
			failure_class = excluded.failure_class,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			sandbox_json = excluded.sandbox_json,
			integration_bindings_json = excluded.integration_bindings_json,
			error_text = excluded.error_text,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		toolCall.ToolCallID,
		toolCall.RunID,
		toolCall.StepID,
		nullString(toolCall.WorkflowID),
		nullString(toolCall.WorkflowStepID),
		toolCall.Attempt,
		nullString(toolCall.ComputerUseSessionID),
		nullString(toolCall.ComputerUseActionID),
		nullString(string(toolCall.InvocationKind)),
		toolCall.CapabilityID,
		nullString(toolCall.SkillID),
		nullString(toolCall.MCPServerID),
		nullString(toolCall.MCPServerName),
		nullString(toolCall.MCPToolName),
		nullString(toolCall.MCPTransportKind),
		nullString(toolCall.MCPSessionID),
		nullString(toolCall.AuthorizationResult),
		toolCall.ToolName,
		string(toolCall.Status),
		nullString(toolCall.SandboxExecutionID),
		nullString(toolCall.FailureClass),
		inputJSON,
		outputJSON,
		sandboxJSON,
		integrationBindingsJSON,
		nullString(toolCall.Error),
		toolCall.CreatedAt.UTC().Format(time.RFC3339Nano),
		toolCall.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert tool call %s: %w", toolCall.ToolCallID, err)
	}

	return nil
}

func (s *SQLiteStore) ListToolCalls(ctx context.Context, runID, stepID string) ([]runtime.ToolCall, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_call_id, run_id, step_id, workflow_id, workflow_step_id, attempt, computer_use_session_id, computer_use_action_id, invocation_kind, capability_id, skill_id, mcp_server_id, mcp_server_name, mcp_tool_name, mcp_transport_kind, mcp_session_id, authorization_result, tool_name, status, sandbox_execution_id, failure_class, input_json, output_json, sandbox_json, integration_bindings_json, error_text, created_at, updated_at
		FROM tool_calls
		WHERE run_id = ? AND step_id = ?
		ORDER BY created_at ASC, tool_call_id ASC
	`, runID, stepID)
	if err != nil {
		return nil, fmt.Errorf("list tool calls for run %s step %s: %w", runID, stepID, err)
	}
	defer rows.Close()

	items := make([]runtime.ToolCall, 0)
	for rows.Next() {
		toolCall, err := scanToolCall(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, toolCall)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertComputerUseSession(ctx context.Context, session computeruse.Session) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(session)
	if err != nil {
		return fmt.Errorf("marshal computer-use session: %w", err)
	}
	trustedScopeJSON, err := marshalJSON(session.TrustedPageScope)
	if err != nil {
		return fmt.Errorf("marshal trusted page scope: %w", err)
	}
	currentPageJSON, err := marshalJSON(session.CurrentPage)
	if err != nil {
		return fmt.Errorf("marshal current page: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO computer_use_sessions (
			computer_use_session_id,
			environment_scope,
			run_id,
			workflow_id,
			workflow_step_id,
			status,
			driver_kind,
			trusted_page_scope_json,
			current_page_json,
			last_action_id,
			started_at,
			updated_at,
			closed_at,
			interrupted_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(computer_use_session_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			status = excluded.status,
			driver_kind = excluded.driver_kind,
			trusted_page_scope_json = excluded.trusted_page_scope_json,
			current_page_json = excluded.current_page_json,
			last_action_id = excluded.last_action_id,
			started_at = excluded.started_at,
			updated_at = excluded.updated_at,
			closed_at = excluded.closed_at,
			interrupted_at = excluded.interrupted_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(computer_use_sessions.tenant_id, excluded.tenant_id)
	`,
		session.ComputerUseSessionID,
		session.EnvironmentScope,
		session.RunID,
		nullString(session.WorkflowID),
		nullString(session.WorkflowStepID),
		string(session.Status),
		session.DriverKind,
		trustedScopeJSON,
		currentPageJSON,
		nullString(session.LastActionID),
		session.StartedAt.UTC().Format(time.RFC3339Nano),
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(session.ClosedAt),
		nullableTimeString(session.InterruptedAt),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert computer-use session %s: %w", session.ComputerUseSessionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListComputerUseSessions(ctx context.Context, environmentScope, runID string) ([]computeruse.Session, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM computer_use_sessions
		WHERE environment_scope = ? AND run_id = ?
		ORDER BY updated_at DESC, computer_use_session_id DESC
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID))
	if err != nil {
		return nil, fmt.Errorf("list computer-use sessions: %w", err)
	}
	defer rows.Close()
	var items []computeruse.Session
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, fmt.Errorf("scan computer-use session: %w", err)
		}
		var session computeruse.Session
		if err := json.Unmarshal([]byte(document), &session); err != nil {
			return nil, fmt.Errorf("decode computer-use session: %w", err)
		}
		items = append(items, session)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetComputerUseSession(ctx context.Context, environmentScope, runID, sessionID string) (computeruse.Session, bool, error) {
	if s == nil {
		return computeruse.Session{}, false, nil
	}
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM computer_use_sessions
		WHERE environment_scope = ? AND run_id = ? AND computer_use_session_id = ?
	`,
		strings.TrimSpace(environmentScope),
		strings.TrimSpace(runID),
		strings.TrimSpace(sessionID),
	).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return computeruse.Session{}, false, nil
	}
	if err != nil {
		return computeruse.Session{}, false, fmt.Errorf("get computer-use session: %w", err)
	}
	var session computeruse.Session
	if err := json.Unmarshal([]byte(document), &session); err != nil {
		return computeruse.Session{}, false, fmt.Errorf("decode computer-use session: %w", err)
	}
	return session, true, nil
}

func (s *SQLiteStore) UpsertComputerUseAction(ctx context.Context, action computeruse.Action) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(action)
	if err != nil {
		return fmt.Errorf("marshal computer-use action: %w", err)
	}
	targetJSON, err := marshalJSON(action.TargetMatchContext)
	if err != nil {
		return fmt.Errorf("marshal target match context: %w", err)
	}
	pageBeforeJSON, err := marshalJSON(action.PageBefore)
	if err != nil {
		return fmt.Errorf("marshal page before: %w", err)
	}
	pageAfterJSON, err := marshalJSON(action.PageAfter)
	if err != nil {
		return fmt.Errorf("marshal page after: %w", err)
	}
	inputJSON, err := marshalJSON(action.Input)
	if err != nil {
		return fmt.Errorf("marshal action input: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO computer_use_actions (
			computer_use_action_id,
			environment_scope,
			computer_use_session_id,
			run_id,
			step_id,
			tool_call_id,
			workflow_id,
			workflow_step_id,
			action_kind,
			status,
			risk_level,
			approval_id,
			target_match_context_json,
			page_before_json,
			page_after_json,
			failure_class,
			failure_reason,
			requested_at,
			updated_at,
			completed_at,
			input_json,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(computer_use_action_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			computer_use_session_id = excluded.computer_use_session_id,
			run_id = excluded.run_id,
			step_id = excluded.step_id,
			tool_call_id = excluded.tool_call_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			action_kind = excluded.action_kind,
			status = excluded.status,
			risk_level = excluded.risk_level,
			approval_id = excluded.approval_id,
			target_match_context_json = excluded.target_match_context_json,
			page_before_json = excluded.page_before_json,
			page_after_json = excluded.page_after_json,
			failure_class = excluded.failure_class,
			failure_reason = excluded.failure_reason,
			requested_at = excluded.requested_at,
			updated_at = excluded.updated_at,
			completed_at = excluded.completed_at,
			input_json = excluded.input_json,
			document_json = excluded.document_json,
			tenant_id = COALESCE(computer_use_actions.tenant_id, excluded.tenant_id)
	`,
		action.ComputerUseActionID,
		action.EnvironmentScope,
		action.ComputerUseSessionID,
		action.RunID,
		nullString(action.StepID),
		nullString(action.ToolCallID),
		nullString(action.WorkflowID),
		nullString(action.WorkflowStepID),
		string(action.ActionKind),
		string(action.Status),
		string(action.RiskLevel),
		nullString(action.ApprovalID),
		targetJSON,
		pageBeforeJSON,
		pageAfterJSON,
		nullString(action.FailureClass),
		nullString(action.FailureReason),
		action.RequestedAt.UTC().Format(time.RFC3339Nano),
		action.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(action.CompletedAt),
		inputJSON,
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert computer-use action %s: %w", action.ComputerUseActionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListComputerUseActions(ctx context.Context, environmentScope, runID, sessionID string) ([]computeruse.Action, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM computer_use_actions
		WHERE environment_scope = ? AND run_id = ? AND computer_use_session_id = ?
		ORDER BY requested_at ASC, computer_use_action_id ASC
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID), strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("list computer-use actions: %w", err)
	}
	defer rows.Close()
	var items []computeruse.Action
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, fmt.Errorf("scan computer-use action: %w", err)
		}
		var action computeruse.Action
		if err := json.Unmarshal([]byte(document), &action); err != nil {
			return nil, fmt.Errorf("decode computer-use action: %w", err)
		}
		items = append(items, action)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetComputerUseAction(ctx context.Context, environmentScope, runID, sessionID, actionID string) (computeruse.Action, bool, error) {
	if s == nil {
		return computeruse.Action{}, false, nil
	}
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM computer_use_actions
		WHERE environment_scope = ? AND run_id = ? AND computer_use_session_id = ? AND computer_use_action_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID), strings.TrimSpace(sessionID), strings.TrimSpace(actionID)).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return computeruse.Action{}, false, nil
	}
	if err != nil {
		return computeruse.Action{}, false, fmt.Errorf("get computer-use action: %w", err)
	}
	var action computeruse.Action
	if err := json.Unmarshal([]byte(document), &action); err != nil {
		return computeruse.Action{}, false, fmt.Errorf("decode computer-use action: %w", err)
	}
	return action, true, nil
}

func (s *SQLiteStore) FindPendingComputerUseActionByApproval(ctx context.Context, environmentScope, approvalID string) (computeruse.Action, bool, error) {
	if s == nil {
		return computeruse.Action{}, false, nil
	}
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM computer_use_actions
		WHERE environment_scope = ? AND approval_id = ? AND status = ?
		ORDER BY requested_at DESC, computer_use_action_id DESC
		LIMIT 1
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(approvalID), string(computeruse.ActionStatusWaitingApproval)).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return computeruse.Action{}, false, nil
	}
	if err != nil {
		return computeruse.Action{}, false, fmt.Errorf("find pending computer-use action: %w", err)
	}
	var action computeruse.Action
	if err := json.Unmarshal([]byte(document), &action); err != nil {
		return computeruse.Action{}, false, fmt.Errorf("decode pending computer-use action: %w", err)
	}
	return action, true, nil
}

func (s *SQLiteStore) UpsertComputerUseArtifact(ctx context.Context, artifact computeruse.Artifact) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(artifact)
	if err != nil {
		return fmt.Errorf("marshal computer-use artifact: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO computer_use_artifacts (
			artifact_id,
			environment_scope,
			computer_use_session_id,
			computer_use_action_id,
			run_id,
			kind,
			status,
			mime_type,
			file_name,
			byte_size,
			storage_key,
			sha256,
			capture_failure_reason,
			created_at,
			available_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artifact_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			computer_use_session_id = excluded.computer_use_session_id,
			computer_use_action_id = excluded.computer_use_action_id,
			run_id = excluded.run_id,
			kind = excluded.kind,
			status = excluded.status,
			mime_type = excluded.mime_type,
			file_name = excluded.file_name,
			byte_size = excluded.byte_size,
			storage_key = excluded.storage_key,
			sha256 = excluded.sha256,
			capture_failure_reason = excluded.capture_failure_reason,
			created_at = excluded.created_at,
			available_at = excluded.available_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(computer_use_artifacts.tenant_id, excluded.tenant_id)
	`,
		artifact.ArtifactID,
		artifact.EnvironmentScope,
		artifact.ComputerUseSessionID,
		artifact.ComputerUseActionID,
		artifact.RunID,
		string(artifact.Kind),
		string(artifact.Status),
		nullString(artifact.MIMEType),
		nullString(artifact.FileName),
		artifact.ByteSize,
		nullString(artifact.StorageKey),
		nullString(artifact.SHA256),
		nullString(artifact.CaptureFailureReason),
		artifact.CreatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(artifact.AvailableAt),
		documentJSON,
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert computer-use artifact %s: %w", artifact.ArtifactID, err)
	}
	return nil
}

func (s *SQLiteStore) ListComputerUseArtifactsForAction(ctx context.Context, environmentScope, runID, actionID string) ([]computeruse.Artifact, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM computer_use_artifacts
		WHERE environment_scope = ? AND run_id = ? AND computer_use_action_id = ?
		ORDER BY created_at ASC, artifact_id ASC
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(runID), strings.TrimSpace(actionID))
	if err != nil {
		return nil, fmt.Errorf("list computer-use artifacts: %w", err)
	}
	defer rows.Close()
	var items []computeruse.Artifact
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, fmt.Errorf("scan computer-use artifact: %w", err)
		}
		var artifact computeruse.Artifact
		if err := json.Unmarshal([]byte(document), &artifact); err != nil {
			return nil, fmt.Errorf("decode computer-use artifact: %w", err)
		}
		items = append(items, artifact)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetComputerUseArtifact(ctx context.Context, environmentScope, artifactID string) (computeruse.Artifact, bool, error) {
	if s == nil {
		return computeruse.Artifact{}, false, nil
	}
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM computer_use_artifacts
		WHERE environment_scope = ? AND artifact_id = ?
	`, strings.TrimSpace(environmentScope), strings.TrimSpace(artifactID)).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return computeruse.Artifact{}, false, nil
	}
	if err != nil {
		return computeruse.Artifact{}, false, fmt.Errorf("get computer-use artifact: %w", err)
	}
	var artifact computeruse.Artifact
	if err := json.Unmarshal([]byte(document), &artifact); err != nil {
		return computeruse.Artifact{}, false, fmt.Errorf("decode computer-use artifact: %w", err)
	}
	return artifact, true, nil
}

func (s *SQLiteStore) MarkInFlightComputerUseInterrupted(ctx context.Context, environmentScope string, interruptedAt time.Time) ([]computeruse.Session, []computeruse.Action, error) {
	if s == nil {
		return nil, nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM computer_use_sessions
		WHERE environment_scope = ? AND status IN (?, ?, ?)
	`, strings.TrimSpace(environmentScope), string(computeruse.SessionStatusStarting), string(computeruse.SessionStatusActive), string(computeruse.SessionStatusBlocked))
	if err != nil {
		return nil, nil, fmt.Errorf("list in-flight computer-use sessions: %w", err)
	}
	defer rows.Close()
	updatedSessions := make([]computeruse.Session, 0)
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, nil, fmt.Errorf("scan in-flight computer-use session: %w", err)
		}
		var session computeruse.Session
		if err := json.Unmarshal([]byte(document), &session); err != nil {
			return nil, nil, fmt.Errorf("decode in-flight computer-use session: %w", err)
		}
		updatedSessions = append(updatedSessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for idx := range updatedSessions {
		updatedSessions[idx].Status = computeruse.SessionStatusInterrupted
		updatedSessions[idx].InterruptedAt = &interruptedAt
		updatedSessions[idx].UpdatedAt = interruptedAt
		if err := s.UpsertComputerUseSession(ctx, updatedSessions[idx]); err != nil {
			return nil, nil, err
		}
	}

	actionRows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM computer_use_actions
		WHERE environment_scope = ? AND status IN (?, ?, ?)
	`, strings.TrimSpace(environmentScope), string(computeruse.ActionStatusRequested), string(computeruse.ActionStatusWaitingApproval), string(computeruse.ActionStatusRunning))
	if err != nil {
		return nil, nil, fmt.Errorf("list in-flight computer-use actions: %w", err)
	}
	defer actionRows.Close()
	updatedActions := make([]computeruse.Action, 0)
	for actionRows.Next() {
		var document string
		if err := actionRows.Scan(&document); err != nil {
			return nil, nil, fmt.Errorf("scan in-flight computer-use action: %w", err)
		}
		var action computeruse.Action
		if err := json.Unmarshal([]byte(document), &action); err != nil {
			return nil, nil, fmt.Errorf("decode in-flight computer-use action: %w", err)
		}
		updatedActions = append(updatedActions, action)
	}
	if err := actionRows.Err(); err != nil {
		return nil, nil, err
	}
	for idx := range updatedActions {
		updatedActions[idx].Status = computeruse.ActionStatusInterrupted
		updatedActions[idx].FailureClass = string(computeruse.FailureClassInterrupted)
		updatedActions[idx].FailureReason = "daemon restarted before computer-use action completed"
		updatedActions[idx].UpdatedAt = interruptedAt
		updatedActions[idx].CompletedAt = &interruptedAt
		if err := s.UpsertComputerUseAction(ctx, updatedActions[idx]); err != nil {
			return nil, nil, err
		}
	}
	return updatedSessions, updatedActions, nil
}

func (s *SQLiteStore) HasActiveMCPToolCalls(ctx context.Context, serverID string) (bool, error) {
	if s == nil {
		return false, nil
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM tool_calls
		WHERE mcp_server_id = ?
		  AND status IN (?, ?)
	`,
		strings.TrimSpace(serverID),
		string(runtime.ToolCallStatusRequested),
		string(runtime.ToolCallStatusRunning),
	).Scan(&count); err != nil {
		return false, fmt.Errorf("count active mcp tool calls for %s: %w", serverID, err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) ListSteps(ctx context.Context, runID string) ([]runtime.Step, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT step_id, run_id, workflow_id, workflow_step_id, attempt, title, kind, status, input_json, output_json, created_at, updated_at
		FROM steps
		WHERE run_id = ?
		ORDER BY created_at ASC, step_id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps for run %s: %w", runID, err)
	}
	defer rows.Close()

	steps := make([]runtime.Step, 0)
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	return steps, rows.Err()
}

func (s *SQLiteStore) UpsertConsumerPolicyRecord(ctx context.Context, record ConsumerPolicyRecordRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO consumer_policy_records (
			policy_record_id,
			consumer_kind,
			consumer_id,
			operation_kind,
			declaration_id,
			status,
			decision,
			approval_status,
			secret_resolution,
			requested_by,
			sandbox_execution_id,
			tool_call_id,
			provider_operation_id,
			started_at,
			completed_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(policy_record_id) DO UPDATE SET
			consumer_kind = excluded.consumer_kind,
			consumer_id = excluded.consumer_id,
			operation_kind = excluded.operation_kind,
			declaration_id = excluded.declaration_id,
			status = excluded.status,
			decision = excluded.decision,
			approval_status = excluded.approval_status,
			secret_resolution = excluded.secret_resolution,
			requested_by = excluded.requested_by,
			sandbox_execution_id = excluded.sandbox_execution_id,
			tool_call_id = excluded.tool_call_id,
			provider_operation_id = excluded.provider_operation_id,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(consumer_policy_records.tenant_id, excluded.tenant_id)
	`,
		record.PolicyRecordID,
		record.ConsumerKind,
		record.ConsumerID,
		record.OperationKind,
		nullString(record.DeclarationID),
		record.Status,
		record.Decision,
		record.ApprovalStatus,
		record.SecretResolution,
		nullString(record.RequestedBy),
		nullString(record.SandboxExecutionID),
		nullString(record.ToolCallID),
		nullString(record.ProviderOperationID),
		record.StartedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(record.CompletedAt),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert consumer policy record %s: %w", record.PolicyRecordID, err)
	}
	return nil
}

func (s *SQLiteStore) ListConsumerPolicyRecords(ctx context.Context) ([]ConsumerPolicyRecordRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT policy_record_id, consumer_kind, consumer_id, operation_kind, declaration_id, status, decision, approval_status, secret_resolution, requested_by, sandbox_execution_id, tool_call_id, provider_operation_id, started_at, completed_at, document_json
		FROM consumer_policy_records
		ORDER BY started_at ASC, policy_record_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list consumer policy records: %w", err)
	}
	defer rows.Close()

	items := make([]ConsumerPolicyRecordRecord, 0)
	for rows.Next() {
		item, err := scanConsumerPolicyRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertSecretScopeBinding(ctx context.Context, record SecretScopeBindingRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO secret_scope_bindings (
			binding_id,
			consumer_kind,
			consumer_id,
			environment_scope,
			secret_ref,
			default_source,
			delivery_kind,
			active,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(binding_id) DO UPDATE SET
			consumer_kind = excluded.consumer_kind,
			consumer_id = excluded.consumer_id,
			environment_scope = excluded.environment_scope,
			secret_ref = excluded.secret_ref,
			default_source = excluded.default_source,
			delivery_kind = excluded.delivery_kind,
			active = excluded.active,
			document_json = excluded.document_json,
			tenant_id = COALESCE(secret_scope_bindings.tenant_id, excluded.tenant_id)
	`,
		record.BindingID,
		record.ConsumerKind,
		record.ConsumerID,
		record.EnvironmentScope,
		record.SecretRef,
		record.DefaultSource,
		record.DeliveryKind,
		boolToInt(record.Active),
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert secret scope binding %s: %w", record.BindingID, err)
	}
	return nil
}

func (s *SQLiteStore) ListSecretScopeBindings(ctx context.Context, consumerKind, consumerID string) ([]SecretScopeBindingRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT binding_id, consumer_kind, consumer_id, environment_scope, secret_ref, default_source, delivery_kind, active, document_json
		FROM secret_scope_bindings
		WHERE consumer_kind = ? AND consumer_id = ?
		ORDER BY secret_ref ASC, binding_id ASC
	`, consumerKind, consumerID)
	if err != nil {
		return nil, fmt.Errorf("list secret scope bindings for %s/%s: %w", consumerKind, consumerID, err)
	}
	defer rows.Close()

	items := make([]SecretScopeBindingRecord, 0)
	for rows.Next() {
		item, err := scanSecretScopeBinding(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMCPServer(ctx context.Context, record MCPServerRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_servers (
			server_id,
			enabled,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			enabled = excluded.enabled,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mcp_servers.tenant_id, excluded.tenant_id)
	`,
		record.ServerID,
		boolToInt(record.Enabled),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveActiveTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mcp server %s: %w", record.ServerID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMCPServers(ctx context.Context) ([]MCPServerRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, enabled, updated_at, document_json
		FROM mcp_servers
		ORDER BY updated_at ASC, server_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	defer rows.Close()

	items := make([]MCPServerRecord, 0)
	for rows.Next() {
		record, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) DeleteMCPServer(ctx context.Context, serverID string) error {
	if s == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE server_id = ?`, strings.TrimSpace(serverID)); err != nil {
		return fmt.Errorf("delete mcp server %s: %w", serverID, err)
	}
	return nil
}

func (s *SQLiteStore) UpsertMCPServerState(ctx context.Context, record MCPServerStateRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_server_states (
			server_id,
			status,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			status = excluded.status,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mcp_server_states.tenant_id, excluded.tenant_id)
	`,
		record.ServerID,
		record.Status,
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveActiveTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mcp server state %s: %w", record.ServerID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMCPServerStates(ctx context.Context) ([]MCPServerStateRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, status, updated_at, document_json
		FROM mcp_server_states
		ORDER BY updated_at ASC, server_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list mcp server states: %w", err)
	}
	defer rows.Close()

	items := make([]MCPServerStateRecord, 0)
	for rows.Next() {
		record, err := scanMCPServerState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMCPTool(ctx context.Context, record MCPToolRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_tools (
			server_id,
			tool_name,
			discovery_status,
			updated_at,
			last_discovered_at,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, tool_name) DO UPDATE SET
			discovery_status = excluded.discovery_status,
			updated_at = excluded.updated_at,
			last_discovered_at = excluded.last_discovered_at,
			document_json = excluded.document_json
	`,
		record.ServerID,
		record.ToolName,
		record.DiscoveryStatus,
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(record.LastDiscoveredAt),
		string(record.Document),
	)
	if err != nil {
		return fmt.Errorf("upsert mcp tool %s/%s: %w", record.ServerID, record.ToolName, err)
	}
	return nil
}

func (s *SQLiteStore) ReplaceMCPTools(ctx context.Context, serverID string, records []MCPToolRecord) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace mcp tools: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	tenantID := s.ResolveActiveTenantBinding(ctx)
	if _, err = tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id = ? AND (? IS NULL OR ? = '' OR tenant_id = ? OR tenant_id IS NULL)`, serverID, tenantID, tenantID, tenantID); err != nil {
		return fmt.Errorf("clear mcp tools for %s: %w", serverID, err)
	}
	for _, record := range records {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO mcp_tools (
				server_id,
				tool_name,
				discovery_status,
				updated_at,
				last_discovered_at,
				document_json,
				tenant_id
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			record.ServerID,
			record.ToolName,
			record.DiscoveryStatus,
			record.UpdatedAt.UTC().Format(time.RFC3339Nano),
			nullableTimeString(record.LastDiscoveredAt),
			string(record.Document),
			tenantID,
		); err != nil {
			return fmt.Errorf("insert mcp tool %s/%s: %w", record.ServerID, record.ToolName, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit replace mcp tools: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListMCPTools(ctx context.Context, serverID string) ([]MCPToolRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, tool_name, discovery_status, updated_at, last_discovered_at, document_json
		FROM mcp_tools
		WHERE (? = '' OR server_id = ?)
		ORDER BY server_id ASC, tool_name ASC
	`, serverID, serverID)
	if err != nil {
		return nil, fmt.Errorf("list mcp tools for %s: %w", serverID, err)
	}
	defer rows.Close()

	items := make([]MCPToolRecord, 0)
	for rows.Next() {
		record, err := scanMCPTool(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertMCPToolExposureRule(ctx context.Context, record MCPToolExposureRuleRecord) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_tool_exposure_rules (
			server_id,
			tool_name,
			runtime_surface,
			exposure_mode,
			active,
			updated_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, tool_name, runtime_surface) DO UPDATE SET
			exposure_mode = excluded.exposure_mode,
			active = excluded.active,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(mcp_tool_exposure_rules.tenant_id, excluded.tenant_id)
	`,
		record.ServerID,
		record.ToolName,
		record.RuntimeSurface,
		record.ExposureMode,
		boolToInt(record.Active),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(record.Document),
		s.ResolveActiveTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert mcp tool exposure rule %s/%s/%s: %w", record.ServerID, record.ToolName, record.RuntimeSurface, err)
	}
	return nil
}

func (s *SQLiteStore) ListMCPToolExposureRules(ctx context.Context, serverID string) ([]MCPToolExposureRuleRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_id, tool_name, runtime_surface, exposure_mode, active, updated_at, document_json
		FROM mcp_tool_exposure_rules
		WHERE (? = '' OR server_id = ?)
		ORDER BY server_id ASC, tool_name ASC, runtime_surface ASC
	`, serverID, serverID)
	if err != nil {
		return nil, fmt.Errorf("list mcp tool exposure rules for %s: %w", serverID, err)
	}
	defer rows.Close()

	items := make([]MCPToolExposureRuleRecord, 0)
	for rows.Next() {
		record, err := scanMCPToolExposureRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertReplayCandidate(ctx context.Context, item evaluation.ReplayCandidate) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal replay candidate: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_replay_candidates (
			candidate_id, environment_scope, candidate_kind, source_kind, source_id, readiness_status,
			latest_attempt_id, latest_comparison_id, created_at, updated_at, document_json, tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(candidate_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			candidate_kind = excluded.candidate_kind,
			source_kind = excluded.source_kind,
			source_id = excluded.source_id,
			readiness_status = excluded.readiness_status,
			latest_attempt_id = excluded.latest_attempt_id,
			latest_comparison_id = excluded.latest_comparison_id,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(evaluation_replay_candidates.tenant_id, excluded.tenant_id)
	`, item.CandidateID, item.EnvironmentScope, string(item.CandidateKind), string(item.SourceKind), item.SourceID, string(item.ReadinessStatus),
		nullString(item.LatestAttemptID), nullString(item.LatestComparisonID), item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano), string(document), s.ResolveDefaultTenantBinding(ctx))
	if err != nil {
		return fmt.Errorf("upsert replay candidate %s: %w", item.CandidateID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReplayCandidates(ctx context.Context, filter evaluation.CandidateFilter) ([]evaluation.ReplayCandidate, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM evaluation_replay_candidates WHERE 1 = 1`
	args := make([]any, 0, 5)
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.CandidateKind != "" {
		query += ` AND candidate_kind = ?`
		args = append(args, string(filter.CandidateKind))
	}
	if filter.SourceKind != "" {
		query += ` AND source_kind = ?`
		args = append(args, string(filter.SourceKind))
	}
	if filter.ReadinessStatus != "" {
		query += ` AND readiness_status = ?`
		args = append(args, string(filter.ReadinessStatus))
	}
	query += ` ORDER BY updated_at DESC, candidate_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list replay candidates: %w", err)
	}
	defer rows.Close()
	items := []evaluation.ReplayCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item evaluation.ReplayCandidate
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode replay candidate: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetReplayCandidate(ctx context.Context, environmentScope, candidateID string) (evaluation.ReplayCandidate, bool, error) {
	if s == nil {
		return evaluation.ReplayCandidate{}, false, nil
	}
	query := `SELECT document_json FROM evaluation_replay_candidates WHERE candidate_id = ?`
	args := []any{candidateID}
	if environmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, environmentScope)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return evaluation.ReplayCandidate{}, false, nil
		}
		return evaluation.ReplayCandidate{}, false, fmt.Errorf("get replay candidate %s: %w", candidateID, err)
	}
	var item evaluation.ReplayCandidate
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return evaluation.ReplayCandidate{}, false, fmt.Errorf("decode replay candidate %s: %w", candidateID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) UpsertReplayAttempt(ctx context.Context, item evaluation.ReplayAttempt) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal replay attempt: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_replay_attempts (
			attempt_id, candidate_id, environment_scope, mode, status, change_window_label,
			baseline_attempt_id, started_at, completed_at, created_at, updated_at, document_json, tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(attempt_id) DO UPDATE SET
			candidate_id = excluded.candidate_id,
			environment_scope = excluded.environment_scope,
			mode = excluded.mode,
			status = excluded.status,
			change_window_label = excluded.change_window_label,
			baseline_attempt_id = excluded.baseline_attempt_id,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(evaluation_replay_attempts.tenant_id, excluded.tenant_id)
	`, item.AttemptID, item.CandidateID, item.EnvironmentScope, string(item.Mode), string(item.Status), nullString(item.ChangeWindowLabel),
		nullString(item.BaselineAttemptID), nullableValueTimeString(item.StartedAt), nullableValueTimeString(item.CompletedAt),
		item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano), string(document), s.ResolveDefaultTenantBinding(ctx))
	if err != nil {
		return fmt.Errorf("upsert replay attempt %s: %w", item.AttemptID, err)
	}
	return nil
}

func (s *SQLiteStore) ListReplayAttempts(ctx context.Context, filter evaluation.AttemptFilter) ([]evaluation.ReplayAttempt, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM evaluation_replay_attempts WHERE 1 = 1`
	args := make([]any, 0, 4)
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.CandidateID != "" {
		query += ` AND candidate_id = ?`
		args = append(args, filter.CandidateID)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	query += ` ORDER BY created_at DESC, attempt_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list replay attempts: %w", err)
	}
	defer rows.Close()
	items := []evaluation.ReplayAttempt{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item evaluation.ReplayAttempt
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode replay attempt: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetReplayAttempt(ctx context.Context, environmentScope, attemptID string) (evaluation.ReplayAttempt, bool, error) {
	if s == nil {
		return evaluation.ReplayAttempt{}, false, nil
	}
	query := `SELECT document_json FROM evaluation_replay_attempts WHERE attempt_id = ?`
	args := []any{attemptID}
	if environmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, environmentScope)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return evaluation.ReplayAttempt{}, false, nil
		}
		return evaluation.ReplayAttempt{}, false, fmt.Errorf("get replay attempt %s: %w", attemptID, err)
	}
	var item evaluation.ReplayAttempt
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return evaluation.ReplayAttempt{}, false, fmt.Errorf("decode replay attempt %s: %w", attemptID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) UpsertComparisonResult(ctx context.Context, item evaluation.ComparisonResult) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal comparison result: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_comparisons (
			comparison_id, candidate_id, attempt_id, environment_scope, terminal_status,
			change_window_label, generated_at, document_json, tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(comparison_id) DO UPDATE SET
			candidate_id = excluded.candidate_id,
			attempt_id = excluded.attempt_id,
			environment_scope = excluded.environment_scope,
			terminal_status = excluded.terminal_status,
			change_window_label = excluded.change_window_label,
			generated_at = excluded.generated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(evaluation_comparisons.tenant_id, excluded.tenant_id)
	`, item.ComparisonID, item.CandidateID, item.AttemptID, item.EnvironmentScope, string(item.TerminalStatus),
		nullString(item.ChangeWindowLabel), item.GeneratedAt.Format(time.RFC3339Nano), string(document), s.ResolveDefaultTenantBinding(ctx))
	if err != nil {
		return fmt.Errorf("upsert comparison %s: %w", item.ComparisonID, err)
	}
	return nil
}

func (s *SQLiteStore) ListComparisonResults(ctx context.Context, filter evaluation.ComparisonFilter) ([]evaluation.ComparisonResult, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM evaluation_comparisons WHERE 1 = 1`
	args := make([]any, 0, 5)
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.CandidateID != "" {
		query += ` AND candidate_id = ?`
		args = append(args, filter.CandidateID)
	}
	if filter.AttemptID != "" {
		query += ` AND attempt_id = ?`
		args = append(args, filter.AttemptID)
	}
	if filter.TerminalStatus != "" {
		query += ` AND terminal_status = ?`
		args = append(args, string(filter.TerminalStatus))
	}
	query += ` ORDER BY generated_at DESC, comparison_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list comparisons: %w", err)
	}
	defer rows.Close()
	items := []evaluation.ComparisonResult{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item evaluation.ComparisonResult
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode comparison: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetComparisonResult(ctx context.Context, environmentScope, comparisonID string) (evaluation.ComparisonResult, bool, error) {
	if s == nil {
		return evaluation.ComparisonResult{}, false, nil
	}
	query := `SELECT document_json FROM evaluation_comparisons WHERE comparison_id = ?`
	args := []any{comparisonID}
	if environmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, environmentScope)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return evaluation.ComparisonResult{}, false, nil
		}
		return evaluation.ComparisonResult{}, false, fmt.Errorf("get comparison %s: %w", comparisonID, err)
	}
	var item evaluation.ComparisonResult
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return evaluation.ComparisonResult{}, false, fmt.Errorf("decode comparison %s: %w", comparisonID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) UpsertRegressionFixture(ctx context.Context, item evaluation.RegressionFixture) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal regression fixture: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_regression_fixtures (
			fixture_id, environment_scope, domain_class, candidate_id, manifest_path, created_at, updated_at, document_json, tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fixture_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			domain_class = excluded.domain_class,
			candidate_id = excluded.candidate_id,
			manifest_path = excluded.manifest_path,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(evaluation_regression_fixtures.tenant_id, excluded.tenant_id)
	`, item.FixtureID, item.EnvironmentScope, string(item.DomainClass), nullString(item.CandidateID), item.ManifestPath,
		item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano), string(document), s.ResolveDefaultTenantBinding(ctx))
	if err != nil {
		return fmt.Errorf("upsert regression fixture %s: %w", item.FixtureID, err)
	}
	return nil
}

func (s *SQLiteStore) ListRegressionFixtures(ctx context.Context, filter evaluation.FixtureFilter) ([]evaluation.RegressionFixture, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM evaluation_regression_fixtures WHERE 1 = 1`
	args := make([]any, 0, 3)
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.DomainClass != "" {
		query += ` AND domain_class = ?`
		args = append(args, string(filter.DomainClass))
	}
	query += ` ORDER BY domain_class ASC, fixture_id ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list regression fixtures: %w", err)
	}
	defer rows.Close()
	items := []evaluation.RegressionFixture{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item evaluation.RegressionFixture
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode regression fixture: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event events.Event) (events.Event, error) {
	if s == nil {
		return event, nil
	}

	payloadJSON, err := marshalJSON(event.Payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal event payload: %w", err)
	}

	// Auto-bind tenant_id for non-global categories so legacy callers
	// (sandbox manager, recovery paths) satisfy the T077b CHECK
	// constraint. Resolution order: caller-provided event.TenantID,
	// then tenant context, then the cached default personal tenant.
	// Global categories stay NULL by inventory contract.
	tenantID := event.TenantID
	if tenantID == "" && !events.IsGlobalCategory(event.Category) {
		if cached, ok := s.defaultTenantCache.getOK(); ok {
			tenantID = cached
		}
	}
	var tenantArg any
	if tenantID != "" {
		tenantArg = tenantID
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO events (
			event_id,
			environment_scope,
			category,
			name,
			occurred_at,
			session_id,
			run_id,
			workflow_id,
			workflow_step_id,
			schedule_id,
			schedule_attempt_id,
			step_id,
			connector_id,
			capability_id,
			resource_kind,
			resource_id,
			payload_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`,
		event.EventID,
		coalesceString(event.EnvironmentScope, events.EnvironmentScopeFromContext(ctx)),
		event.Category,
		event.Name,
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
		nullString(event.Scope.SessionID),
		nullString(event.Scope.RunID),
		nullString(event.Scope.WorkflowID),
		nullString(event.Scope.WorkflowStepID),
		nullString(event.Scope.ScheduleID),
		nullString(event.Scope.ScheduleAttemptID),
		nullString(event.Scope.StepID),
		nullString(event.Scope.ConnectorID),
		nullString(event.Scope.CapabilityID),
		event.Resource.Kind,
		event.Resource.ID,
		payloadJSON,
		tenantArg,
	)
	if err != nil {
		return events.Event{}, fmt.Errorf("append event %s: %w", event.EventID, err)
	}

	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM events WHERE event_id = ?`, event.EventID).Scan(&event.Sequence); err != nil {
		return events.Event{}, fmt.Errorf("load event sequence %s: %w", event.EventID, err)
	}
	if tenantID != "" {
		event.TenantID = tenantID
	}

	return event, nil
}

func (s *SQLiteStore) ListEvents(ctx context.Context, filter events.Filter) ([]events.Event, error) {
	if s == nil {
		return nil, nil
	}

	query := `
		SELECT rowid, event_id, environment_scope, category, name, occurred_at, session_id, run_id, workflow_id, workflow_step_id, schedule_id, schedule_attempt_id, step_id, connector_id, capability_id, resource_kind, resource_id, payload_json
		FROM events
		WHERE 1 = 1
	`
	args := make([]any, 0, 7)

	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, filter.Category)
	}
	if filter.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, filter.RunID)
	}
	if filter.SessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, filter.SessionID)
	}
	if filter.ScheduleID != "" {
		query += ` AND schedule_id = ?`
		args = append(args, filter.ScheduleID)
	}
	if filter.ScheduleAttemptID != "" {
		query += ` AND schedule_attempt_id = ?`
		args = append(args, filter.ScheduleAttemptID)
	}
	if filter.ResourceKind != "" {
		query += ` AND resource_kind = ?`
		args = append(args, filter.ResourceKind)
	}
	if filter.Cursor > 0 {
		query += ` AND rowid > ?`
		args = append(args, filter.Cursor)
	}

	query += ` ORDER BY rowid ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	items := make([]events.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, event)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertSandboxExecution(ctx context.Context, record SandboxExecutionRecord) error {
	if s == nil {
		return nil
	}
	var (
		approvalID  sql.NullString
		startedAt   sql.NullString
		completedAt sql.NullString
	)
	if strings.TrimSpace(record.ApprovalID) != "" {
		approvalID = sql.NullString{String: strings.TrimSpace(record.ApprovalID), Valid: true}
	}
	if record.StartedAt != nil {
		startedAt = sql.NullString{String: record.StartedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	if record.CompletedAt != nil {
		completedAt = sql.NullString{String: record.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sandbox_executions (
			execution_id,
			profile_id,
			backend_kind,
			status,
			approval_id,
			requested_at,
			updated_at,
			started_at,
			completed_at,
			document_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(execution_id) DO UPDATE SET
			profile_id = excluded.profile_id,
			backend_kind = excluded.backend_kind,
			status = excluded.status,
			approval_id = excluded.approval_id,
			requested_at = excluded.requested_at,
			updated_at = excluded.updated_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			document_json = excluded.document_json,
			tenant_id = COALESCE(sandbox_executions.tenant_id, excluded.tenant_id)
	`,
		record.ExecutionID,
		record.ProfileID,
		record.BackendKind,
		record.Status,
		approvalID,
		record.RequestedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano),
		startedAt,
		completedAt,
		string(record.Document),
		s.ResolveDefaultTenantBinding(ctx),
	)
	if err != nil {
		return fmt.Errorf("upsert sandbox execution %s: %w", record.ExecutionID, err)
	}
	return nil
}

func (s *SQLiteStore) ListSandboxExecutions(ctx context.Context) ([]SandboxExecutionRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT execution_id, profile_id, backend_kind, status, approval_id, requested_at, updated_at, started_at, completed_at, document_json
		FROM sandbox_executions
		ORDER BY requested_at ASC, execution_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list sandbox executions: %w", err)
	}
	defer rows.Close()

	items := make([]SandboxExecutionRecord, 0)
	for rows.Next() {
		record, err := scanSandboxExecution(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, checkpoint runtime.RunCheckpoint) error {
	if s == nil {
		return nil
	}

	snapshotJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	// Roadmap 35 Pass B: bind tenant_id at insert time. The checkpoint
	// row inherits tenant_id from the runs table (checkpoints derive
	// from runs per the inventory's parent-FK rule). Resolve via a
	// lookup on runs; fall back to the bootstrapped personal tenant if
	// the parent has no tenant id yet (pre-backfill window).
	var tenantID sql.NullString
	var parentTenant sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT tenant_id FROM runs WHERE run_id = ?`, checkpoint.Run.RunID).Scan(&parentTenant)
	if parentTenant.Valid && parentTenant.String != "" {
		tenantID = parentTenant
	} else if id, err := s.ResolveDefaultPersonalTenantID(ctx); err == nil && id != "" {
		tenantID = sql.NullString{String: id, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO checkpoints (
			checkpoint_id,
			run_id,
			captured_at,
			snapshot_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?)
	`,
		newCheckpointID(),
		checkpoint.Run.RunID,
		checkpoint.CapturedAt.UTC().Format(time.RFC3339Nano),
		string(snapshotJSON),
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("save checkpoint for run %s: %w", checkpoint.Run.RunID, err)
	}

	return nil
}

func (s *SQLiteStore) ListLatestCheckpoints(ctx context.Context) ([]runtime.RunCheckpoint, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT checkpoint_id, run_id, captured_at, snapshot_json
		FROM checkpoints
		WHERE (run_id, captured_at) IN (
			SELECT run_id, MAX(captured_at)
			FROM checkpoints
			GROUP BY run_id
		)
		ORDER BY captured_at ASC, run_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list latest checkpoints: %w", err)
	}
	defer rows.Close()

	items := make([]runtime.RunCheckpoint, 0)
	for rows.Next() {
		checkpoint, err := scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, checkpoint)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) configure(ctx context.Context) error {
	pragmas := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
	}
	for _, stmt := range pragmas {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}
	return nil
}

// MigrateToVersion applies schema migrations only up through `targetVersion`,
// stopping before any later migration is applied. Used by the Roadmap 35
// US2 fixture builder to produce a pre-tenant (v21) database that
// subsequent test code can run the rest of the migrations against.
//
// This is a test-only helper. Production callers MUST use NewSQLiteStore
// which always migrates to the head version.
func (s *SQLiteStore) MigrateToVersion(ctx context.Context, targetVersion int) error {
	if s == nil {
		return errors.New("MigrateToVersion: nil store")
	}
	if targetVersion < 1 {
		return fmt.Errorf("MigrateToVersion: invalid target %d", targetVersion)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	currentVersion, err := currentSchemaVersion(ctx, tx)
	if err != nil {
		return err
	}
	for _, migration := range schemaMigrations {
		if migration.Version <= currentVersion {
			continue
		}
		if migration.Version > targetVersion {
			break
		}
		for _, stmt := range migration.Statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
			}
		}
		if err := recordSchemaMigration(ctx, tx, migration.Version, migration.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NewSQLiteStoreAtVersion is a test-only constructor that opens a SQLite
// store and migrates only up through `targetVersion`. Used to build the
// Roadmap 35 US2 pre-tenant (v21) fixture without applying the v22+
// tenant_id columns.
//
// Callers must invoke ApplyHeadMigrations later to bring the database
// to the production head; doing so is what the migration regression
// suite exercises.
func NewSQLiteStoreAtVersion(dataDir string, targetVersion int) (*SQLiteStore, error) {
	resolvedDir, err := resolveDataDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(resolvedDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &SQLiteStore{DataDir: resolvedDir, DBPath: dbPath, db: db}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.MigrateToVersion(context.Background(), targetVersion); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

// ApplyHeadMigrations brings a store opened via NewSQLiteStoreAtVersion
// up to CurrentSchemaVersion and registers every batch of progress
// rows. Mirrors the post-migration steps in NewSQLiteStore.
func (s *SQLiteStore) ApplyHeadMigrations(ctx context.Context) error {
	if err := s.migrate(ctx); err != nil {
		return err
	}
	if err := s.RegisterEventsMigrationSteps(ctx); err != nil {
		return err
	}
	if err := s.RegisterRuntimeMigrationSteps(ctx); err != nil {
		return err
	}
	if err := s.RegisterBatch2MigrationSteps(ctx); err != nil {
		return err
	}
	if err := s.RegisterBatch3MigrationSteps(ctx); err != nil {
		return err
	}
	if err := s.RegisterEnforcementMigrationSteps(ctx); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	currentVersion, err := currentSchemaVersion(ctx, tx)
	if err != nil {
		return fmt.Errorf("load current schema version: %w", err)
	}

	if currentVersion == 0 {
		legacy, err := hasLegacySchema(ctx, tx)
		if err != nil {
			return fmt.Errorf("detect legacy schema: %w", err)
		}
		if legacy {
			if err := recordSchemaMigration(ctx, tx, schemaMigrations[0].Version, schemaMigrations[0].Name+"_legacy_bootstrap"); err != nil {
				return fmt.Errorf("record legacy schema bootstrap: %w", err)
			}
			currentVersion = schemaMigrations[0].Version
		}
	}

	if currentVersion > CurrentSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", currentVersion, CurrentSchemaVersion)
	}

	for _, migration := range schemaMigrations {
		if migration.Version <= currentVersion {
			continue
		}
		for _, stmt := range migration.Statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply schema migration %d (%s): %w", migration.Version, migration.Name, err)
			}
		}
		if err := recordSchemaMigration(ctx, tx, migration.Version, migration.Name); err != nil {
			return fmt.Errorf("record schema migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		currentVersion = migration.Version
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration transaction: %w", err)
	}
	return nil
}

func currentSchemaVersion(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (int, error) {
	var version sql.NullInt64
	if err := queryer.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

func hasLegacySchema(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (bool, error) {
	var count int
	if err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM sqlite_master
		WHERE type = 'table'
		  AND name IN ('sessions', 'runs', 'steps', 'events', 'checkpoints', 'tool_calls', 'llm_dispatches', 'provider_checks', 'provider_auth_states', 'provider_models', 'provider_preferences', 'connectors', 'capabilities', 'auth_pairings', 'auth_tokens', 'approvals', 'decisions', 'sandbox_executions')
	`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordSchemaMigration(ctx context.Context, execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}, version int, name string) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES (?, ?, ?)
		ON CONFLICT(version) DO UPDATE SET
			name = excluded.name,
			applied_at = excluded.applied_at
	`, version, name, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func resolveDataDir(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("data dir is required")
	}
	if dataDir == "~" || strings.HasPrefix(dataDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		if dataDir == "~" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, strings.TrimPrefix(dataDir, "~/")), nil
	}
	return dataDir, nil
}

func scanRun(scanner interface {
	Scan(dest ...any) error
}) (runtime.Run, error) {
	var (
		run                  runtime.Run
		status               string
		sessionID            sql.NullString
		scheduleID           sql.NullString
		scheduleAttemptID    sql.NullString
		reminderID           sql.NullString
		reminderOccurrenceID sql.NullString
		createdAt            string
		updatedAt            string
	)

	if err := scanner.Scan(
		&run.RunID,
		&sessionID,
		&scheduleID,
		&scheduleAttemptID,
		&reminderID,
		&reminderOccurrenceID,
		&run.Entrypoint,
		&status,
		&run.Goal,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.Run{}, fmt.Errorf("scan run: %w", err)
	}

	run.SessionID = sessionID.String
	run.ScheduleID = scheduleID.String
	run.ScheduleAttemptID = scheduleAttemptID.String
	run.ReminderID = reminderID.String
	run.ReminderOccurrenceID = reminderOccurrenceID.String
	run.Status = runtime.RunStatus(status)

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.Run{}, fmt.Errorf("parse run created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.Run{}, fmt.Errorf("parse run updated_at: %w", err)
	}

	run.CreatedAt = parsedCreatedAt
	run.UpdatedAt = parsedUpdatedAt

	return run, nil
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (router.Session, error) {
	var (
		session      router.Session
		kind         string
		status       string
		accountID    sql.NullString
		threadID     sql.NullString
		createdAt    string
		updatedAt    string
		lastActiveAt string
		lastResetAt  sql.NullString
	)

	if err := scanner.Scan(
		&session.SessionID,
		&kind,
		&status,
		&session.Channel,
		&accountID,
		&session.PeerID,
		&threadID,
		&session.RoutingKey,
		&session.Generation,
		&createdAt,
		&updatedAt,
		&lastActiveAt,
		&lastResetAt,
	); err != nil {
		return router.Session{}, fmt.Errorf("scan session: %w", err)
	}

	session.Kind = router.SessionKind(kind)
	session.Status = router.SessionStatus(status)
	session.AccountID = accountID.String
	session.ThreadID = threadID.String

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session updated_at: %w", err)
	}
	parsedLastActiveAt, err := time.Parse(time.RFC3339Nano, lastActiveAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session last_active_at: %w", err)
	}

	session.CreatedAt = parsedCreatedAt
	session.UpdatedAt = parsedUpdatedAt
	session.LastActiveAt = parsedLastActiveAt

	if lastResetAt.Valid {
		parsedLastResetAt, err := time.Parse(time.RFC3339Nano, lastResetAt.String)
		if err != nil {
			return router.Session{}, fmt.Errorf("parse session last_reset_at: %w", err)
		}
		session.LastResetAt = &parsedLastResetAt
	}

	return session, nil
}

func scanPairing(scanner interface {
	Scan(dest ...any) error
}) (auth.Pairing, error) {
	var (
		pairing     auth.Pairing
		mode        string
		status      string
		codePreview sql.NullString
		createdAt   string
		updatedAt   string
		expiresAt   string
		completedAt sql.NullString
	)

	if err := scanner.Scan(
		&pairing.PairingID,
		&mode,
		&pairing.Label,
		&status,
		&pairing.CodeHash,
		&codePreview,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&completedAt,
	); err != nil {
		return auth.Pairing{}, fmt.Errorf("scan pairing: %w", err)
	}

	pairing.Mode = auth.PairingMode(mode)
	pairing.Status = auth.PairingStatus(status)
	pairing.CodePreview = codePreview.String
	if err := assignRequiredTime(&pairing.CreatedAt, createdAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignRequiredTime(&pairing.UpdatedAt, updatedAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignRequiredTime(&pairing.ExpiresAt, expiresAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignOptionalTime(&pairing.CompletedAt, completedAt); err != nil {
		return auth.Pairing{}, err
	}

	return pairing, nil
}

func scanAccessToken(scanner interface {
	Scan(dest ...any) error
}) (auth.AccessToken, error) {
	var (
		token              auth.AccessToken
		principalID        sql.NullString
		mode               string
		status             string
		defaultTenantID    sql.NullString
		createdAt          string
		updatedAt          string
		lastUsedAt         sql.NullString
		expiresAt          sql.NullString
		revokedAt          sql.NullString
		rotatedFromTokenID sql.NullString
		rotatedToTokenID   sql.NullString
	)

	if err := scanner.Scan(
		&token.TokenID,
		&principalID,
		&token.Label,
		&mode,
		&token.TokenHash,
		&token.TokenPreview,
		&status,
		&defaultTenantID,
		&createdAt,
		&updatedAt,
		&lastUsedAt,
		&expiresAt,
		&revokedAt,
		&rotatedFromTokenID,
		&rotatedToTokenID,
	); err != nil {
		return auth.AccessToken{}, fmt.Errorf("scan access token: %w", err)
	}

	token.PrincipalID = principalID.String
	token.Mode = auth.PairingMode(mode)
	token.Status = status
	if token.Status == "" {
		token.Status = "active"
	}
	token.DefaultTenantID = defaultTenantID.String
	if err := assignRequiredTime(&token.CreatedAt, createdAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignRequiredTime(&token.UpdatedAt, updatedAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignOptionalTime(&token.LastUsedAt, lastUsedAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignOptionalTime(&token.ExpiresAt, expiresAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignOptionalTime(&token.RevokedAt, revokedAt); err != nil {
		return auth.AccessToken{}, err
	}
	token.RotatedFromTokenID = rotatedFromTokenID.String
	token.RotatedToTokenID = rotatedToTokenID.String

	return token, nil
}

func scanTenant(scanner interface {
	Scan(dest ...any) error
}) (identity.Tenant, error) {
	var (
		tenant                  identity.Tenant
		tenantKind              string
		status                  string
		createdAt               string
		updatedAt               string
		createdByPrincipalID    sql.NullString
		defaultOwnerPrincipalID sql.NullString
	)
	if err := scanner.Scan(
		&tenant.TenantID,
		&tenantKind,
		&tenant.DisplayName,
		&status,
		&createdAt,
		&updatedAt,
		&createdByPrincipalID,
		&defaultOwnerPrincipalID,
	); err != nil {
		return identity.Tenant{}, fmt.Errorf("scan tenant: %w", err)
	}
	tenant.TenantKind = identity.TenantKind(tenantKind)
	tenant.Status = identity.LifecycleStatus(status)
	tenant.CreatedByPrincipalID = createdByPrincipalID.String
	tenant.DefaultOwnerPrincipalID = defaultOwnerPrincipalID.String
	if err := assignRequiredTime(&tenant.CreatedAt, createdAt); err != nil {
		return identity.Tenant{}, err
	}
	if err := assignRequiredTime(&tenant.UpdatedAt, updatedAt); err != nil {
		return identity.Tenant{}, err
	}
	return tenant, nil
}

func scanPrincipal(scanner interface {
	Scan(dest ...any) error
}) (identity.Principal, error) {
	var (
		principal     identity.Principal
		principalKind string
		status        string
		createdAt     string
		updatedAt     string
		disabledAt    sql.NullString
		removedAt     sql.NullString
	)
	if err := scanner.Scan(
		&principal.PrincipalID,
		&principalKind,
		&principal.DisplayName,
		&status,
		&principal.DefaultTenantID,
		&createdAt,
		&updatedAt,
		&disabledAt,
		&removedAt,
	); err != nil {
		return identity.Principal{}, fmt.Errorf("scan principal: %w", err)
	}
	principal.PrincipalKind = identity.PrincipalKind(principalKind)
	principal.Status = identity.LifecycleStatus(status)
	if err := assignRequiredTime(&principal.CreatedAt, createdAt); err != nil {
		return identity.Principal{}, err
	}
	if err := assignRequiredTime(&principal.UpdatedAt, updatedAt); err != nil {
		return identity.Principal{}, err
	}
	if err := assignOptionalTime(&principal.DisabledAt, disabledAt); err != nil {
		return identity.Principal{}, err
	}
	if err := assignOptionalTime(&principal.RemovedAt, removedAt); err != nil {
		return identity.Principal{}, err
	}
	return principal, nil
}

func scanMembership(scanner interface {
	Scan(dest ...any) error
}) (identity.Membership, error) {
	var (
		membership   identity.Membership
		role         string
		status       string
		invitationID sql.NullString
		createdAt    string
		updatedAt    string
		acceptedAt   sql.NullString
		removedAt    sql.NullString
	)
	if err := scanner.Scan(
		&membership.MembershipID,
		&membership.TenantID,
		&membership.PrincipalID,
		&role,
		&status,
		&invitationID,
		&createdAt,
		&updatedAt,
		&acceptedAt,
		&removedAt,
	); err != nil {
		return identity.Membership{}, fmt.Errorf("scan membership: %w", err)
	}
	membership.Role = identity.Role(role)
	membership.Status = identity.LifecycleStatus(status)
	membership.InvitationID = invitationID.String
	if err := assignRequiredTime(&membership.CreatedAt, createdAt); err != nil {
		return identity.Membership{}, err
	}
	if err := assignRequiredTime(&membership.UpdatedAt, updatedAt); err != nil {
		return identity.Membership{}, err
	}
	if err := assignOptionalTime(&membership.AcceptedAt, acceptedAt); err != nil {
		return identity.Membership{}, err
	}
	if err := assignOptionalTime(&membership.RemovedAt, removedAt); err != nil {
		return identity.Membership{}, err
	}
	return membership, nil
}

func scanTenantInvitation(scanner interface {
	Scan(dest ...any) error
}) (identity.TenantInvitation, error) {
	var (
		invitation identity.TenantInvitation
		role       string
		status     string
		createdAt  string
		updatedAt  string
		expiresAt  sql.NullString
		decidedAt  sql.NullString
	)
	if err := scanner.Scan(
		&invitation.InvitationID,
		&invitation.TenantID,
		&invitation.InvitedPrincipalID,
		&invitation.InvitedByPrincipalID,
		&role,
		&status,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&decidedAt,
	); err != nil {
		return identity.TenantInvitation{}, fmt.Errorf("scan tenant invitation: %w", err)
	}
	invitation.Role = identity.Role(role)
	invitation.Status = identity.LifecycleStatus(status)
	if err := assignRequiredTime(&invitation.CreatedAt, createdAt); err != nil {
		return identity.TenantInvitation{}, err
	}
	if err := assignRequiredTime(&invitation.UpdatedAt, updatedAt); err != nil {
		return identity.TenantInvitation{}, err
	}
	if err := assignOptionalTime(&invitation.ExpiresAt, expiresAt); err != nil {
		return identity.TenantInvitation{}, err
	}
	if err := assignOptionalTime(&invitation.DecidedAt, decidedAt); err != nil {
		return identity.TenantInvitation{}, err
	}
	return invitation, nil
}

func scanTokenTenantGrant(scanner interface {
	Scan(dest ...any) error
}) (identity.TokenTenantGrant, error) {
	var (
		grant                identity.TokenTenantGrant
		isDefault            int
		status               string
		createdAt            string
		updatedAt            string
		revokedAt            sql.NullString
		grantedByPrincipalID sql.NullString
	)
	if err := scanner.Scan(
		&grant.GrantID,
		&grant.TokenID,
		&grant.TenantID,
		&isDefault,
		&status,
		&createdAt,
		&updatedAt,
		&revokedAt,
		&grantedByPrincipalID,
	); err != nil {
		return identity.TokenTenantGrant{}, fmt.Errorf("scan token tenant grant: %w", err)
	}
	grant.IsDefault = isDefault == 1
	grant.Status = identity.LifecycleStatus(status)
	grant.GrantedByPrincipalID = grantedByPrincipalID.String
	if err := assignRequiredTime(&grant.CreatedAt, createdAt); err != nil {
		return identity.TokenTenantGrant{}, err
	}
	if err := assignRequiredTime(&grant.UpdatedAt, updatedAt); err != nil {
		return identity.TokenTenantGrant{}, err
	}
	if err := assignOptionalTime(&grant.RevokedAt, revokedAt); err != nil {
		return identity.TokenTenantGrant{}, err
	}
	return grant, nil
}

func scanTenantAuditEvent(scanner interface {
	Scan(dest ...any) error
}) (identity.TenantAuditEvent, error) {
	var (
		event             identity.TenantAuditEvent
		tenantID          sql.NullString
		principalID       sql.NullString
		targetPrincipalID sql.NullString
		tokenID           sql.NullString
		createdAt         string
		documentJSON      sql.NullString
	)
	if err := scanner.Scan(
		&event.AuditEventID,
		&event.EventKind,
		&tenantID,
		&principalID,
		&targetPrincipalID,
		&tokenID,
		&event.Outcome,
		&event.ReasonCode,
		&createdAt,
		&documentJSON,
	); err != nil {
		return identity.TenantAuditEvent{}, fmt.Errorf("scan tenant audit event: %w", err)
	}
	event.TenantID = tenantID.String
	event.PrincipalID = principalID.String
	event.TargetPrincipalID = targetPrincipalID.String
	event.TokenID = tokenID.String
	if err := assignRequiredTime(&event.CreatedAt, createdAt); err != nil {
		return identity.TenantAuditEvent{}, err
	}
	if err := unmarshalNullableJSON(documentJSON, &event.Document); err != nil {
		return identity.TenantAuditEvent{}, fmt.Errorf("decode tenant audit event document: %w", err)
	}
	if event.Document == nil {
		event.Document = map[string]any{}
	}
	return event, nil
}

func scanApproval(scanner interface {
	Scan(dest ...any) error
}) (policy.Approval, error) {
	var (
		approval                policy.Approval
		resourceKind            sql.NullString
		resourceID              sql.NullString
		requestedBy             sql.NullString
		status                  string
		createdAt               string
		updatedAt               string
		resolvedAt              sql.NullString
		resolution              sql.NullString
		comment                 sql.NullString
		integrationBindingsJSON sql.NullString
	)

	if err := scanner.Scan(
		&approval.ApprovalID,
		&approval.Action,
		&resourceKind,
		&resourceID,
		&approval.Reason,
		&requestedBy,
		&status,
		&createdAt,
		&updatedAt,
		&resolvedAt,
		&resolution,
		&comment,
		&integrationBindingsJSON,
	); err != nil {
		return policy.Approval{}, fmt.Errorf("scan approval: %w", err)
	}

	approval.ResourceKind = resourceKind.String
	approval.ResourceID = resourceID.String
	approval.RequestedBy = requestedBy.String
	approval.Status = policy.ApprovalStatus(status)
	approval.Resolution = resolution.String
	approval.Comment = comment.String
	if err := assignRequiredTime(&approval.CreatedAt, createdAt); err != nil {
		return policy.Approval{}, err
	}
	if err := assignRequiredTime(&approval.UpdatedAt, updatedAt); err != nil {
		return policy.Approval{}, err
	}
	if err := assignOptionalTime(&approval.ResolvedAt, resolvedAt); err != nil {
		return policy.Approval{}, err
	}
	if err := unmarshalNullableJSON(integrationBindingsJSON, &approval.IntegrationBindings); err != nil {
		return policy.Approval{}, fmt.Errorf("decode approval integration bindings: %w", err)
	}

	return approval, nil
}

func scanDecision(scanner interface {
	Scan(dest ...any) error
}) (policy.Decision, error) {
	var (
		decision     policy.Decision
		resourceKind sql.NullString
		resourceID   sql.NullString
		outcome      string
		approvalID   sql.NullString
		createdAt    string
	)

	if err := scanner.Scan(
		&decision.DecisionID,
		&decision.Action,
		&resourceKind,
		&resourceID,
		&outcome,
		&decision.Reason,
		&approvalID,
		&createdAt,
	); err != nil {
		return policy.Decision{}, fmt.Errorf("scan decision: %w", err)
	}

	decision.ResourceKind = resourceKind.String
	decision.ResourceID = resourceID.String
	decision.Outcome = policy.DecisionOutcome(outcome)
	decision.ApprovalID = approvalID.String
	if err := assignRequiredTime(&decision.CreatedAt, createdAt); err != nil {
		return policy.Decision{}, err
	}

	return decision, nil
}

func scanConnector(scanner interface {
	Scan(dest ...any) error
}) (connectors.Connector, error) {
	var (
		item            connectors.Connector
		tenantID        sql.NullString
		status          string
		disabledReason  sql.NullString
		secretRefsRaw   string
		nextRestartAt   sql.NullString
		lastRestartAt   sql.NullString
		lastHeartbeatAt sql.NullString
		lastFailure     sql.NullString
		createdAt       string
		updatedAt       string
	)

	if err := scanner.Scan(
		&item.ConnectorID,
		&tenantID,
		&item.Kind,
		&item.DisplayName,
		&status,
		&disabledReason,
		&secretRefsRaw,
		&item.FailureCount,
		&item.RestartCount,
		&item.BackoffSeconds,
		&nextRestartAt,
		&lastRestartAt,
		&lastHeartbeatAt,
		&lastFailure,
		&createdAt,
		&updatedAt,
	); err != nil {
		return connectors.Connector{}, fmt.Errorf("scan connector: %w", err)
	}

	item.TenantID = tenantID.String
	item.Status = connectors.Status(status)
	item.DisabledReason = disabledReason.String
	item.LastFailureReason = lastFailure.String
	if secretRefsRaw != "" {
		if err := json.Unmarshal([]byte(secretRefsRaw), &item.SecretRefs); err != nil {
			return connectors.Connector{}, fmt.Errorf("parse connector secret refs: %w", err)
		}
	}

	if err := assignOptionalTime(&item.NextRestartAt, nextRestartAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector next_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastRestartAt, lastRestartAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector last_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastHeartbeatAt, lastHeartbeatAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector last_heartbeat_at: %w", err)
	}
	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector created_at: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector updated_at: %w", err)
	}

	return item, nil
}

func scanConnectorMessage(scanner interface {
	Scan(dest ...any) error
}) (imtypes.MessageRecord, error) {
	var (
		item                     imtypes.MessageRecord
		direction                string
		status                   string
		externalMessageID        sql.NullString
		sessionID                sql.NullString
		runID                    sql.NullString
		peerID                   sql.NullString
		threadID                 sql.NullString
		authorID                 sql.NullString
		errorText                sql.NullString
		replyToExternalMessageID sql.NullString
		responseToDeliveryID     sql.NullString
		createdAt                string
		updatedAt                string
	)

	if err := scanner.Scan(
		&item.DeliveryID,
		&item.ConnectorID,
		&direction,
		&externalMessageID,
		&sessionID,
		&runID,
		&item.ChannelID,
		&peerID,
		&threadID,
		&authorID,
		&item.Content,
		&status,
		&errorText,
		&replyToExternalMessageID,
		&responseToDeliveryID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return imtypes.MessageRecord{}, fmt.Errorf("scan connector message: %w", err)
	}

	item.Direction = imtypes.DeliveryDirection(direction)
	item.ExternalMessageID = externalMessageID.String
	item.SessionID = sessionID.String
	item.RunID = runID.String
	item.PeerID = peerID.String
	item.ThreadID = threadID.String
	item.AuthorID = authorID.String
	item.Status = imtypes.DeliveryStatus(status)
	item.Error = errorText.String
	item.ReplyToExternalMessageID = replyToExternalMessageID.String
	item.ResponseToDeliveryID = responseToDeliveryID.String

	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return imtypes.MessageRecord{}, fmt.Errorf("parse connector message created_at: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return imtypes.MessageRecord{}, fmt.Errorf("parse connector message updated_at: %w", err)
	}

	return item, nil
}

func scanCapability(scanner interface {
	Scan(dest ...any) error
}) (capabilities.Capability, error) {
	var (
		item            capabilities.Capability
		status          string
		nextRestartAt   sql.NullString
		lastRestartAt   sql.NullString
		lastHeartbeatAt sql.NullString
		lastFailure     sql.NullString
		createdAt       string
		updatedAt       string
	)

	if err := scanner.Scan(
		&item.CapabilityID,
		&item.Kind,
		&item.DisplayName,
		&status,
		&item.FailureCount,
		&item.RestartCount,
		&item.BackoffSeconds,
		&nextRestartAt,
		&lastRestartAt,
		&lastHeartbeatAt,
		&lastFailure,
		&createdAt,
		&updatedAt,
	); err != nil {
		return capabilities.Capability{}, fmt.Errorf("scan capability: %w", err)
	}

	item.Status = capabilities.Status(status)
	item.LastFailureReason = lastFailure.String

	if err := assignOptionalTime(&item.NextRestartAt, nextRestartAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability next_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastRestartAt, lastRestartAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability last_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastHeartbeatAt, lastHeartbeatAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability last_heartbeat_at: %w", err)
	}
	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability created_at: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability updated_at: %w", err)
	}

	return item, nil
}

func scanLLMDispatch(scanner interface {
	Scan(dest ...any) error
}) (llm.Dispatch, error) {
	var (
		dispatch     llm.Dispatch
		messagesRaw  string
		stream       bool
		status       string
		finishReason sql.NullString
		usageRaw     string
		errorCode    sql.NullString
		errorText    sql.NullString
		createdAt    string
		updatedAt    string
		startedAt    sql.NullString
		completedAt  sql.NullString
	)

	if err := scanner.Scan(
		&dispatch.DispatchID,
		&dispatch.Provider,
		&dispatch.Model,
		&messagesRaw,
		&stream,
		&status,
		&dispatch.Output,
		&finishReason,
		&usageRaw,
		&errorCode,
		&errorText,
		&dispatch.TimeoutMs,
		&dispatch.MaxRetries,
		&dispatch.AttemptCount,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return llm.Dispatch{}, err
	}

	dispatch.Stream = stream
	dispatch.Status = llm.DispatchStatus(status)
	dispatch.Partial = dispatch.Status == llm.DispatchStatusPartialFailed
	dispatch.FinishReason = finishReason.String
	dispatch.ErrorCode = errorCode.String
	dispatch.Error = errorText.String

	if err := json.Unmarshal([]byte(messagesRaw), &dispatch.Messages); err != nil {
		return llm.Dispatch{}, fmt.Errorf("decode llm dispatch messages: %w", err)
	}
	if err := json.Unmarshal([]byte(usageRaw), &dispatch.Usage); err != nil {
		return llm.Dispatch{}, fmt.Errorf("decode llm dispatch usage: %w", err)
	}
	if err := assignRequiredTime(&dispatch.CreatedAt, createdAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch created_at: %w", err)
	}
	if err := assignRequiredTime(&dispatch.UpdatedAt, updatedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch updated_at: %w", err)
	}
	if err := assignOptionalTime(&dispatch.StartedAt, startedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch started_at: %w", err)
	}
	if err := assignOptionalTime(&dispatch.CompletedAt, completedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch completed_at: %w", err)
	}

	return dispatch, nil
}

func scanProviderCheck(scanner interface {
	Scan(dest ...any) error
}) (providers.Check, error) {
	var (
		item         providers.Check
		family       string
		authMode     string
		status       string
		endpoint     sql.NullString
		errorClass   sql.NullString
		errorCode    sql.NullString
		errorMessage sql.NullString
		usageRaw     string
		createdAt    string
		completedAt  string
	)

	if err := scanner.Scan(
		&item.CheckID,
		&item.ProviderID,
		&family,
		&authMode,
		&status,
		&item.Model,
		&endpoint,
		&errorClass,
		&errorCode,
		&errorMessage,
		&usageRaw,
		&createdAt,
		&completedAt,
	); err != nil {
		return providers.Check{}, fmt.Errorf("scan provider check: %w", err)
	}

	item.Family = providers.Family(family)
	item.AuthMode = providers.AuthMode(authMode)
	item.Status = providers.CheckStatus(status)
	item.Endpoint = endpoint.String
	item.ErrorClass = providers.CheckErrorClass(errorClass.String)
	item.ErrorCode = errorCode.String
	item.ErrorMessage = errorMessage.String

	if err := json.Unmarshal([]byte(usageRaw), &item.Usage); err != nil {
		return providers.Check{}, fmt.Errorf("decode provider check usage: %w", err)
	}
	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return providers.Check{}, fmt.Errorf("parse provider check created_at: %w", err)
	}
	if err := assignRequiredTime(&item.CompletedAt, completedAt); err != nil {
		return providers.Check{}, fmt.Errorf("parse provider check completed_at: %w", err)
	}

	return item, nil
}

func scanProviderAuthState(scanner interface {
	Scan(dest ...any) error
}) (providers.AuthState, error) {
	var (
		item                providers.AuthState
		family              string
		authMode            string
		status              string
		cliPath             sql.NullString
		cliAvailable        int
		accountLabel        sql.NullString
		accountID           sql.NullString
		plan                sql.NullString
		authMethod          sql.NullString
		loginCommandRaw     string
		logoutCommandRaw    string
		lastCheckedAt       string
		lastAuthenticatedAt sql.NullString
		lastError           sql.NullString
		metadataRaw         string
		sandboxRaw          sql.NullString
		tenantID            sql.NullString
	)

	if err := scanner.Scan(
		&item.ProviderID,
		&tenantID,
		&family,
		&authMode,
		&status,
		&cliPath,
		&cliAvailable,
		&accountLabel,
		&accountID,
		&plan,
		&authMethod,
		&loginCommandRaw,
		&logoutCommandRaw,
		&lastCheckedAt,
		&lastAuthenticatedAt,
		&lastError,
		&metadataRaw,
		&sandboxRaw,
	); err != nil {
		return providers.AuthState{}, fmt.Errorf("scan provider auth state: %w", err)
	}

	item.TenantID = tenantID.String
	item.Family = providers.Family(family)
	item.AuthMode = providers.AuthMode(authMode)
	item.Status = providers.AuthStatus(status)
	item.CLIPath = cliPath.String
	item.CLIAvailable = cliAvailable == 1
	item.AccountLabel = accountLabel.String
	item.AccountID = accountID.String
	item.Plan = plan.String
	item.AuthMethod = authMethod.String
	item.LastError = lastError.String
	if err := json.Unmarshal([]byte(loginCommandRaw), &item.LoginCommand); err != nil {
		return providers.AuthState{}, fmt.Errorf("decode provider auth login command: %w", err)
	}
	if err := json.Unmarshal([]byte(logoutCommandRaw), &item.LogoutCommand); err != nil {
		return providers.AuthState{}, fmt.Errorf("decode provider auth logout command: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataRaw), &item.Metadata); err != nil {
		return providers.AuthState{}, fmt.Errorf("decode provider auth metadata: %w", err)
	}
	if err := unmarshalNullableJSON(sandboxRaw, &item.Sandbox); err != nil {
		return providers.AuthState{}, fmt.Errorf("decode provider auth sandbox metadata: %w", err)
	}
	if err := assignRequiredTime(&item.LastCheckedAt, lastCheckedAt); err != nil {
		return providers.AuthState{}, fmt.Errorf("parse provider auth last_checked_at: %w", err)
	}
	if err := assignOptionalTimeString(&item.LastAuthenticatedAt, lastAuthenticatedAt); err != nil {
		return providers.AuthState{}, fmt.Errorf("parse provider auth last_authenticated_at: %w", err)
	}
	return item, nil
}

func scanProviderModel(scanner interface {
	Scan(dest ...any) error
}) (providers.Model, error) {
	var (
		item                providers.Model
		description         sql.NullString
		defaultFlag         int
		availableFlag       int
		chat                int
		stream              int
		coding              int
		toolUse             int
		reasoningLevelsJSON string
	)

	if err := scanner.Scan(
		&item.ProviderID,
		&item.ModelID,
		&item.DisplayName,
		&description,
		&defaultFlag,
		&availableFlag,
		&item.Source,
		&chat,
		&stream,
		&coding,
		&toolUse,
		&reasoningLevelsJSON,
	); err != nil {
		return providers.Model{}, fmt.Errorf("scan provider model: %w", err)
	}

	item.Description = description.String
	item.Default = defaultFlag == 1
	item.Available = availableFlag == 1
	item.Chat = chat == 1
	item.Stream = stream == 1
	item.Coding = coding == 1
	item.ToolUse = toolUse == 1
	if err := json.Unmarshal([]byte(reasoningLevelsJSON), &item.ReasoningLevels); err != nil {
		return providers.Model{}, fmt.Errorf("decode provider model reasoning levels: %w", err)
	}
	return item, nil
}

func scanProviderPreference(scanner interface {
	Scan(dest ...any) error
}) (providers.Preference, error) {
	var (
		item      providers.Preference
		updatedAt string
	)
	if err := scanner.Scan(&item.ProviderID, &item.DefaultModel, &updatedAt); err != nil {
		return providers.Preference{}, fmt.Errorf("scan provider preference: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return providers.Preference{}, fmt.Errorf("parse provider preference updated_at: %w", err)
	}
	return item, nil
}

func scanSandboxExecution(scanner interface {
	Scan(dest ...any) error
}) (SandboxExecutionRecord, error) {
	var (
		record      SandboxExecutionRecord
		approvalID  sql.NullString
		requestedAt string
		updatedAt   string
		startedAt   sql.NullString
		completedAt sql.NullString
		document    string
	)
	if err := scanner.Scan(
		&record.ExecutionID,
		&record.ProfileID,
		&record.BackendKind,
		&record.Status,
		&approvalID,
		&requestedAt,
		&updatedAt,
		&startedAt,
		&completedAt,
		&document,
	); err != nil {
		return SandboxExecutionRecord{}, fmt.Errorf("scan sandbox execution: %w", err)
	}
	record.ApprovalID = approvalID.String
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.RequestedAt, requestedAt); err != nil {
		return SandboxExecutionRecord{}, fmt.Errorf("parse sandbox execution requested_at: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return SandboxExecutionRecord{}, fmt.Errorf("parse sandbox execution updated_at: %w", err)
	}
	if err := assignOptionalTime(&record.StartedAt, startedAt); err != nil {
		return SandboxExecutionRecord{}, fmt.Errorf("parse sandbox execution started_at: %w", err)
	}
	if err := assignOptionalTime(&record.CompletedAt, completedAt); err != nil {
		return SandboxExecutionRecord{}, fmt.Errorf("parse sandbox execution completed_at: %w", err)
	}
	return record, nil
}

func scanWorkflowRecord(scanner interface {
	Scan(dest ...any) error
}) (WorkflowRecord, error) {
	var (
		record            WorkflowRecord
		scheduleID        sql.NullString
		scheduleAttemptID sql.NullString
		planSummary       sql.NullString
		failureSummary    sql.NullString
		createdAt         string
		updatedAt         string
		startedAt         sql.NullString
		completedAt       sql.NullString
		interruptedAt     sql.NullString
		document          string
	)
	if err := scanner.Scan(
		&record.WorkflowID,
		&record.RunID,
		&scheduleID,
		&scheduleAttemptID,
		&record.EnvironmentScope,
		&record.Goal,
		&record.Status,
		&planSummary,
		&failureSummary,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
		&interruptedAt,
		&document,
	); err != nil {
		return WorkflowRecord{}, fmt.Errorf("scan workflow: %w", err)
	}
	record.ScheduleID = scheduleID.String
	record.ScheduleAttemptID = scheduleAttemptID.String
	record.PlanSummary = planSummary.String
	record.FailureSummary = failureSummary.String
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.CreatedAt, createdAt); err != nil {
		return WorkflowRecord{}, fmt.Errorf("parse workflow created_at: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return WorkflowRecord{}, fmt.Errorf("parse workflow updated_at: %w", err)
	}
	if err := assignOptionalTime(&record.StartedAt, startedAt); err != nil {
		return WorkflowRecord{}, fmt.Errorf("parse workflow started_at: %w", err)
	}
	if err := assignOptionalTime(&record.CompletedAt, completedAt); err != nil {
		return WorkflowRecord{}, fmt.Errorf("parse workflow completed_at: %w", err)
	}
	if err := assignOptionalTime(&record.InterruptedAt, interruptedAt); err != nil {
		return WorkflowRecord{}, fmt.Errorf("parse workflow interrupted_at: %w", err)
	}
	return record, nil
}

func scanReminderRecord(scanner interface {
	Scan(dest ...any) error
}) (ReminderRecord, error) {
	var (
		record             ReminderRecord
		tenantID           sql.NullString
		nextDueAt          sql.NullString
		activeOccurrenceID sql.NullString
		updatedAt          string
		document           string
	)
	if err := scanner.Scan(
		&record.ReminderID,
		&record.EnvironmentScope,
		&tenantID,
		&record.BehaviorMode,
		&record.CurrentState,
		&nextDueAt,
		&activeOccurrenceID,
		&updatedAt,
		&document,
	); err != nil {
		return ReminderRecord{}, fmt.Errorf("scan reminder: %w", err)
	}
	record.TenantID = tenantID.String
	record.ActiveOccurrenceID = activeOccurrenceID.String
	if err := assignOptionalTime(&record.NextDueAt, nextDueAt); err != nil {
		return ReminderRecord{}, fmt.Errorf("parse reminder next_due_at: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return ReminderRecord{}, fmt.Errorf("parse reminder updated_at: %w", err)
	}
	record.Document = []byte(document)
	return record, nil
}

func scanReminderOccurrenceRecord(scanner interface {
	Scan(dest ...any) error
}) (ReminderOccurrenceRecord, error) {
	var (
		record               ReminderOccurrenceRecord
		runID                sql.NullString
		workflowID           sql.NullString
		latestDeliveryID     sql.NullString
		latestDeliveryStatus sql.NullString
		scheduledFor         string
		updatedAt            string
		document             string
	)
	if err := scanner.Scan(
		&record.OccurrenceID,
		&record.ReminderID,
		&record.EnvironmentScope,
		&record.State,
		&scheduledFor,
		&runID,
		&workflowID,
		&latestDeliveryID,
		&latestDeliveryStatus,
		&updatedAt,
		&document,
	); err != nil {
		return ReminderOccurrenceRecord{}, fmt.Errorf("scan reminder occurrence: %w", err)
	}
	record.RunID = runID.String
	record.WorkflowID = workflowID.String
	record.LatestDeliveryID = latestDeliveryID.String
	record.LatestDeliveryStatus = latestDeliveryStatus.String
	if err := assignRequiredTime(&record.ScheduledFor, scheduledFor); err != nil {
		return ReminderOccurrenceRecord{}, fmt.Errorf("parse reminder occurrence scheduled_for: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return ReminderOccurrenceRecord{}, fmt.Errorf("parse reminder occurrence updated_at: %w", err)
	}
	record.Document = []byte(document)
	return record, nil
}

func scanReminderActionRecord(scanner interface {
	Scan(dest ...any) error
}) (ReminderActionRecord, error) {
	var (
		record       ReminderActionRecord
		occurrenceID sql.NullString
		newState     sql.NullString
		runID        sql.NullString
		workflowID   sql.NullString
		deliveryID   sql.NullString
		createdAt    string
		document     string
	)
	if err := scanner.Scan(
		&record.ActionID,
		&record.ReminderID,
		&occurrenceID,
		&record.ActionKind,
		&newState,
		&runID,
		&workflowID,
		&deliveryID,
		&createdAt,
		&document,
	); err != nil {
		return ReminderActionRecord{}, fmt.Errorf("scan reminder action: %w", err)
	}
	record.OccurrenceID = occurrenceID.String
	record.NewState = newState.String
	record.RunID = runID.String
	record.WorkflowID = workflowID.String
	record.DeliveryID = deliveryID.String
	if err := assignRequiredTime(&record.CreatedAt, createdAt); err != nil {
		return ReminderActionRecord{}, fmt.Errorf("parse reminder action created_at: %w", err)
	}
	record.Document = []byte(document)
	return record, nil
}

func scanWorkflowStepRecord(scanner interface {
	Scan(dest ...any) error
}) (WorkflowStepRecord, error) {
	var (
		record           WorkflowStepRecord
		runtimeStepID    sql.NullString
		activeToolCallID sql.NullString
		lastFailureClass sql.NullString
		blockedReason    sql.NullString
		document         string
	)
	if err := scanner.Scan(
		&record.WorkflowStepID,
		&record.WorkflowID,
		&record.Position,
		&record.Status,
		&runtimeStepID,
		&activeToolCallID,
		&record.AttemptCount,
		&record.MaxAttempts,
		&lastFailureClass,
		&blockedReason,
		&document,
	); err != nil {
		return WorkflowStepRecord{}, fmt.Errorf("scan workflow step: %w", err)
	}
	record.RuntimeStepID = runtimeStepID.String
	record.ActiveToolCallID = activeToolCallID.String
	record.LastFailureClass = lastFailureClass.String
	record.BlockedReason = blockedReason.String
	record.Document = []byte(document)
	return record, nil
}

func scanWorkflowDependencyRecord(scanner interface {
	Scan(dest ...any) error
}) (WorkflowDependencyRecord, error) {
	var record WorkflowDependencyRecord
	var document string
	if err := scanner.Scan(&record.DependencyID, &record.WorkflowID, &document); err != nil {
		return WorkflowDependencyRecord{}, fmt.Errorf("scan workflow dependency: %w", err)
	}
	record.Document = []byte(document)
	return record, nil
}

func scanWorkflowHandoffRecord(scanner interface {
	Scan(dest ...any) error
}) (WorkflowHandoffRecord, error) {
	var record WorkflowHandoffRecord
	var document string
	if err := scanner.Scan(&record.HandoffID, &record.WorkflowID, &record.Status, &document); err != nil {
		return WorkflowHandoffRecord{}, fmt.Errorf("scan workflow handoff: %w", err)
	}
	record.Document = []byte(document)
	return record, nil
}

func scanScheduleRecord(scanner interface {
	Scan(dest ...any) error
}) (ScheduleRecord, error) {
	var (
		record        ScheduleRecord
		tenantID      sql.NullString
		timezone      sql.NullString
		nextDueAt     sql.NullString
		lastAttemptAt sql.NullString
		lastOutcome   sql.NullString
		createdAt     string
		updatedAt     string
		pausedAt      sql.NullString
		cancelledAt   sql.NullString
		completedAt   sql.NullString
		document      string
	)
	if err := scanner.Scan(
		&record.ScheduleID,
		&record.EnvironmentScope,
		&tenantID,
		&record.Kind,
		&record.Status,
		&record.TargetRefID,
		&timezone,
		&nextDueAt,
		&lastAttemptAt,
		&lastOutcome,
		&createdAt,
		&updatedAt,
		&pausedAt,
		&cancelledAt,
		&completedAt,
		&document,
	); err != nil {
		return ScheduleRecord{}, fmt.Errorf("scan schedule: %w", err)
	}
	record.TenantID = tenantID.String
	record.Timezone = timezone.String
	record.LastOutcome = lastOutcome.String
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.CreatedAt, createdAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule created_at: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule updated_at: %w", err)
	}
	if err := assignOptionalTime(&record.NextDueAt, nextDueAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule next_due_at: %w", err)
	}
	if err := assignOptionalTime(&record.LastAttemptAt, lastAttemptAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule last_attempt_at: %w", err)
	}
	if err := assignOptionalTime(&record.PausedAt, pausedAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule paused_at: %w", err)
	}
	if err := assignOptionalTime(&record.CancelledAt, cancelledAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule cancelled_at: %w", err)
	}
	if err := assignOptionalTime(&record.CompletedAt, completedAt); err != nil {
		return ScheduleRecord{}, fmt.Errorf("parse schedule completed_at: %w", err)
	}
	return record, nil
}

func scanScheduleTargetRecord(scanner interface {
	Scan(dest ...any) error
}) (ScheduleTargetRecord, error) {
	var (
		record    ScheduleTargetRecord
		active    int
		updatedAt string
		document  string
	)
	if err := scanner.Scan(
		&record.TargetRefID,
		&record.ScheduleID,
		&record.TargetKind,
		&record.Revision,
		&active,
		&updatedAt,
		&document,
	); err != nil {
		return ScheduleTargetRecord{}, fmt.Errorf("scan schedule target: %w", err)
	}
	record.Active = active != 0
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return ScheduleTargetRecord{}, fmt.Errorf("parse schedule target updated_at: %w", err)
	}
	return record, nil
}

func scanScheduleDispatchAttemptRecord(scanner interface {
	Scan(dest ...any) error
}) (ScheduleDispatchAttemptRecord, error) {
	var (
		record        ScheduleDispatchAttemptRecord
		failureClass  sql.NullString
		failureReason sql.NullString
		nextRetryAt   sql.NullString
		runID         sql.NullString
		workflowID    sql.NullString
		skippedReason sql.NullString
		dueAt         string
		createdAt     string
		updatedAt     string
		document      string
	)
	if err := scanner.Scan(
		&record.AttemptID,
		&record.ScheduleID,
		&dueAt,
		&record.TriggerSource,
		&record.DispatchStatus,
		&failureClass,
		&failureReason,
		&record.RetryCount,
		&record.RetryBudget,
		&nextRetryAt,
		&record.ResolvedTargetRevision,
		&runID,
		&workflowID,
		&record.DownstreamStatus,
		&skippedReason,
		&record.MissedCount,
		&createdAt,
		&updatedAt,
		&document,
	); err != nil {
		return ScheduleDispatchAttemptRecord{}, fmt.Errorf("scan schedule attempt: %w", err)
	}
	record.FailureClass = failureClass.String
	record.FailureReason = failureReason.String
	record.RunID = runID.String
	record.WorkflowID = workflowID.String
	record.SkippedReason = skippedReason.String
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.DueAt, dueAt); err != nil {
		return ScheduleDispatchAttemptRecord{}, fmt.Errorf("parse schedule attempt due_at: %w", err)
	}
	if err := assignRequiredTime(&record.CreatedAt, createdAt); err != nil {
		return ScheduleDispatchAttemptRecord{}, fmt.Errorf("parse schedule attempt created_at: %w", err)
	}
	if err := assignRequiredTime(&record.UpdatedAt, updatedAt); err != nil {
		return ScheduleDispatchAttemptRecord{}, fmt.Errorf("parse schedule attempt updated_at: %w", err)
	}
	if err := assignOptionalTime(&record.NextRetryAt, nextRetryAt); err != nil {
		return ScheduleDispatchAttemptRecord{}, fmt.Errorf("parse schedule attempt next_retry_at: %w", err)
	}
	return record, nil
}

func scanStep(scanner interface {
	Scan(dest ...any) error
}) (runtime.Step, error) {
	var (
		step           runtime.Step
		workflowID     sql.NullString
		workflowStepID sql.NullString
		status         string
		inputJSON      sql.NullString
		outputJSON     sql.NullString
		createdAt      string
		updatedAt      string
	)

	if err := scanner.Scan(
		&step.StepID,
		&step.RunID,
		&workflowID,
		&workflowStepID,
		&step.Attempt,
		&step.Title,
		&step.Kind,
		&status,
		&inputJSON,
		&outputJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.Step{}, fmt.Errorf("scan step: %w", err)
	}

	step.WorkflowID = workflowID.String
	step.WorkflowStepID = workflowStepID.String
	step.Status = runtime.StepStatus(status)

	if err := unmarshalNullableJSON(inputJSON, &step.Input); err != nil {
		return runtime.Step{}, fmt.Errorf("decode step input: %w", err)
	}
	if err := unmarshalNullableJSON(outputJSON, &step.Output); err != nil {
		return runtime.Step{}, fmt.Errorf("decode step output: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.Step{}, fmt.Errorf("parse step created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.Step{}, fmt.Errorf("parse step updated_at: %w", err)
	}

	step.CreatedAt = parsedCreatedAt
	step.UpdatedAt = parsedUpdatedAt

	return step, nil
}

func scanToolCall(scanner interface {
	Scan(dest ...any) error
}) (runtime.ToolCall, error) {
	var (
		toolCall                runtime.ToolCall
		workflowID              sql.NullString
		workflowStepID          sql.NullString
		computerUseSessionID    sql.NullString
		computerUseActionID     sql.NullString
		invocationKind          sql.NullString
		skillID                 sql.NullString
		mcpServerID             sql.NullString
		mcpServerName           sql.NullString
		mcpToolName             sql.NullString
		mcpTransportKind        sql.NullString
		mcpSessionID            sql.NullString
		authorizationResult     sql.NullString
		sandboxExecutionID      sql.NullString
		failureClass            sql.NullString
		status                  string
		inputJSON               sql.NullString
		outputJSON              sql.NullString
		sandboxJSON             sql.NullString
		integrationBindingsJSON sql.NullString
		errorText               sql.NullString
		createdAt               string
		updatedAt               string
	)

	if err := scanner.Scan(
		&toolCall.ToolCallID,
		&toolCall.RunID,
		&toolCall.StepID,
		&workflowID,
		&workflowStepID,
		&toolCall.Attempt,
		&computerUseSessionID,
		&computerUseActionID,
		&invocationKind,
		&toolCall.CapabilityID,
		&skillID,
		&mcpServerID,
		&mcpServerName,
		&mcpToolName,
		&mcpTransportKind,
		&mcpSessionID,
		&authorizationResult,
		&toolCall.ToolName,
		&status,
		&sandboxExecutionID,
		&failureClass,
		&inputJSON,
		&outputJSON,
		&sandboxJSON,
		&integrationBindingsJSON,
		&errorText,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("scan tool call: %w", err)
	}

	toolCall.WorkflowID = workflowID.String
	toolCall.WorkflowStepID = workflowStepID.String
	toolCall.ComputerUseSessionID = computerUseSessionID.String
	toolCall.ComputerUseActionID = computerUseActionID.String
	toolCall.Status = runtime.ToolCallStatus(status)
	toolCall.InvocationKind = runtime.ToolCallInvocationKind(invocationKind.String)
	toolCall.SkillID = skillID.String
	toolCall.MCPServerID = mcpServerID.String
	toolCall.MCPServerName = mcpServerName.String
	toolCall.MCPToolName = mcpToolName.String
	toolCall.MCPTransportKind = mcpTransportKind.String
	toolCall.MCPSessionID = mcpSessionID.String
	toolCall.AuthorizationResult = authorizationResult.String
	toolCall.SandboxExecutionID = sandboxExecutionID.String
	toolCall.FailureClass = failureClass.String
	toolCall.Error = errorText.String

	if err := unmarshalNullableJSON(inputJSON, &toolCall.Input); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call input: %w", err)
	}
	if err := unmarshalNullableJSON(outputJSON, &toolCall.Output); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call output: %w", err)
	}
	if err := unmarshalNullableJSON(sandboxJSON, &toolCall.Sandbox); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call sandbox metadata: %w", err)
	}
	if err := unmarshalNullableJSON(integrationBindingsJSON, &toolCall.IntegrationBindings); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call integration bindings: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.ToolCall{}, fmt.Errorf("parse tool call created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.ToolCall{}, fmt.Errorf("parse tool call updated_at: %w", err)
	}

	toolCall.CreatedAt = parsedCreatedAt
	toolCall.UpdatedAt = parsedUpdatedAt

	return toolCall, nil
}

func scanIntegrationRecord(scanner interface {
	Scan(dest ...any) error
}) (IntegrationRecord, error) {
	var (
		record           IntegrationRecord
		accountKey       sql.NullString
		backendKind      string
		readinessStatus  string
		canonicalDefault int
		updatedAt        string
		document         string
	)
	if err := scanner.Scan(
		&record.IntegrationID,
		&record.DomainKind,
		&record.EnvironmentScope,
		&accountKey,
		&backendKind,
		&readinessStatus,
		&canonicalDefault,
		&updatedAt,
		&document,
	); err != nil {
		return IntegrationRecord{}, fmt.Errorf("scan integration record: %w", err)
	}
	record.AccountKey = accountKey.String
	record.BackendKind = backendKind
	record.ReadinessStatus = readinessStatus
	record.CanonicalDefault = canonicalDefault != 0
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return IntegrationRecord{}, fmt.Errorf("parse integration updated_at: %w", err)
	}
	record.UpdatedAt = parsedUpdatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanCalendarAccountRecord(scanner interface {
	Scan(dest ...any) error
}) (CalendarAccountRecord, error) {
	var (
		record           CalendarAccountRecord
		accountKey       sql.NullString
		readinessStatus  string
		canonicalDefault int
		updatedAt        string
		document         string
	)
	if err := scanner.Scan(
		&record.CalendarAccountID,
		&record.IntegrationID,
		&record.EnvironmentScope,
		&accountKey,
		&readinessStatus,
		&canonicalDefault,
		&updatedAt,
		&document,
	); err != nil {
		return CalendarAccountRecord{}, fmt.Errorf("scan calendar account record: %w", err)
	}
	record.AccountKey = accountKey.String
	record.ReadinessStatus = readinessStatus
	record.CanonicalDefault = canonicalDefault != 0
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return CalendarAccountRecord{}, fmt.Errorf("parse calendar account updated_at: %w", err)
	}
	record.UpdatedAt = parsedUpdatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanCalendarOperationRecord(scanner interface {
	Scan(dest ...any) error
}) (CalendarOperationRecord, error) {
	var (
		record          CalendarOperationRecord
		externalEventID sql.NullString
		runID           sql.NullString
		workflowID      sql.NullString
		scheduleID      sql.NullString
		deliveryID      sql.NullString
		updatedAt       string
		document        string
	)
	if err := scanner.Scan(
		&record.OperationID,
		&record.IntegrationID,
		&record.CalendarAccountID,
		&record.EnvironmentScope,
		&record.OperationClass,
		&record.Status,
		&externalEventID,
		&runID,
		&workflowID,
		&scheduleID,
		&deliveryID,
		&updatedAt,
		&document,
	); err != nil {
		return CalendarOperationRecord{}, fmt.Errorf("scan calendar operation record: %w", err)
	}
	record.ExternalEventID = externalEventID.String
	record.RunID = runID.String
	record.WorkflowID = workflowID.String
	record.ScheduleID = scheduleID.String
	record.DeliveryID = deliveryID.String
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return CalendarOperationRecord{}, fmt.Errorf("parse calendar operation updated_at: %w", err)
	}
	record.UpdatedAt = parsedUpdatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanCalendarArtifactRecord(scanner interface {
	Scan(dest ...any) error
}) (CalendarArtifactRecord, error) {
	var (
		record          CalendarArtifactRecord
		externalEventID sql.NullString
		createdAt       string
		document        string
	)
	if err := scanner.Scan(
		&record.ArtifactID,
		&record.OperationID,
		&record.IntegrationID,
		&record.EnvironmentScope,
		&record.Kind,
		&externalEventID,
		&createdAt,
		&document,
	); err != nil {
		return CalendarArtifactRecord{}, fmt.Errorf("scan calendar artifact record: %w", err)
	}
	record.ExternalEventID = externalEventID.String
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return CalendarArtifactRecord{}, fmt.Errorf("parse calendar artifact created_at: %w", err)
	}
	record.CreatedAt = parsedCreatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanMailAccountRecord(scanner interface {
	Scan(dest ...any) error
}) (MailAccountRecord, error) {
	var (
		record           MailAccountRecord
		accountKey       sql.NullString
		readinessStatus  string
		canonicalDefault int
		updatedAt        string
		document         string
	)
	if err := scanner.Scan(
		&record.MailAccountID,
		&record.IntegrationID,
		&record.EnvironmentScope,
		&accountKey,
		&readinessStatus,
		&canonicalDefault,
		&updatedAt,
		&document,
	); err != nil {
		return MailAccountRecord{}, fmt.Errorf("scan mail account record: %w", err)
	}
	record.AccountKey = accountKey.String
	record.ReadinessStatus = readinessStatus
	record.CanonicalDefault = canonicalDefault != 0
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return MailAccountRecord{}, fmt.Errorf("parse mail account updated_at: %w", err)
	}
	record.UpdatedAt = parsedUpdatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanMailOperationRecord(scanner interface {
	Scan(dest ...any) error
}) (MailOperationRecord, error) {
	var (
		record     MailOperationRecord
		threadID   sql.NullString
		messageID  sql.NullString
		draftID    sql.NullString
		runID      sql.NullString
		workflowID sql.NullString
		scheduleID sql.NullString
		deliveryID sql.NullString
		updatedAt  string
		document   string
	)
	if err := scanner.Scan(
		&record.OperationID,
		&record.IntegrationID,
		&record.MailAccountID,
		&record.EnvironmentScope,
		&record.OperationClass,
		&record.Status,
		&record.ResultMode,
		&threadID,
		&messageID,
		&draftID,
		&runID,
		&workflowID,
		&scheduleID,
		&deliveryID,
		&updatedAt,
		&document,
	); err != nil {
		return MailOperationRecord{}, fmt.Errorf("scan mail operation record: %w", err)
	}
	record.ThreadID = threadID.String
	record.MessageID = messageID.String
	record.DraftID = draftID.String
	record.RunID = runID.String
	record.WorkflowID = workflowID.String
	record.ScheduleID = scheduleID.String
	record.DeliveryID = deliveryID.String
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return MailOperationRecord{}, fmt.Errorf("parse mail operation updated_at: %w", err)
	}
	record.UpdatedAt = parsedUpdatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanMailArtifactRecord(scanner interface {
	Scan(dest ...any) error
}) (MailArtifactRecord, error) {
	var (
		record          MailArtifactRecord
		threadID        sql.NullString
		messageID       sql.NullString
		draftID         sql.NullString
		attachmentRefID sql.NullString
		createdAt       string
		document        string
	)
	if err := scanner.Scan(
		&record.ArtifactID,
		&record.OperationID,
		&record.IntegrationID,
		&record.EnvironmentScope,
		&record.Kind,
		&threadID,
		&messageID,
		&draftID,
		&attachmentRefID,
		&createdAt,
		&document,
	); err != nil {
		return MailArtifactRecord{}, fmt.Errorf("scan mail artifact record: %w", err)
	}
	record.ThreadID = threadID.String
	record.MessageID = messageID.String
	record.DraftID = draftID.String
	record.AttachmentRefID = attachmentRefID.String
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return MailArtifactRecord{}, fmt.Errorf("parse mail artifact created_at: %w", err)
	}
	record.CreatedAt = parsedCreatedAt
	record.Document = []byte(document)
	return record, nil
}

func scanConsumerPolicyRecord(scanner interface {
	Scan(dest ...any) error
}) (ConsumerPolicyRecordRecord, error) {
	var (
		record              ConsumerPolicyRecordRecord
		declarationID       sql.NullString
		requestedBy         sql.NullString
		sandboxExecutionID  sql.NullString
		toolCallID          sql.NullString
		providerOperationID sql.NullString
		startedAt           string
		completedAt         sql.NullString
		document            string
	)
	if err := scanner.Scan(
		&record.PolicyRecordID,
		&record.ConsumerKind,
		&record.ConsumerID,
		&record.OperationKind,
		&declarationID,
		&record.Status,
		&record.Decision,
		&record.ApprovalStatus,
		&record.SecretResolution,
		&requestedBy,
		&sandboxExecutionID,
		&toolCallID,
		&providerOperationID,
		&startedAt,
		&completedAt,
		&document,
	); err != nil {
		return ConsumerPolicyRecordRecord{}, fmt.Errorf("scan consumer policy record: %w", err)
	}
	record.DeclarationID = declarationID.String
	record.RequestedBy = requestedBy.String
	record.SandboxExecutionID = sandboxExecutionID.String
	record.ToolCallID = toolCallID.String
	record.ProviderOperationID = providerOperationID.String
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.StartedAt, startedAt); err != nil {
		return ConsumerPolicyRecordRecord{}, fmt.Errorf("parse consumer policy record started_at: %w", err)
	}
	if err := assignOptionalTime(&record.CompletedAt, completedAt); err != nil {
		return ConsumerPolicyRecordRecord{}, fmt.Errorf("parse consumer policy record completed_at: %w", err)
	}
	return record, nil
}

func scanMCPServer(scanner interface {
	Scan(dest ...any) error
}) (MCPServerRecord, error) {
	var (
		record   MCPServerRecord
		enabled  int
		updated  string
		document string
	)
	if err := scanner.Scan(&record.ServerID, &enabled, &updated, &document); err != nil {
		return MCPServerRecord{}, fmt.Errorf("scan mcp server: %w", err)
	}
	record.Enabled = enabled == 1
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.UpdatedAt, updated); err != nil {
		return MCPServerRecord{}, fmt.Errorf("parse mcp server updated_at: %w", err)
	}
	return record, nil
}

func scanMCPServerState(scanner interface {
	Scan(dest ...any) error
}) (MCPServerStateRecord, error) {
	var (
		record   MCPServerStateRecord
		updated  string
		document string
	)
	if err := scanner.Scan(&record.ServerID, &record.Status, &updated, &document); err != nil {
		return MCPServerStateRecord{}, fmt.Errorf("scan mcp server state: %w", err)
	}
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.UpdatedAt, updated); err != nil {
		return MCPServerStateRecord{}, fmt.Errorf("parse mcp server state updated_at: %w", err)
	}
	return record, nil
}

func scanMCPTool(scanner interface {
	Scan(dest ...any) error
}) (MCPToolRecord, error) {
	var (
		record           MCPToolRecord
		updated          string
		lastDiscoveredAt sql.NullString
		document         string
	)
	if err := scanner.Scan(&record.ServerID, &record.ToolName, &record.DiscoveryStatus, &updated, &lastDiscoveredAt, &document); err != nil {
		return MCPToolRecord{}, fmt.Errorf("scan mcp tool: %w", err)
	}
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.UpdatedAt, updated); err != nil {
		return MCPToolRecord{}, fmt.Errorf("parse mcp tool updated_at: %w", err)
	}
	if err := assignOptionalTime(&record.LastDiscoveredAt, lastDiscoveredAt); err != nil {
		return MCPToolRecord{}, fmt.Errorf("parse mcp tool last_discovered_at: %w", err)
	}
	return record, nil
}

func scanMCPToolExposureRule(scanner interface {
	Scan(dest ...any) error
}) (MCPToolExposureRuleRecord, error) {
	var (
		record   MCPToolExposureRuleRecord
		active   int
		updated  string
		document string
	)
	if err := scanner.Scan(&record.ServerID, &record.ToolName, &record.RuntimeSurface, &record.ExposureMode, &active, &updated, &document); err != nil {
		return MCPToolExposureRuleRecord{}, fmt.Errorf("scan mcp tool exposure rule: %w", err)
	}
	record.Active = active == 1
	record.Document = []byte(document)
	if err := assignRequiredTime(&record.UpdatedAt, updated); err != nil {
		return MCPToolExposureRuleRecord{}, fmt.Errorf("parse mcp tool exposure rule updated_at: %w", err)
	}
	return record, nil
}

func scanSecretScopeBinding(scanner interface {
	Scan(dest ...any) error
}) (SecretScopeBindingRecord, error) {
	var (
		record   SecretScopeBindingRecord
		active   int
		document string
	)
	if err := scanner.Scan(
		&record.BindingID,
		&record.ConsumerKind,
		&record.ConsumerID,
		&record.EnvironmentScope,
		&record.SecretRef,
		&record.DefaultSource,
		&record.DeliveryKind,
		&active,
		&document,
	); err != nil {
		return SecretScopeBindingRecord{}, fmt.Errorf("scan secret scope binding: %w", err)
	}
	record.Active = active == 1
	record.Document = []byte(document)
	return record, nil
}

func assignRequiredTime(target *time.Time, value string) error {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return err
	}
	*target = parsed
	return nil
}

func assignOptionalTime(target **time.Time, value sql.NullString) error {
	if !value.Valid || value.String == "" {
		*target = nil
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return err
	}
	*target = &parsed
	return nil
}

func assignOptionalTimeString(target **time.Time, value sql.NullString) error {
	return assignOptionalTime(target, value)
}

func nullableTimeString(value *time.Time) sql.NullString {
	if value == nil || value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: value.UTC().Format(time.RFC3339Nano), Valid: true}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func defaultStringMap(value map[string]string) map[string]string {
	if value != nil {
		return value
	}
	return map[string]string{}
}

func scanEvent(scanner interface {
	Scan(dest ...any) error
}) (events.Event, error) {
	var (
		event             events.Event
		sequence          int64
		environmentScope  string
		occurredAt        string
		sessionID         sql.NullString
		runID             sql.NullString
		workflowID        sql.NullString
		workflowStepID    sql.NullString
		scheduleID        sql.NullString
		scheduleAttemptID sql.NullString
		stepID            sql.NullString
		connectorID       sql.NullString
		capabilityID      sql.NullString
		payloadJSON       sql.NullString
	)

	if err := scanner.Scan(
		&sequence,
		&event.EventID,
		&environmentScope,
		&event.Category,
		&event.Name,
		&occurredAt,
		&sessionID,
		&runID,
		&workflowID,
		&workflowStepID,
		&scheduleID,
		&scheduleAttemptID,
		&stepID,
		&connectorID,
		&capabilityID,
		&event.Resource.Kind,
		&event.Resource.ID,
		&payloadJSON,
	); err != nil {
		return events.Event{}, fmt.Errorf("scan event: %w", err)
	}

	parsedOccurredAt, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		return events.Event{}, fmt.Errorf("parse event occurred_at: %w", err)
	}

	event.Sequence = sequence
	event.EnvironmentScope = environmentScope
	event.OccurredAt = parsedOccurredAt
	event.Scope = events.Scope{
		SessionID:         sessionID.String,
		RunID:             runID.String,
		WorkflowID:        workflowID.String,
		WorkflowStepID:    workflowStepID.String,
		ScheduleID:        scheduleID.String,
		ScheduleAttemptID: scheduleAttemptID.String,
		StepID:            stepID.String,
		ConnectorID:       connectorID.String,
		CapabilityID:      capabilityID.String,
	}
	if err := unmarshalNullableJSON(payloadJSON, &event.Payload); err != nil {
		return events.Event{}, fmt.Errorf("decode event payload: %w", err)
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}

	return event, nil
}

func scanCheckpoint(scanner interface {
	Scan(dest ...any) error
}) (runtime.RunCheckpoint, error) {
	var (
		checkpointID string
		runID        string
		capturedAt   string
		snapshotJSON string
		checkpoint   runtime.RunCheckpoint
	)

	if err := scanner.Scan(&checkpointID, &runID, &capturedAt, &snapshotJSON); err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("scan checkpoint: %w", err)
	}

	if err := json.Unmarshal([]byte(snapshotJSON), &checkpoint); err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("decode checkpoint snapshot: %w", err)
	}
	if checkpoint.Run.RunID != runID {
		return runtime.RunCheckpoint{}, fmt.Errorf("checkpoint run mismatch: row=%s snapshot=%s", runID, checkpoint.Run.RunID)
	}

	parsedCapturedAt, err := time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("parse checkpoint captured_at: %w", err)
	}
	checkpoint.CapturedAt = parsedCapturedAt

	return checkpoint, nil
}

func marshalJSON(value any) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return sql.NullString{}, err
	}

	return sql.NullString{String: string(encoded), Valid: true}, nil
}

func unmarshalNullableJSON(raw sql.NullString, target any) error {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw.String), target)
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullableValueTimeString(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: value.UTC().Format(time.RFC3339Nano), Valid: true}
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func coalesceTenantBinding(values ...string) any {
	value := coalesceString(values...)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func tenantBindingString(value any) string {
	if value == nil {
		return ""
	}
	if tenantID, ok := value.(string); ok {
		return tenantID
	}
	return ""
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed")
}

func newCheckpointID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "ckpt_fallback"
	}
	return "ckpt_" + hex.EncodeToString(buf)
}

func newTenantAuditEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "audit_fallback"
	}
	return "audit_" + hex.EncodeToString(buf)
}
