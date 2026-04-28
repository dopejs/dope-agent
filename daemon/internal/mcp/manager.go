package mcp

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

var (
	ErrServerIDRequired       = errors.New("mcp server id is required")
	ErrDeclarationIDRequired  = errors.New("mcp declaration id is required")
	ErrProfileIDRequired      = errors.New("mcp sandbox profile id is required")
	ErrCommandRequired        = errors.New("mcp command is required")
	ErrUnsupportedTransport   = errors.New("mcp transport kind is unsupported")
	ErrAutoRestartRequiresOn  = errors.New("mcp auto-restart requires enabled server")
	ErrServerNotFound         = errors.New("mcp server not found")
	ErrToolNameRequired       = errors.New("mcp tool name is required")
	ErrRuntimeSurfaceRequired = errors.New("mcp runtime surface is required")
	ErrApprovalIDInvalid      = errors.New("mcp approval does not authorize this tool use")
	ErrTransportNotConfigured = errors.New("mcp transport is not configured")
	ErrSandboxManagerMissing  = errors.New("mcp sandbox manager is not configured")
)

const websocketReconnectMaxAttempts = 3

var mcpBackoffDelay = restartBackoffDelay

func SetReconnectBackoffDelayForTest(delay time.Duration) func() {
	previous := mcpBackoffDelay
	mcpBackoffDelay = func(int) time.Duration { return delay }
	return func() {
		mcpBackoffDelay = previous
	}
}

const (
	resourceKindServer = "mcp_server"
	resourceKindTool   = "mcp_tool"
)

var mcpSessionStartTimeout = 10 * time.Second

func SetSessionStartTimeoutForTest(timeout time.Duration) func() {
	previous := mcpSessionStartTimeout
	mcpSessionStartTimeout = timeout
	return func() {
		mcpSessionStartTimeout = previous
	}
}

func isRestoreLifecycleRequester(requestedBy string) bool {
	return strings.TrimSpace(requestedBy) == "system.restore"
}

func isWebsocketReconnectRequester(requestedBy string) bool {
	return strings.TrimSpace(requestedBy) == "mcp.websocket_reconnect"
}

type attachedExecutionStarter interface {
	StartAttachedExecution(context.Context, sandbox.ExecutionRequest) (sandbox.Execution, *sandbox.AttachedExecution, error)
	CancelExecution(executionID string) (sandbox.Execution, bool, error)
	GetExecution(executionID string) (sandbox.Execution, bool)
	PersistConsumerView(context.Context, *sandbox.ConsumerContractView) error
	GetProfile(profileID string) (sandbox.Profile, bool)
}

type sessionState struct {
	sessionID       string
	executionID     string
	session         Session
	transportKind   TransportKind
	stopRequested   bool
	cancelRequested bool
}

type Manager struct {
	cfg       config.Config
	store     *store.SQLiteStore
	eventBus  *events.Bus
	policy    *policy.Engine
	sandboxes attachedExecutionStarter
	secrets   *secrets.Manager
	transport Transport

	mu        sync.RWMutex
	servers   map[string]Server
	serverIDs []string
	states    map[string]ServerState
	tools     map[string]map[string]Tool
	exposure  map[string]map[string]map[string]ToolExposureRule
	sessions  map[string]*sessionState
}

func NewManager(cfg config.Config, sqliteStore *store.SQLiteStore, eventBus *events.Bus, sandboxManager attachedExecutionStarter, policyEngine *policy.Engine, transport Transport) *Manager {
	if transport == nil {
		transport = NewTransportMux(nil, nil)
	}
	return &Manager{
		cfg:       cfg,
		store:     sqliteStore,
		eventBus:  eventBus,
		policy:    policyEngine,
		sandboxes: sandboxManager,
		transport: transport,
		servers:   map[string]Server{},
		states:    map[string]ServerState{},
		tools:     map[string]map[string]Tool{},
		exposure:  map[string]map[string]map[string]ToolExposureRule{},
		sessions:  map[string]*sessionState{},
	}
}

func (m *Manager) SetSecretManager(secretManager *secrets.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets = secretManager
}

func (m *Manager) ListCatalog() []CatalogEntry {
	return bundledCatalogEntries(m.cfg)
}

func (m *Manager) ListTransportCapabilities() []TransportCapability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := []TransportCapability{
		{
			TransportKind:          TransportKindStdio,
			AvailabilityStatus:     AvailabilityStatusReady,
			HealthStatus:           TransportHealthStatusHealthy,
			Prerequisites:          []string{"stdio command must be configured per server", "sandbox profile must remain available for subprocess execution"},
			EnvironmentScope:       string(m.cfg.Environment),
			DaemonManagedReconnect: false,
			RecoverySummary:        "stdio sessions restart through the existing daemon-owned lifecycle path",
		},
		{
			TransportKind:          TransportKindStreamableHTTP,
			AvailabilityStatus:     AvailabilityStatusReady,
			HealthStatus:           TransportHealthStatusHealthy,
			Prerequisites:          []string{"streamable-http endpoint must be configured per server", "remote endpoint reachability is evaluated per server"},
			EnvironmentScope:       string(m.cfg.Environment),
			DaemonManagedReconnect: false,
			RecoverySummary:        "streamable-http sessions restart through the normal lifecycle path",
		},
		{
			TransportKind:          TransportKindWebsocket,
			AvailabilityStatus:     AvailabilityStatusReady,
			HealthStatus:           TransportHealthStatusHealthy,
			Prerequisites:          []string{"websocket endpoint must be configured per server", "authenticated endpoints require secret-ref-backed header auth"},
			EnvironmentScope:       string(m.cfg.Environment),
			SupportedAuthKinds:     []string{string(WebsocketAuthModeBearerHeader), string(WebsocketAuthModeHeader)},
			DaemonManagedReconnect: true,
			RecoverySummary:        "daemon manages bounded websocket reconnect and restore history",
		},
	}

	for _, serverID := range m.serverIDs {
		server := m.servers[serverID]
		if server.EnvironmentScope != "" && server.EnvironmentScope != string(m.cfg.Environment) {
			continue
		}
		state := m.states[serverID]
		for i := range items {
			if items[i].TransportKind != server.TransportKind {
				continue
			}
			switch state.Status {
			case LifecycleStatusDegraded, LifecycleStatusBackingOff:
				items[i].HealthStatus = TransportHealthStatusDegraded
				items[i].Reason = firstNonEmpty(items[i].Reason, state.HealthReason, "one or more servers are recovering")
			case LifecycleStatusUnsupported:
				items[i].AvailabilityStatus = AvailabilityStatusUnsupported
				items[i].Reason = firstNonEmpty(items[i].Reason, state.HealthReason, "transport is unsupported for at least one configured server")
			}
		}
	}

	return items
}

func (m *Manager) GetCatalogEntry(entryID string) (CatalogEntry, bool) {
	for _, entry := range bundledCatalogEntries(m.cfg) {
		if entry.ID == strings.TrimSpace(entryID) {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func (m *Manager) Restore(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	serverRecords, err := m.store.ListMCPServers(ctx)
	if err != nil {
		return fmt.Errorf("list mcp servers: %w", err)
	}
	stateRecords, err := m.store.ListMCPServerStates(ctx)
	if err != nil {
		return fmt.Errorf("list mcp server states: %w", err)
	}
	toolRecords, err := m.store.ListMCPTools(ctx, "")
	if err != nil {
		return fmt.Errorf("list mcp tools: %w", err)
	}
	exposureRecords, err := m.store.ListMCPToolExposureRules(ctx, "")
	if err != nil {
		return fmt.Errorf("list mcp tool exposure rules: %w", err)
	}

	m.mu.Lock()
	m.servers = map[string]Server{}
	m.serverIDs = m.serverIDs[:0]
	m.states = map[string]ServerState{}
	m.tools = map[string]map[string]Tool{}
	m.exposure = map[string]map[string]map[string]ToolExposureRule{}
	m.sessions = map[string]*sessionState{}
	for _, record := range serverRecords {
		var server Server
		if err := json.Unmarshal(record.Document, &server); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("decode mcp server %s: %w", record.ServerID, err)
		}
		m.servers[server.ServerID] = cloneServer(server)
		m.serverIDs = append(m.serverIDs, server.ServerID)
	}
	for _, record := range stateRecords {
		var state ServerState
		if err := json.Unmarshal(record.Document, &state); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("decode mcp server state %s: %w", record.ServerID, err)
		}
		m.states[state.ServerID] = cloneServerState(state)
	}
	for _, record := range toolRecords {
		var tool Tool
		if err := json.Unmarshal(record.Document, &tool); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("decode mcp tool %s/%s: %w", record.ServerID, record.ToolName, err)
		}
		if m.tools[tool.ServerID] == nil {
			m.tools[tool.ServerID] = map[string]Tool{}
		}
		m.tools[tool.ServerID][tool.ToolName] = cloneTool(tool)
	}
	for _, record := range exposureRecords {
		var rule ToolExposureRule
		if err := json.Unmarshal(record.Document, &rule); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("decode mcp tool exposure rule %s/%s/%s: %w", record.ServerID, record.ToolName, record.RuntimeSurface, err)
		}
		if m.exposure[rule.ServerID] == nil {
			m.exposure[rule.ServerID] = map[string]map[string]ToolExposureRule{}
		}
		if m.exposure[rule.ServerID][rule.ToolName] == nil {
			m.exposure[rule.ServerID][rule.ToolName] = map[string]ToolExposureRule{}
		}
		m.exposure[rule.ServerID][rule.ToolName][rule.RuntimeSurface] = cloneToolExposureRule(rule)
	}
	for _, serverID := range m.serverIDs {
		if _, ok := m.states[serverID]; !ok {
			m.states[serverID] = defaultStateForServer(m.servers[serverID])
			continue
		}
		server := m.servers[serverID]
		state := m.states[serverID]
		if !server.Enabled {
			state.Status = LifecycleStatusDisabled
		} else if state.Status != LifecycleStatusStopped && state.Status != LifecycleStatusDisabled {
			state.Status = LifecycleStatusStopped
			state.HealthReason = "daemon restart cleared in-memory MCP session state"
			state.LastExecutionID = ""
		}
		state.UpdatedAt = time.Now().UTC()
		m.states[serverID] = state
	}
	for _, serverID := range m.serverIDs {
		_ = m.persistState(ctx, m.states[serverID])
	}
	for _, serverID := range m.serverIDs {
		if len(m.tools[serverID]) == 0 {
			continue
		}
		server := m.servers[serverID]
		if !server.Enabled || m.states[serverID].Status != LifecycleStatusHealthy {
			for toolName, tool := range m.tools[serverID] {
				if tool.DiscoveryStatus == DiscoveryStatusDiscovered {
					tool.DiscoveryStatus = DiscoveryStatusStale
					tool.UpdatedAt = time.Now().UTC()
					m.tools[serverID][toolName] = tool
				}
			}
		}
	}
	serverIDs := append([]string(nil), m.serverIDs...)
	m.mu.Unlock()

	for _, serverID := range serverIDs {
		m.mu.RLock()
		tools := cloneToolMap(m.tools[serverID])
		m.mu.RUnlock()
		if err := m.persistToolMap(ctx, serverID, tools); err != nil {
			return err
		}
	}

	for _, serverID := range serverIDs {
		server, ok := m.GetServer(serverID)
		if !ok || !server.Enabled {
			continue
		}
		if _, err := m.Start(ctx, serverID, "system.restore"); err != nil {
			_ = err
		}
	}
	return nil
}

func (m *Manager) ListServers() []ServerResource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]ServerResource, 0, len(m.serverIDs))
	for _, serverID := range m.serverIDs {
		server := m.servers[serverID]
		items = append(items, m.buildServerResourceLocked(server))
	}
	return items
}

func (m *Manager) ListServersForTenant(tenantID string) []ServerResource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantID = strings.TrimSpace(tenantID)
	items := make([]ServerResource, 0, len(m.serverIDs))
	for _, serverID := range m.serverIDs {
		server := m.servers[serverID]
		if tenantID == "" || server.TenantID == tenantID {
			items = append(items, m.buildServerResourceLocked(server))
		}
	}
	return items
}

func (m *Manager) GetServer(serverID string) (Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return Server{}, false
	}
	return cloneServer(server), true
}

func (m *Manager) GetServerForTenant(serverID, tenantID string) (Server, bool) {
	server, ok := m.GetServer(serverID)
	if !ok {
		return Server{}, false
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID != "" && server.TenantID != tenantID {
		return Server{}, false
	}
	return server, true
}

func (m *Manager) GetServerResource(serverID string) (ServerResource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return ServerResource{}, false
	}
	return m.buildServerResourceLocked(server), true
}

func (m *Manager) GetServerResourceForTenant(serverID, tenantID string) (ServerResource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return ServerResource{}, false
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID != "" && server.TenantID != tenantID {
		return ServerResource{}, false
	}
	return m.buildServerResourceLocked(server), true
}

func (m *Manager) ListTools(serverID string) ([]ToolResource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return nil, ErrServerNotFound
	}
	toolMap := m.tools[server.ServerID]
	items := make([]ToolResource, 0, len(toolMap))
	for _, tool := range toolMap {
		items = append(items, m.buildToolResourceLocked(server, tool))
	}
	return items, nil
}

func (m *Manager) ListToolsForTenant(serverID, tenantID string) ([]ToolResource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return nil, ErrServerNotFound
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID != "" && server.TenantID != tenantID {
		return nil, ErrServerNotFound
	}
	toolMap := m.tools[server.ServerID]
	items := make([]ToolResource, 0, len(toolMap))
	for _, tool := range toolMap {
		items = append(items, m.buildToolResourceLocked(server, tool))
	}
	return items, nil
}

func (m *Manager) CreateServer(ctx context.Context, input CreateServerInput) (ServerResource, bool, error) {
	server, created, err := m.upsertServer(ctx, input, nil)
	if err != nil {
		return ServerResource{}, false, err
	}
	return server, created, nil
}

func (m *Manager) UpdateServer(ctx context.Context, serverID string, input UpdateServerInput) (ServerResource, error) {
	server, _, err := m.upsertServer(ctx, CreateServerInput{}, &updateOperation{
		serverID: strings.TrimSpace(serverID),
		input:    input,
	})
	if err != nil {
		return ServerResource{}, err
	}
	return server, nil
}

func (m *Manager) UpdateToolExposure(ctx context.Context, serverID, toolName string, input UpdateExposureInput) (ToolResource, error) {
	serverID = strings.TrimSpace(serverID)
	toolName = strings.TrimSpace(toolName)
	if serverID == "" {
		return ToolResource{}, ErrServerIDRequired
	}
	if toolName == "" {
		return ToolResource{}, ErrToolNameRequired
	}
	if strings.TrimSpace(input.RuntimeSurface) == "" {
		return ToolResource{}, ErrRuntimeSurfaceRequired
	}

	now := time.Now().UTC()
	rule := ToolExposureRule{
		ServerID:       serverID,
		ToolName:       toolName,
		RuntimeSurface: strings.TrimSpace(input.RuntimeSurface),
		ExposureMode:   input.ExposureMode,
		Active:         input.Active,
		Reason:         strings.TrimSpace(input.Reason),
		UpdatedAt:      now,
	}

	m.mu.Lock()
	server, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return ToolResource{}, ErrServerNotFound
	}
	tool, ok := m.tools[serverID][toolName]
	if !ok {
		m.mu.Unlock()
		return ToolResource{}, ErrToolNameRequired
	}
	if m.exposure[serverID] == nil {
		m.exposure[serverID] = map[string]map[string]ToolExposureRule{}
	}
	if m.exposure[serverID][toolName] == nil {
		m.exposure[serverID][toolName] = map[string]ToolExposureRule{}
	}
	m.exposure[serverID][toolName][rule.RuntimeSurface] = cloneToolExposureRule(rule)
	resource := m.buildToolResourceLocked(server, tool)
	m.mu.Unlock()

	if err := m.persistExposureRule(ctx, rule); err != nil {
		return ToolResource{}, err
	}
	if err := m.publishEvent(ctx, "mcp", "mcp.tool_exposure_updated", events.Resource{Kind: resourceKindTool, ID: serverID + ":" + toolName}, map[string]any{
		"serverId":       serverID,
		"toolName":       toolName,
		"runtimeSurface": rule.RuntimeSurface,
		"exposureMode":   rule.ExposureMode,
		"active":         rule.Active,
		"reason":         rule.Reason,
	}); err != nil {
		return ToolResource{}, err
	}
	return resource, nil
}

func (m *Manager) AuthorizeTool(ctx context.Context, serverID, toolName string, input AuthorizeToolInput) (ToolAuthorizationResponse, error) {
	serverID = strings.TrimSpace(serverID)
	toolName = strings.TrimSpace(toolName)
	runtimeSurface := strings.TrimSpace(input.RuntimeSurface)
	if serverID == "" {
		return ToolAuthorizationResponse{}, ErrServerIDRequired
	}
	if toolName == "" {
		return ToolAuthorizationResponse{}, ErrToolNameRequired
	}
	if runtimeSurface == "" {
		return ToolAuthorizationResponse{}, ErrRuntimeSurfaceRequired
	}

	m.mu.RLock()
	server, ok := m.servers[serverID]
	if !ok {
		m.mu.RUnlock()
		return ToolAuthorizationResponse{}, ErrServerNotFound
	}
	tool, ok := m.tools[serverID][toolName]
	if !ok {
		m.mu.RUnlock()
		return ToolAuthorizationResponse{}, ErrToolNameRequired
	}
	active := m.sessions[serverID]
	rule, hasRule := m.exposure[serverID][toolName][runtimeSurface]
	resource := m.buildToolResourceLocked(server, tool)
	m.mu.RUnlock()

	if !hasRule || !rule.Active || rule.ExposureMode == ExposureModeBlocked {
		message := firstNonEmpty(resource.UnavailableReason, "tool is not allowlisted for this runtime surface")
		return ToolAuthorizationResponse{Status: ToolAuthorizationStatusBlocked, Tool: resource, Message: message}, nil
	}
	if resource.EffectiveAvailability != "available" {
		return ToolAuthorizationResponse{Status: ToolAuthorizationStatusBlocked, Tool: resource, Message: firstNonEmpty(resource.UnavailableReason, "tool is not currently available")}, nil
	}

	approvalMode := sandbox.ApprovalModeAllow
	if rule.ExposureMode == ExposureModeApprovalRequired {
		approvalMode = sandbox.ApprovalModeAsk
	}
	consumer, err := m.buildToolConsumerView(ctx, server, toolName, runtimeSurface, firstNonEmpty(strings.TrimSpace(input.RequestedBy), "mcp"), approvalMode)
	if err != nil {
		return ToolAuthorizationResponse{}, err
	}

	if rule.ExposureMode == ExposureModeAllow {
		if err := m.persistConsumerView(ctx, consumer); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		return ToolAuthorizationResponse{
			Status:    ToolAuthorizationStatusAllowed,
			Tool:      resource,
			SessionID: sessionID(active),
			Sandbox:   consumer,
			Message:   "tool use is allowed",
		}, nil
	}

	if m.policy == nil {
		return ToolAuthorizationResponse{}, errors.New("policy engine is not configured")
	}

	approvalResourceID := serverID + ":" + toolName + ":" + runtimeSurface
	requestedBy := firstNonEmpty(strings.TrimSpace(input.RequestedBy), "mcp")
	if strings.TrimSpace(input.ApprovalID) == "" {
		approval, decision, err := m.policy.RequestApproval(policy.RequestApprovalInput{
			Action:       "tool_call.execute",
			ResourceKind: resourceKindTool,
			ResourceID:   approvalResourceID,
			Reason:       "MCP tool execution requires approval",
			RequestedBy:  requestedBy,
		})
		if err != nil {
			return ToolAuthorizationResponse{}, err
		}
		approval.Sandbox = consumerViewMap(consumer)
		decision.Sandbox = consumerViewMap(consumer)
		consumer.PolicyRecord.ApprovalID = approval.ApprovalID
		consumer.PolicyRecord.DecisionID = decision.DecisionID
		consumer.PolicyRecord.Decision = sandbox.DecisionResolutionAsk
		consumer.PolicyRecord.ApprovalStatus = sandbox.DecisionApprovalStatusPending
		consumer.PolicyRecord.Status = sandbox.PolicyRecordStatusApprovalPending
		consumer.PolicyRecord.FailureClass = string(sandbox.ErrorClassApprovalRequired)
		if err := m.persistApproval(ctx, approval); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		if err := m.persistDecision(ctx, decision); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		if err := m.persistConsumerView(ctx, consumer); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		if err := m.publishEvent(ctx, "policy", "policy.approval_requested", events.Resource{Kind: "approval", ID: approval.ApprovalID}, map[string]any{
			"action":       approval.Action,
			"resourceKind": approval.ResourceKind,
			"resourceId":   approval.ResourceID,
			"status":       approval.Status,
			"sandbox":      approval.Sandbox,
		}); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		if err := m.publishEvent(ctx, "policy", "policy.decision_recorded", events.Resource{Kind: "decision", ID: decision.DecisionID}, map[string]any{
			"action":       decision.Action,
			"resourceKind": decision.ResourceKind,
			"resourceId":   decision.ResourceID,
			"outcome":      decision.Outcome,
			"approvalId":   decision.ApprovalID,
			"sandbox":      decision.Sandbox,
		}); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		return ToolAuthorizationResponse{
			Status:    ToolAuthorizationStatusPending,
			Tool:      resource,
			SessionID: sessionID(active),
			Message:   "tool use requires approval",
			Approval:  &approval,
			Decision:  &decision,
			Sandbox:   consumer,
		}, nil
	}

	approval, ok := m.policy.GetApproval(strings.TrimSpace(input.ApprovalID))
	if !ok {
		return ToolAuthorizationResponse{}, policy.ErrApprovalNotFound
	}
	if approval.Action != "tool_call.execute" || approval.ResourceKind != resourceKindTool || approval.ResourceID != approvalResourceID {
		return ToolAuthorizationResponse{}, ErrApprovalIDInvalid
	}
	consumer.PolicyRecord.ApprovalID = approval.ApprovalID
	switch approval.Status {
	case policy.ApprovalStatusApproved:
		consumer.PolicyRecord.Decision = sandbox.DecisionResolutionAllow
		consumer.PolicyRecord.ApprovalStatus = sandbox.DecisionApprovalStatusApproved
		consumer.PolicyRecord.Status = sandbox.PolicyRecordStatusPreflightAllowed
		consumer.PolicyRecord.FailureClass = ""
		if err := m.persistConsumerView(ctx, consumer); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		return ToolAuthorizationResponse{
			Status:    ToolAuthorizationStatusAllowed,
			Tool:      resource,
			SessionID: sessionID(active),
			Message:   "tool use is allowed by approval",
			Sandbox:   consumer,
		}, nil
	case policy.ApprovalStatusRejected:
		consumer.PolicyRecord.Decision = sandbox.DecisionResolutionDeny
		consumer.PolicyRecord.ApprovalStatus = sandbox.DecisionApprovalStatusRejected
		consumer.PolicyRecord.Status = sandbox.PolicyRecordStatusDenied
		consumer.PolicyRecord.FailureClass = string(sandbox.ErrorClassApprovalRejected)
		if err := m.persistConsumerView(ctx, consumer); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		return ToolAuthorizationResponse{
			Status:    ToolAuthorizationStatusRejected,
			Tool:      resource,
			SessionID: sessionID(active),
			Message:   "approval was rejected",
			Approval:  &approval,
			Sandbox:   consumer,
		}, nil
	default:
		consumer.PolicyRecord.Decision = sandbox.DecisionResolutionAsk
		consumer.PolicyRecord.ApprovalStatus = sandbox.DecisionApprovalStatusPending
		consumer.PolicyRecord.Status = sandbox.PolicyRecordStatusApprovalPending
		consumer.PolicyRecord.FailureClass = string(sandbox.ErrorClassApprovalRequired)
		if err := m.persistConsumerView(ctx, consumer); err != nil {
			return ToolAuthorizationResponse{}, err
		}
		return ToolAuthorizationResponse{
			Status:    ToolAuthorizationStatusPending,
			Tool:      resource,
			SessionID: sessionID(active),
			Message:   "approval is still pending",
			Approval:  &approval,
			Sandbox:   consumer,
		}, nil
	}
}

func (m *Manager) InstallCatalogEntry(ctx context.Context, entryID string, input CatalogInstallInput, method InstallMethod) (CatalogInstallResult, error) {
	entry, ok := m.GetCatalogEntry(entryID)
	if !ok {
		return CatalogInstallResult{}, ErrServerNotFound
	}
	installID := fmt.Sprintf("mcp_install_%d", time.Now().UTC().UnixNano())
	requestedPayload := map[string]any{
		"installId":      installID,
		"catalogEntryId": entry.ID,
		"method":         method,
		"environment":    string(m.cfg.Environment),
	}
	requestedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_install_requested", events.Resource{Kind: "mcp_catalog_install", ID: installID}, requestedPayload)
	if err != nil {
		return CatalogInstallResult{}, err
	}
	createInput := mergeCatalogInstallInput(entry, input, method, m.cfg.Environment)
	installAvailability, installReason := evaluateCatalogInstallSpecAvailability(m.cfg, createInput, entry.SecretRequirements)
	if installAvailability != AvailabilityStatusReady {
		result := CatalogInstallResult{
			InstallID:          installID,
			Status:             "blocked",
			CatalogEntryID:     entry.ID,
			ServerID:           createInput.ServerID,
			AvailabilityStatus: installAvailability,
			AvailabilityReason: installReason,
			AuditEventIDs:      []string{requestedEvent.EventID},
		}
		failedEvent, failedErr := m.publishAuditEvent(ctx, "mcp.catalog_install_failed", events.Resource{Kind: "mcp_catalog_install", ID: installID}, map[string]any{
			"installId":          installID,
			"catalogEntryId":     entry.ID,
			"method":             method,
			"status":             result.Status,
			"availabilityStatus": result.AvailabilityStatus,
			"availabilityReason": result.AvailabilityReason,
		})
		if failedErr == nil {
			result.AuditEventIDs = append(result.AuditEventIDs, failedEvent.EventID)
		}
		return result, nil
	}
	if existing, ok := m.GetServer(createInput.ServerID); ok {
		if reason, blocked := catalogInstallConflictReason(existing, entry.ID); blocked {
			result := CatalogInstallResult{
				InstallID:          installID,
				Status:             "blocked",
				CatalogEntryID:     entry.ID,
				ServerID:           existing.ServerID,
				AvailabilityStatus: AvailabilityStatusBlocked,
				AvailabilityReason: reason,
				AuditEventIDs:      []string{requestedEvent.EventID},
			}
			failedEvent, failedErr := m.publishAuditEvent(ctx, "mcp.catalog_install_failed", events.Resource{Kind: "mcp_catalog_install", ID: installID}, map[string]any{
				"installId":          installID,
				"catalogEntryId":     entry.ID,
				"serverId":           existing.ServerID,
				"method":             method,
				"status":             result.Status,
				"availabilityStatus": result.AvailabilityStatus,
				"availabilityReason": result.AvailabilityReason,
			})
			if failedErr == nil {
				result.AuditEventIDs = append(result.AuditEventIDs, failedEvent.EventID)
			}
			return result, nil
		}
	}
	createInput.CatalogManagement = catalogManagementForCreate(entry, createInput, nil, CatalogActionInstall, time.Now().UTC())
	resource, _, err := m.CreateServer(ctx, createInput)
	if err != nil {
		_, _ = m.publishAuditEvent(ctx, "mcp.catalog_install_failed", events.Resource{Kind: "mcp_catalog_install", ID: installID}, map[string]any{
			"installId":      installID,
			"catalogEntryId": entry.ID,
			"method":         method,
			"status":         "failed",
			"reason":         err.Error(),
		})
		return CatalogInstallResult{}, err
	}
	result := CatalogInstallResult{
		InstallID:          installID,
		Status:             "installed",
		CatalogEntryID:     entry.ID,
		ServerID:           resource.ServerID,
		AvailabilityStatus: resource.AvailabilityStatus,
		AvailabilityReason: resource.AvailabilityReason,
		AuditEventIDs:      []string{requestedEvent.EventID},
		Server:             &resource,
	}
	completedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_install_completed", events.Resource{Kind: "mcp_catalog_install", ID: installID}, map[string]any{
		"installId":          installID,
		"catalogEntryId":     entry.ID,
		"serverId":           resource.ServerID,
		"method":             method,
		"status":             result.Status,
		"availabilityStatus": result.AvailabilityStatus,
		"availabilityReason": result.AvailabilityReason,
	})
	if err == nil {
		result.AuditEventIDs = append(result.AuditEventIDs, completedEvent.EventID)
	}
	return result, nil
}

func (m *Manager) RefreshCatalogServer(ctx context.Context, serverID string) (CatalogLifecycleResult, error) {
	return m.runCatalogLifecycleAction(ctx, strings.TrimSpace(serverID), CatalogActionRefresh)
}

func (m *Manager) ReinstallCatalogServer(ctx context.Context, serverID string) (CatalogLifecycleResult, error) {
	return m.runCatalogLifecycleAction(ctx, strings.TrimSpace(serverID), CatalogActionReinstall)
}

func (m *Manager) UninstallCatalogServer(ctx context.Context, serverID string) (CatalogLifecycleResult, error) {
	return m.runCatalogLifecycleAction(ctx, strings.TrimSpace(serverID), CatalogActionUninstall)
}

func (m *Manager) RevalidateCatalogServer(ctx context.Context, serverID string) (CatalogRevalidationResult, error) {
	startedAt := time.Now()
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return CatalogRevalidationResult{}, ErrServerIDRequired
	}
	server, ok := m.GetServer(serverID)
	if !ok {
		return CatalogRevalidationResult{}, ErrServerNotFound
	}
	result := CatalogRevalidationResult{
		ActionID:       fmt.Sprintf("mcp_revalidate_%d", startedAt.UTC().UnixNano()),
		Action:         CatalogActionRevalidate,
		ServerID:       server.ServerID,
		CatalogEntryID: server.CatalogEntryID,
	}
	requestedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_lifecycle_requested", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         result.Action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"environment":    string(m.cfg.Environment),
	})
	if err != nil {
		return CatalogRevalidationResult{}, err
	}
	result.AuditEventIDs = append(result.AuditEventIDs, requestedEvent.EventID)

	if blocked, handled := m.catalogTargetBlockResult(server); handled {
		return m.catalogRevalidationBlockedResult(ctx, server, result, blocked, startedAt), nil
	}
	if blocked, handled, err := m.catalogRevalidationBusyBlockResult(ctx, server); err != nil {
		return CatalogRevalidationResult{}, err
	} else if handled {
		return m.catalogRevalidationBlockedResult(ctx, server, result, blocked, startedAt), nil
	}

	management := m.buildCatalogManagementLocked(server)
	issues, status, classification, reason := m.collectRevalidationIssues(server, management)
	checkedAt := time.Now().UTC()
	server.CatalogManagement = management
	if server.CatalogManagement == nil {
		server.CatalogManagement = &CatalogManagement{}
	}
	server.CatalogManagement.LastAction = CatalogActionRevalidate
	server.CatalogManagement.LastActionStatus = CatalogActionStatusCompleted
	server.CatalogManagement.LastActionFailureClass = ""
	server.CatalogManagement.LastActionReason = reason
	server.CatalogManagement.LastActionAt = &checkedAt
	server.CatalogManagement.LastRevalidation = &RevalidationSnapshot{
		CheckedAt:      checkedAt,
		Status:         status,
		Classification: classification,
		Reason:         reason,
		Issues:         append([]RevalidationIssue(nil), issues...),
	}
	m.setServer(server)
	if err := m.persistServer(ctx, server); err != nil {
		return CatalogRevalidationResult{}, err
	}

	result.Status = status
	result.Classification = classification
	result.Reason = reason
	result.Issues = append([]RevalidationIssue(nil), issues...)
	result.PreflightMs = time.Since(startedAt).Milliseconds()
	if resource, ok := m.GetServerResource(server.ServerID); ok {
		result.Server = &resource
	}
	completedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_revalidation_completed", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         result.Action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"status":         result.Status,
		"classification": result.Classification,
		"reason":         result.Reason,
		"issues":         redactedIssues(issues),
		"environment":    string(m.cfg.Environment),
	})
	if err != nil {
		return CatalogRevalidationResult{}, err
	}
	result.AuditEventIDs = append(result.AuditEventIDs, completedEvent.EventID)
	return result, nil
}

func (m *Manager) runCatalogLifecycleAction(ctx context.Context, serverID string, action CatalogAction) (CatalogLifecycleResult, error) {
	startedAt := time.Now()
	if serverID == "" {
		return CatalogLifecycleResult{}, ErrServerIDRequired
	}
	server, ok := m.GetServer(serverID)
	if !ok {
		return CatalogLifecycleResult{}, ErrServerNotFound
	}
	result := CatalogLifecycleResult{
		ActionID:       fmt.Sprintf("mcp_catalog_%s_%d", action, startedAt.UTC().UnixNano()),
		Action:         action,
		ServerID:       server.ServerID,
		CatalogEntryID: server.CatalogEntryID,
	}
	requestedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_lifecycle_requested", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"environment":    string(m.cfg.Environment),
	})
	if err != nil {
		return CatalogLifecycleResult{}, err
	}
	result.AuditEventIDs = append(result.AuditEventIDs, requestedEvent.EventID)
	if blocked, handled, err := m.catalogLifecycleBlockResult(ctx, server, action, action != CatalogActionUninstall); err != nil {
		return CatalogLifecycleResult{}, err
	} else if handled {
		return m.catalogLifecycleBlockedResult(ctx, server, result, blocked, startedAt)
	}

	switch action {
	case CatalogActionUninstall:
		if err := m.deleteCatalogServer(ctx, server.ServerID); err != nil {
			return m.catalogLifecycleFailedResult(ctx, server, result, "failed", err.Error(), startedAt)
		}
		result.Status = CatalogActionStatusCompleted
		result.Removed = true
	case CatalogActionRefresh, CatalogActionReinstall:
		entry, ok := m.GetCatalogEntry(server.CatalogEntryID)
		if !ok {
			return m.catalogLifecycleBlockedResult(ctx, server, result, CatalogLifecycleResult{
				Status:       CatalogActionStatusBlocked,
				FailureClass: "missing_entry",
				Reason:       "catalog entry is no longer available",
			}, startedAt)
		}
		createInput := mergeCatalogInstallInput(entry, catalogInstallInputFromSnapshot(server.CatalogManagement.InstallInputSnapshot), server.InstallMethod, m.cfg.Environment)
		createInput.CatalogManagement = catalogManagementForCreate(entry, createInput, &server, action, time.Now().UTC())
		previousInput := serverToCreateInput(server)
		if action == CatalogActionReinstall {
			if err := m.deleteCatalogServer(ctx, server.ServerID); err != nil {
				return m.catalogLifecycleFailedResult(ctx, server, result, "failed", err.Error(), startedAt)
			}
		}
		resource, _, err := m.CreateServer(ctx, createInput)
		if err != nil {
			if action == CatalogActionReinstall {
				_, _, _ = m.CreateServer(ctx, previousInput)
			}
			return m.catalogLifecycleFailedResult(ctx, server, result, "failed", err.Error(), startedAt)
		}
		result.Status = CatalogActionStatusCompleted
		result.Server = &resource
	default:
		return CatalogLifecycleResult{}, fmt.Errorf("unsupported catalog action %s", action)
	}
	result.PreflightMs = time.Since(startedAt).Milliseconds()
	completedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_lifecycle_completed", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"status":         result.Status,
		"removed":        result.Removed,
		"environment":    string(m.cfg.Environment),
	})
	if err != nil {
		return CatalogLifecycleResult{}, err
	}
	result.AuditEventIDs = append(result.AuditEventIDs, completedEvent.EventID)
	return result, nil
}

func (m *Manager) catalogLifecycleBlockResult(ctx context.Context, server Server, action CatalogAction, failOnModified bool) (CatalogLifecycleResult, bool, error) {
	if blocked, handled := m.catalogTargetBlockResult(server); handled {
		return blocked, true, nil
	}
	m.mu.RLock()
	activeSession := m.sessions[server.ServerID] != nil
	state := m.states[server.ServerID]
	m.mu.RUnlock()
	if activeSession || state.Status == LifecycleStatusStarting || state.Status == LifecycleStatusStopping || state.Status == LifecycleStatusBackingOff {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "busy",
			Reason:       "server has an active lifecycle or transport session",
		}, true, nil
	}
	if m.store != nil {
		activeToolCalls, err := m.store.HasActiveMCPToolCalls(ctx, server.ServerID)
		if err != nil {
			return CatalogLifecycleResult{}, false, err
		}
		if activeToolCalls {
			return CatalogLifecycleResult{
				Status:       CatalogActionStatusBlocked,
				FailureClass: "busy",
				Reason:       "server has an active tool invocation",
			}, true, nil
		}
	}
	if failOnModified && server.OperatorModified {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "conflict",
			Reason:       "server has local operator modifications",
		}, true, nil
	}
	if (action == CatalogActionRefresh || action == CatalogActionReinstall) && server.CatalogManagement == nil {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "conflict",
			Reason:       "server is missing catalog install snapshot metadata",
		}, true, nil
	}
	return CatalogLifecycleResult{}, false, nil
}

func (m *Manager) catalogTargetBlockResult(server Server) (CatalogLifecycleResult, bool) {
	if server.OriginKind != OriginKindCatalog {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "not_catalog_managed",
			Reason:       "server is not catalog-managed",
		}, true
	}
	if scope := strings.TrimSpace(server.EnvironmentScope); scope != "" && scope != string(m.cfg.Environment) {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "environment_mismatch",
			Reason:       fmt.Sprintf("server belongs to %s environment", scope),
		}, true
	}
	return CatalogLifecycleResult{}, false
}

func (m *Manager) catalogRevalidationBusyBlockResult(ctx context.Context, server Server) (CatalogLifecycleResult, bool, error) {
	if m.store == nil {
		return CatalogLifecycleResult{}, false, nil
	}
	activeToolCalls, err := m.store.HasActiveMCPToolCalls(ctx, server.ServerID)
	if err != nil {
		return CatalogLifecycleResult{}, false, err
	}
	if activeToolCalls {
		return CatalogLifecycleResult{
			Status:       CatalogActionStatusBlocked,
			FailureClass: "busy",
			Reason:       "server has an active tool invocation",
		}, true, nil
	}
	return CatalogLifecycleResult{}, false, nil
}

func (m *Manager) catalogLifecycleBlockedResult(ctx context.Context, server Server, result CatalogLifecycleResult, blocked CatalogLifecycleResult, startedAt time.Time) (CatalogLifecycleResult, error) {
	result.Status = blocked.Status
	result.FailureClass = blocked.FailureClass
	result.Reason = blocked.Reason
	result.PreflightMs = time.Since(startedAt).Milliseconds()
	if err := m.persistCatalogActionOutcome(ctx, server, result.Action, result.Status, result.FailureClass, result.Reason); err != nil {
		return CatalogLifecycleResult{}, err
	}
	failedEvent, err := m.publishAuditEvent(ctx, "mcp.catalog_lifecycle_failed", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         result.Action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"status":         result.Status,
		"failureClass":   result.FailureClass,
		"reason":         result.Reason,
		"environment":    string(m.cfg.Environment),
	})
	if err != nil {
		return CatalogLifecycleResult{}, err
	}
	result.AuditEventIDs = append(result.AuditEventIDs, failedEvent.EventID)
	if resource, ok := m.GetServerResource(server.ServerID); ok {
		result.Server = &resource
	}
	return result, nil
}

func (m *Manager) catalogLifecycleFailedResult(ctx context.Context, server Server, result CatalogLifecycleResult, failureClass, reason string, startedAt time.Time) (CatalogLifecycleResult, error) {
	return m.catalogLifecycleBlockedResult(ctx, server, result, CatalogLifecycleResult{
		Status:       CatalogActionStatusFailed,
		FailureClass: failureClass,
		Reason:       reason,
	}, startedAt)
}

func (m *Manager) catalogRevalidationBlockedResult(ctx context.Context, server Server, result CatalogRevalidationResult, blocked CatalogLifecycleResult, startedAt time.Time) CatalogRevalidationResult {
	result.Status = AvailabilityStatusBlocked
	result.Classification = RevalidationClassificationPrerequisiteLost
	result.Reason = blocked.Reason
	result.Issues = []RevalidationIssue{{
		Kind:             "configuration",
		Name:             blocked.FailureClass,
		Status:           RevalidationIssueStatusBlocked,
		Reason:           blocked.Reason,
		EnvironmentScope: string(m.cfg.Environment),
	}}
	result.PreflightMs = time.Since(startedAt).Milliseconds()
	_ = m.persistCatalogActionOutcome(ctx, server, CatalogActionRevalidate, CatalogActionStatusBlocked, blocked.FailureClass, blocked.Reason)
	if event, err := m.publishAuditEvent(ctx, "mcp.catalog_revalidation_completed", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"actionId":       result.ActionID,
		"action":         result.Action,
		"serverId":       server.ServerID,
		"catalogEntryId": server.CatalogEntryID,
		"status":         result.Status,
		"classification": result.Classification,
		"reason":         result.Reason,
		"issues":         redactedIssues(result.Issues),
		"environment":    string(m.cfg.Environment),
	}); err == nil {
		result.AuditEventIDs = append(result.AuditEventIDs, event.EventID)
	}
	if resource, ok := m.GetServerResource(server.ServerID); ok {
		result.Server = &resource
	}
	return result
}

func (m *Manager) persistCatalogActionOutcome(ctx context.Context, server Server, action CatalogAction, status CatalogActionStatus, failureClass, reason string) error {
	now := time.Now().UTC()
	server.CatalogManagement = m.buildCatalogManagementLocked(server)
	if server.CatalogManagement == nil {
		server.CatalogManagement = &CatalogManagement{}
	}
	server.CatalogManagement.LastAction = action
	server.CatalogManagement.LastActionStatus = status
	server.CatalogManagement.LastActionFailureClass = strings.TrimSpace(failureClass)
	server.CatalogManagement.LastActionReason = strings.TrimSpace(reason)
	server.CatalogManagement.LastActionAt = &now
	m.setServer(server)
	return m.persistServer(ctx, server)
}

func (m *Manager) deleteCatalogServer(ctx context.Context, serverID string) error {
	if m.store != nil {
		if err := m.store.DeleteMCPServer(ctx, serverID); err != nil {
			return err
		}
	}
	m.mu.Lock()
	delete(m.servers, serverID)
	delete(m.states, serverID)
	delete(m.tools, serverID)
	delete(m.exposure, serverID)
	delete(m.sessions, serverID)
	filtered := m.serverIDs[:0]
	for _, item := range m.serverIDs {
		if item != serverID {
			filtered = append(filtered, item)
		}
	}
	m.serverIDs = filtered
	m.mu.Unlock()
	return nil
}

func (m *Manager) setServer(server Server) {
	m.mu.Lock()
	if _, ok := m.servers[server.ServerID]; !ok {
		m.serverIDs = append(m.serverIDs, server.ServerID)
	}
	m.servers[server.ServerID] = cloneServer(server)
	m.mu.Unlock()
}

func catalogManagementForCreate(entry CatalogEntry, createInput CreateServerInput, previous *Server, action CatalogAction, now time.Time) *CatalogManagement {
	management := &CatalogManagement{
		SourceKind:           entry.SourceKind,
		InstalledRevision:    fingerprintCreateServerSpec(createInput),
		CurrentRevision:      fingerprintCreateServerSpec(createInput),
		InstallInputSnapshot: installSnapshotFromCreateSpec(createInput),
		LastAction:           action,
		LastActionStatus:     CatalogActionStatusCompleted,
		LastActionAt:         &now,
	}
	if previous != nil && previous.CatalogManagement != nil {
		management.InstalledAt = previous.CatalogManagement.InstalledAt
	}
	if management.InstalledAt == nil {
		management.InstalledAt = &now
	}
	if action == CatalogActionRefresh || action == CatalogActionReinstall {
		management.LastMaintainedAt = &now
	}
	return management
}

func (m *Manager) collectRevalidationIssues(server Server, management *CatalogManagement) ([]RevalidationIssue, AvailabilityStatus, RevalidationClassification, string) {
	issues := make([]RevalidationIssue, 0)
	entry, ok := m.GetCatalogEntry(server.CatalogEntryID)
	if !ok {
		issues = append(issues, RevalidationIssue{
			Kind:             "catalog",
			Name:             server.CatalogEntryID,
			Status:           RevalidationIssueStatusUnavailable,
			Reason:           "catalog entry is no longer available",
			EnvironmentScope: string(m.cfg.Environment),
		})
		return issues, AvailabilityStatusUnavailable, RevalidationClassificationMissingEntry, issues[0].Reason
	}
	if management == nil {
		management = m.buildCatalogManagementLocked(server)
	}
	spec, _ := m.catalogSpecForServer(server, entry, true)
	if spec.TransportKind == TransportKindStdio {
		if strings.TrimSpace(spec.Command) == "" {
			issues = append(issues, RevalidationIssue{Kind: "binary", Name: "command", Status: RevalidationIssueStatusUnavailable, Reason: "stdio command is not configured", EnvironmentScope: string(m.cfg.Environment)})
		} else if _, err := exec.LookPath(strings.TrimSpace(spec.Command)); err != nil {
			issues = append(issues, RevalidationIssue{Kind: "binary", Name: strings.TrimSpace(spec.Command), Status: RevalidationIssueStatusUnavailable, Reason: "required binary is unavailable", EnvironmentScope: string(m.cfg.Environment)})
		}
		if requiresOfflineVerifiedLocalCommand(spec) {
			issues = append(issues, RevalidationIssue{Kind: "configuration", Name: "command", Status: RevalidationIssueStatusUnavailable, Reason: "default bundled stdio command requires a local command override because sandbox network is denied", EnvironmentScope: string(m.cfg.Environment)})
		}
	}
	if spec.TransportKind == TransportKindStreamableHTTP && strings.TrimSpace(spec.Endpoint) == "" {
		issues = append(issues, RevalidationIssue{Kind: "endpoint", Name: "streamable-http", Status: RevalidationIssueStatusUnsupported, Reason: "streamable-http endpoint is not configured", EnvironmentScope: string(m.cfg.Environment)})
	}
	resolved, _ := m.resolveSecretValues(context.Background(), secretRefsFromRequirements(entry.SecretRequirements))
	for _, requirement := range entry.SecretRequirements {
		if requirement.Required {
			if _, ok := resolved[requirement.SecretRef]; !ok {
				issues = append(issues, RevalidationIssue{Kind: "secret", Name: requirement.SecretRef, Status: RevalidationIssueStatusBlocked, Reason: firstNonEmpty(requirement.Description, fmt.Sprintf("%s is required", requirement.SecretRef)), EnvironmentScope: string(m.cfg.Environment)})
			}
		}
	}
	if management != nil {
		switch management.DriftStatus {
		case CatalogDriftStatusLocallyModified:
			issues = append(issues, RevalidationIssue{Kind: "catalog", Name: server.CatalogEntryID, Status: RevalidationIssueStatusWarning, Reason: firstNonEmpty(management.DriftReason, "server has local operator modifications"), EnvironmentScope: string(m.cfg.Environment)})
		case CatalogDriftStatusCatalogUpdated:
			issues = append(issues, RevalidationIssue{Kind: "catalog", Name: server.CatalogEntryID, Status: RevalidationIssueStatusWarning, Reason: firstNonEmpty(management.DriftReason, "installed server no longer matches current catalog revision"), EnvironmentScope: string(m.cfg.Environment)})
		}
	}
	state := m.states[server.ServerID]
	switch state.Status {
	case LifecycleStatusFailed, LifecycleStatusDenied, LifecycleStatusDegraded, LifecycleStatusUnsupported:
		status := RevalidationIssueStatusUnavailable
		if state.Status == LifecycleStatusUnsupported {
			status = RevalidationIssueStatusUnsupported
		}
		issues = append(issues, RevalidationIssue{Kind: "runtime", Name: string(state.Status), Status: status, Reason: firstNonEmpty(state.HealthReason, "server is not healthy"), EnvironmentScope: string(m.cfg.Environment)})
	case LifecycleStatusDisabled:
		issues = append(issues, RevalidationIssue{Kind: "runtime", Name: string(state.Status), Status: RevalidationIssueStatusBlocked, Reason: "server is disabled", EnvironmentScope: string(m.cfg.Environment)})
	}

	classification := RevalidationClassificationHealthy
	status := AvailabilityStatusReady
	reason := ""
	for _, issue := range issues {
		if reason == "" {
			reason = issue.Reason
		}
		switch issue.Status {
		case RevalidationIssueStatusUnsupported:
			status = AvailabilityStatusUnsupported
		case RevalidationIssueStatusBlocked:
			if status != AvailabilityStatusUnsupported {
				status = AvailabilityStatusBlocked
			}
		case RevalidationIssueStatusUnavailable:
			if status == AvailabilityStatusReady {
				status = AvailabilityStatusUnavailable
			}
		}
		switch issue.Kind {
		case "secret", "binary", "endpoint", "configuration":
			if classification == RevalidationClassificationHealthy {
				classification = RevalidationClassificationPrerequisiteLost
			}
		case "runtime":
			if classification == RevalidationClassificationHealthy {
				classification = RevalidationClassificationRuntimeUnhealthy
			}
		}
	}
	if classification == RevalidationClassificationHealthy && management != nil {
		switch management.DriftStatus {
		case CatalogDriftStatusLocallyModified:
			classification = RevalidationClassificationLocallyModified
			reason = firstNonEmpty(reason, management.DriftReason)
		case CatalogDriftStatusCatalogUpdated:
			classification = RevalidationClassificationCatalogDrift
			reason = firstNonEmpty(reason, management.DriftReason)
		}
	}
	return issues, status, classification, reason
}

func redactedIssues(issues []RevalidationIssue) []map[string]any {
	items := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		items = append(items, map[string]any{
			"kind":             issue.Kind,
			"name":             issue.Name,
			"status":           issue.Status,
			"reason":           issue.Reason,
			"environmentScope": issue.EnvironmentScope,
		})
	}
	return items
}

func catalogManagementPayload(management *CatalogManagement) map[string]any {
	if management == nil {
		return nil
	}
	payload := map[string]any{
		"sourceKind":        management.SourceKind,
		"installedRevision": management.InstalledRevision,
		"currentRevision":   management.CurrentRevision,
		"driftStatus":       management.DriftStatus,
		"driftReason":       management.DriftReason,
	}
	if management.InstalledAt != nil {
		payload["installedAt"] = management.InstalledAt.UTC().Format(time.RFC3339Nano)
	}
	if management.LastMaintainedAt != nil {
		payload["lastMaintainedAt"] = management.LastMaintainedAt.UTC().Format(time.RFC3339Nano)
	}
	if management.LastActionAt != nil {
		payload["lastActionAt"] = management.LastActionAt.UTC().Format(time.RFC3339Nano)
	}
	if management.LastAction != "" {
		payload["lastAction"] = management.LastAction
	}
	if management.LastActionStatus != "" {
		payload["lastActionStatus"] = management.LastActionStatus
	}
	if management.LastActionFailureClass != "" {
		payload["lastActionFailureClass"] = management.LastActionFailureClass
	}
	if management.LastActionReason != "" {
		payload["lastActionReason"] = management.LastActionReason
	}
	if management.LastRevalidation != nil {
		payload["lastRevalidation"] = map[string]any{
			"checkedAt":      management.LastRevalidation.CheckedAt.UTC().Format(time.RFC3339Nano),
			"status":         management.LastRevalidation.Status,
			"classification": management.LastRevalidation.Classification,
			"reason":         management.LastRevalidation.Reason,
			"issues":         redactedIssues(management.LastRevalidation.Issues),
		}
	}
	return payload
}

func (m *Manager) CallTool(ctx context.Context, serverID, toolName string, input any, authorization ToolAuthorizationResponse) (ToolInvocationResult, error) {
	serverID = strings.TrimSpace(serverID)
	toolName = strings.TrimSpace(toolName)
	if serverID == "" {
		return ToolInvocationResult{FailureClass: "blocked", Error: ErrServerIDRequired.Error()}, ErrServerIDRequired
	}
	if toolName == "" {
		return ToolInvocationResult{FailureClass: "blocked", Error: ErrToolNameRequired.Error()}, ErrToolNameRequired
	}
	if authorization.Status != ToolAuthorizationStatusAllowed {
		return ToolInvocationResult{
			FailureClass: "blocked",
			Error:        firstNonEmpty(authorization.Message, "tool use is not allowed"),
		}, nil
	}

	m.mu.RLock()
	active := m.sessions[serverID]
	server, ok := m.servers[serverID]
	m.mu.RUnlock()
	if !ok {
		return ToolInvocationResult{FailureClass: "server_unhealthy", Error: ErrServerNotFound.Error()}, ErrServerNotFound
	}
	if active == nil || active.session == nil {
		return ToolInvocationResult{FailureClass: "server_unhealthy", Error: "mcp server is not healthy"}, nil
	}
	output, err := active.session.CallTool(ctx, toolName, input)
	if err != nil {
		return ToolInvocationResult{
			SessionID:    active.sessionID,
			FailureClass: "transport_failed",
			Error:        err.Error(),
		}, nil
	}
	redacted := m.redactValue(ctx, server, output)
	if flag, ok := output["isError"].(bool); ok && flag {
		return ToolInvocationResult{
			SessionID:    active.sessionID,
			Output:       redacted,
			FailureClass: "remote_tool_error",
			Error:        firstNonEmpty(stringFromMap(output, "message"), "remote MCP tool returned an error"),
		}, nil
	}
	return ToolInvocationResult{
		SessionID: active.sessionID,
		Output:    redacted,
	}, nil
}

func (m *Manager) Start(ctx context.Context, serverID, requestedBy string) (LifecycleResponse, error) {
	startedAt := time.Now()
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return LifecycleResponse{}, ErrServerIDRequired
	}
	if m.transport == nil {
		return LifecycleResponse{}, ErrTransportNotConfigured
	}
	restoreRequest := isRestoreLifecycleRequester(requestedBy)
	reconnectRequest := isWebsocketReconnectRequester(requestedBy)

	m.mu.Lock()
	server, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return LifecycleResponse{}, ErrServerNotFound
	}
	if active := m.sessions[serverID]; active != nil {
		resource := m.buildServerResourceLocked(server)
		m.mu.Unlock()
		return LifecycleResponse{
			Action:      LifecycleActionStart,
			Server:      resource,
			Idempotent:  true,
			ExecutionID: active.executionID,
			PreflightMs: time.Since(startedAt).Milliseconds(),
		}, nil
	}
	state := m.states[serverID]
	if !server.Enabled {
		state.Status = LifecycleStatusDisabled
		state.UpdatedAt = time.Now().UTC()
		m.states[serverID] = state
		resource := m.buildServerResourceLocked(server)
		m.mu.Unlock()
		_ = m.persistState(ctx, state)
		return LifecycleResponse{
			Action:        LifecycleActionStart,
			Server:        resource,
			Idempotent:    true,
			Blocked:       true,
			BlockedReason: "server is disabled",
			PreflightMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
	state.Status = LifecycleStatusStarting
	state.HealthReason = ""
	state.NextReconnectAt = nil
	if restoreRequest {
		restoreStartedAt := time.Now().UTC()
		state.LastRecoveryAt = &restoreStartedAt
		state.LastRecoveryClass = "restore_requested"
	} else if !reconnectRequest {
		state.LastRecoveryAt = nil
		state.LastRecoveryClass = ""
	}
	state.UpdatedAt = time.Now().UTC()
	m.states[serverID] = state
	resource := m.buildServerResourceLocked(server)
	m.mu.Unlock()
	if err := m.persistState(ctx, state); err != nil {
		return LifecycleResponse{}, err
	}

	consumer, err := m.buildLifecycleConsumerView(ctx, server, firstNonEmpty(strings.TrimSpace(requestedBy), "mcp"))
	if err != nil {
		if restoreRequest {
			state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
		} else {
			state = m.recordFailure(ctx, serverID, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
		}
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:        LifecycleActionStart,
			Server:        resource,
			FailureClass:  "invalid_configuration",
			Blocked:       true,
			BlockedReason: err.Error(),
			PreflightMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
	var (
		execution   sandbox.Execution
		executionID string
		pipes       SessionPipes
	)
	transportServer := cloneServer(server)
	if server.TransportKind == TransportKindStdio {
		if m.sandboxes == nil {
			return LifecycleResponse{}, ErrSandboxManagerMissing
		}
		request, err := m.buildExecutionRequest(ctx, server, consumer, "")
		if err != nil {
			if restoreRequest {
				state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
			} else {
				state = m.recordFailure(ctx, serverID, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
			}
			resource, _ = m.GetServerResource(serverID)
			return LifecycleResponse{
				Action:        LifecycleActionStart,
				Server:        resource,
				FailureClass:  "invalid_configuration",
				Blocked:       true,
				BlockedReason: err.Error(),
				PreflightMs:   time.Since(startedAt).Milliseconds(),
			}, nil
		}
		attachedExecution, attached, err := m.sandboxes.StartAttachedExecution(ctx, request)
		if err != nil {
			if restoreRequest {
				state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusFailed, err.Error(), "launch_failed")
			} else {
				state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "launch_failed")
			}
			resource, _ = m.GetServerResource(serverID)
			return LifecycleResponse{
				Action:       LifecycleActionStart,
				Server:       resource,
				FailureClass: "launch_failed",
				PreflightMs:  time.Since(startedAt).Milliseconds(),
			}, nil
		}
		execution = attachedExecution
		executionID = attachedExecution.ExecutionID
		if attached == nil {
			state = m.updateStateFromExecution(ctx, serverID, state, execution, false)
			resource, _ = m.GetServerResource(serverID)
			return LifecycleResponse{
				Action:        LifecycleActionStart,
				Server:        resource,
				ExecutionID:   execution.ExecutionID,
				FailureClass:  classifyExecutionFailure(execution),
				Blocked:       true,
				BlockedReason: firstNonEmpty(execution.Result.Error, execution.Decision.Explanation, state.HealthReason),
				PreflightMs:   time.Since(startedAt).Milliseconds(),
			}, nil
		}
		pipes = SessionPipes{
			Stdin:  attached.Stdin,
			Stdout: attached.Stdout,
			Stderr: attached.Stderr,
		}
	}
	if server.TransportKind == TransportKindWebsocket {
		headers, err := m.resolveWebsocketHeaders(ctx, server)
		if err != nil {
			if restoreRequest {
				state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
			} else {
				state = m.recordFailure(ctx, serverID, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
			}
			resource, _ = m.GetServerResource(serverID)
			return LifecycleResponse{
				Action:        LifecycleActionStart,
				Server:        resource,
				FailureClass:  "invalid_configuration",
				Blocked:       true,
				BlockedReason: err.Error(),
				PreflightMs:   time.Since(startedAt).Milliseconds(),
			}, nil
		}
		transportServer.ResolvedWebsocketHeaders = headers
	}

	openCtx, cancelOpen := context.WithTimeout(ctx, mcpSessionStartTimeout)
	session, err := m.transport.Open(openCtx, transportServer, pipes)
	cancelOpen()
	if err != nil {
		if executionID != "" && m.sandboxes != nil {
			_, _, _ = m.sandboxes.CancelExecution(executionID)
		}
		if restoreRequest {
			state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		} else {
			state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		}
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:       LifecycleActionStart,
			Server:       resource,
			ExecutionID:  executionID,
			FailureClass: "transport_runtime_failure",
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}

	listCtx, cancelList := context.WithTimeout(ctx, mcpSessionStartTimeout)
	tools, err := session.ListTools(listCtx)
	cancelList()
	if err != nil {
		_ = session.Close()
		if executionID != "" && m.sandboxes != nil {
			_, _, _ = m.sandboxes.CancelExecution(executionID)
		}
		if restoreRequest {
			state = m.recordRestoreFailure(ctx, server, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		} else {
			state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		}
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:       LifecycleActionStart,
			Server:       resource,
			ExecutionID:  executionID,
			FailureClass: "transport_runtime_failure",
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}

	now := time.Now().UTC()
	if state.LastStartedAt != nil {
		state.RestartCount++
	}
	state.Status = LifecycleStatusHealthy
	state.LastExecutionID = executionID
	state.LastSessionID = session.ID()
	state.LastStartedAt = &now
	state.LastHeartbeatAt = &now
	state.HealthReason = ""
	state.FailureCount = 0
	state.ReconnectAttemptCount = 0
	state.NextReconnectAt = nil
	if restoreRequest {
		state.LastRecoveryAt = &now
		state.LastRecoveryClass = "restore_succeeded"
	}
	state.UpdatedAt = now

	m.mu.Lock()
	m.states[serverID] = state
	m.sessions[serverID] = &sessionState{
		sessionID:     session.ID(),
		executionID:   executionID,
		session:       session,
		transportKind: server.TransportKind,
	}
	if m.tools[serverID] == nil {
		m.tools[serverID] = map[string]Tool{}
	} else {
		for toolName, existing := range m.tools[serverID] {
			existing.DiscoveryStatus = DiscoveryStatusStale
			existing.UpdatedAt = now
			m.tools[serverID][toolName] = existing
		}
	}
	for _, tool := range tools {
		tool.ServerID = serverID
		tool.DiscoveryStatus = DiscoveryStatusDiscovered
		tool.UpdatedAt = now
		tool.LastDiscoveredAt = &now
		m.tools[serverID][tool.ToolName] = cloneTool(tool)
	}
	resource = m.buildServerResourceLocked(server)
	persistedTools := cloneToolMap(m.tools[serverID])
	m.mu.Unlock()

	if err := m.persistState(ctx, state); err != nil {
		return LifecycleResponse{}, err
	}
	if err := m.persistToolMap(ctx, serverID, persistedTools); err != nil {
		return LifecycleResponse{}, err
	}
	if err := m.publishEvent(ctx, "mcp", "mcp.server_started", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId":      serverID,
		"status":        state.Status,
		"executionId":   executionID,
		"sessionId":     session.ID(),
		"toolCount":     len(tools),
		"transportKind": server.TransportKind,
	}); err != nil {
		return LifecycleResponse{}, err
	}
	if err := m.publishHealthChanged(ctx, serverID, state.Status, state.HealthReason); err != nil {
		return LifecycleResponse{}, err
	}
	if restoreRequest {
		if err := m.publishEvent(ctx, "mcp", "mcp.server_restore_completed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
			"serverId":      serverID,
			"transportKind": server.TransportKind,
			"sessionId":     session.ID(),
			"toolCount":     len(tools),
		}); err != nil {
			return LifecycleResponse{}, err
		}
	}

	go m.watchSession(serverID, executionID, session)

	resource, _ = m.GetServerResource(serverID)
	return LifecycleResponse{
		Action:       LifecycleActionStart,
		Server:       resource,
		ExecutionID:  executionID,
		PreflightMs:  time.Since(startedAt).Milliseconds(),
		Idempotent:   false,
		FailureClass: "",
	}, nil
}

func (m *Manager) Stop(ctx context.Context, serverID string) (LifecycleResponse, error) {
	return m.stopOrCancel(ctx, serverID, false)
}

func (m *Manager) Cancel(ctx context.Context, serverID string) (LifecycleResponse, error) {
	return m.stopOrCancel(ctx, serverID, true)
}

func (m *Manager) Restart(ctx context.Context, serverID, requestedBy string) (LifecycleResponse, error) {
	if _, err := m.stopOrCancel(ctx, serverID, false); err != nil && !errors.Is(err, ErrServerNotFound) {
		return LifecycleResponse{}, err
	}
	response, err := m.Start(ctx, serverID, requestedBy)
	if err != nil {
		return LifecycleResponse{}, err
	}
	response.Action = LifecycleActionRestart
	return response, nil
}

type updateOperation struct {
	serverID string
	input    UpdateServerInput
}

func (m *Manager) upsertServer(ctx context.Context, createInput CreateServerInput, update *updateOperation) (ServerResource, bool, error) {
	now := time.Now().UTC()
	var (
		server  Server
		created bool
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	if update == nil {
		serverID := strings.TrimSpace(createInput.ServerID)
		if serverID == "" {
			return ServerResource{}, false, ErrServerIDRequired
		}
		if existing, ok := m.servers[serverID]; ok {
			server = existing
			created = false
		} else {
			server = Server{
				TenantID:         activeTenantID(ctx),
				ServerID:         serverID,
				Source:           SourceAPI,
				OriginKind:       OriginKindManual,
				InstallMethod:    InstallMethodAPI,
				EnvironmentScope: string(m.cfg.Environment),
				CreatedAt:        now,
				TransportKind:    TransportKindStdio,
				Declaration:      defaultDeclaration(),
			}
			created = true
			m.serverIDs = append(m.serverIDs, serverID)
		}
		if tenantID := activeTenantID(ctx); tenantID != "" {
			server.TenantID = tenantID
		}
		server.DisplayName = strings.TrimSpace(createInput.DisplayName)
		if createInput.OriginKind != "" {
			server.OriginKind = createInput.OriginKind
		}
		server.CatalogEntryID = strings.TrimSpace(createInput.CatalogEntryID)
		if createInput.InstallMethod != "" {
			server.InstallMethod = createInput.InstallMethod
		}
		if strings.TrimSpace(createInput.EnvironmentScope) != "" {
			server.EnvironmentScope = strings.TrimSpace(createInput.EnvironmentScope)
		}
		server.Enabled = createInput.Enabled
		server.SandboxProfileID = strings.TrimSpace(createInput.SandboxProfileID)
		server.DeclarationID = strings.TrimSpace(createInput.DeclarationID)
		if createInput.Declaration != nil {
			server.Declaration = normalizeDeclaration(*createInput.Declaration)
		} else {
			server.Declaration = normalizeDeclaration(server.Declaration)
		}
		if createInput.TransportKind != "" {
			server.TransportKind = createInput.TransportKind
		}
		server.Command = strings.TrimSpace(createInput.Command)
		server.Args = cloneStrings(createInput.Args)
		server.Endpoint = strings.TrimSpace(createInput.Endpoint)
		server.WebsocketConfig = cloneWebsocketConfig(createInput.WebsocketConfig)
		server.WorkingDir = strings.TrimSpace(createInput.WorkingDir)
		server.SecretRefs = cleanStrings(createInput.SecretRefs)
		server.AutoRestart = createInput.AutoRestart
		server.OperatorModified = createInput.OperatorModified
		server.CatalogManagement = cloneCatalogManagement(createInput.CatalogManagement)
		server.Source = SourceAPI
		server.UpdatedAt = now
	} else {
		existing, ok := m.servers[update.serverID]
		if !ok {
			return ServerResource{}, false, ErrServerNotFound
		}
		server = existing
		if tenantID := activeTenantID(ctx); tenantID != "" && server.TenantID != "" && server.TenantID != tenantID {
			return ServerResource{}, false, ErrServerNotFound
		}
		created = false
		if update.input.DisplayName != nil {
			server.DisplayName = strings.TrimSpace(*update.input.DisplayName)
			server.OperatorModified = true
		}
		if update.input.Enabled != nil {
			server.Enabled = *update.input.Enabled
			server.OperatorModified = true
		}
		if update.input.SandboxProfileID != nil {
			server.SandboxProfileID = strings.TrimSpace(*update.input.SandboxProfileID)
			server.OperatorModified = true
		}
		if update.input.DeclarationID != nil {
			server.DeclarationID = strings.TrimSpace(*update.input.DeclarationID)
			server.OperatorModified = true
		}
		if update.input.Declaration != nil {
			server.Declaration = normalizeDeclaration(*update.input.Declaration)
			server.OperatorModified = true
		}
		if update.input.TransportKind != nil {
			server.TransportKind = *update.input.TransportKind
			server.OperatorModified = true
		}
		if update.input.Command != nil {
			server.Command = strings.TrimSpace(*update.input.Command)
			server.OperatorModified = true
		}
		if update.input.Args != nil {
			server.Args = cloneStrings(update.input.Args)
			server.OperatorModified = true
		}
		if update.input.Endpoint != nil {
			server.Endpoint = strings.TrimSpace(*update.input.Endpoint)
			server.OperatorModified = true
		}
		if update.input.WebsocketConfig != nil {
			server.WebsocketConfig = cloneWebsocketConfig(update.input.WebsocketConfig)
			server.OperatorModified = true
		}
		if update.input.WorkingDir != nil {
			server.WorkingDir = strings.TrimSpace(*update.input.WorkingDir)
			server.OperatorModified = true
		}
		if update.input.SecretRefs != nil {
			server.SecretRefs = cleanStrings(update.input.SecretRefs)
			server.OperatorModified = true
		}
		if update.input.AutoRestart != nil {
			server.AutoRestart = *update.input.AutoRestart
			server.OperatorModified = true
		}
		server.UpdatedAt = now
	}
	if server.TransportKind == "" {
		server.TransportKind = TransportKindStdio
	}
	server.Declaration = normalizeDeclaration(server.Declaration)
	if err := m.validateServer(server); err != nil {
		return ServerResource{}, false, err
	}
	m.servers[server.ServerID] = cloneServer(server)
	if _, ok := m.states[server.ServerID]; !ok {
		m.states[server.ServerID] = defaultStateForServer(server)
	}
	resource := m.buildServerResourceLocked(server)
	if err := m.persistServer(ctx, server); err != nil {
		return ServerResource{}, false, err
	}
	if err := m.persistState(ctx, m.states[server.ServerID]); err != nil {
		return ServerResource{}, false, err
	}
	if err := m.persistDeclarationView(ctx, server); err != nil {
		return ServerResource{}, false, err
	}
	eventName := "mcp.server_updated"
	if created {
		eventName = "mcp.server_registered"
	}
	if err := m.publishEvent(ctx, "mcp", eventName, events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"serverId":           server.ServerID,
		"displayName":        server.DisplayName,
		"originKind":         server.OriginKind,
		"catalogEntryId":     server.CatalogEntryID,
		"installMethod":      server.InstallMethod,
		"enabled":            server.Enabled,
		"sandboxProfileId":   server.SandboxProfileID,
		"declarationId":      server.DeclarationID,
		"transportKind":      server.TransportKind,
		"availabilityStatus": resource.AvailabilityStatus,
		"availabilityReason": resource.AvailabilityReason,
		"catalogManagement":  catalogManagementPayload(resource.Server.CatalogManagement),
		"created":            created,
	}); err != nil {
		return ServerResource{}, false, err
	}
	return resource, created, nil
}

func (m *Manager) stopOrCancel(ctx context.Context, serverID string, cancel bool) (LifecycleResponse, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return LifecycleResponse{}, ErrServerIDRequired
	}
	startedAt := time.Now()

	m.mu.Lock()
	server, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return LifecycleResponse{}, ErrServerNotFound
	}
	state := m.states[serverID]
	active := m.sessions[serverID]
	if active == nil {
		resource := m.buildServerResourceLocked(server)
		m.mu.Unlock()
		action := LifecycleActionStop
		if cancel {
			action = LifecycleActionCancel
		}
		return LifecycleResponse{
			Action:      action,
			Server:      resource,
			Idempotent:  true,
			PreflightMs: time.Since(startedAt).Milliseconds(),
		}, nil
	}
	if cancel {
		active.cancelRequested = true
		state.HealthReason = "cancelled by operator"
	} else {
		active.stopRequested = true
		state.HealthReason = "stopped by operator"
	}
	state.Status = LifecycleStatusStopping
	state.UpdatedAt = time.Now().UTC()
	m.states[serverID] = state
	executionID := active.executionID
	resource := m.buildServerResourceLocked(server)
	m.mu.Unlock()

	if err := m.persistState(ctx, state); err != nil {
		return LifecycleResponse{}, err
	}
	if active.executionID == "" {
		_ = active.session.Close()
		now := time.Now().UTC()
		state.Status = LifecycleStatusStopped
		state.LastStoppedAt = &now
		state.UpdatedAt = now
		m.mu.Lock()
		delete(m.sessions, serverID)
		m.states[serverID] = state
		m.mu.Unlock()
		if err := m.persistState(ctx, state); err != nil {
			return LifecycleResponse{}, err
		}
		action := LifecycleActionStop
		failureClass := ""
		if cancel {
			action = LifecycleActionCancel
			failureClass = "cancelled"
		}
		resource, _ = m.GetServerResource(serverID)
		if err := m.publishEvent(ctx, "mcp", "mcp.server_stopped", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
			"serverId":      serverID,
			"status":        state.Status,
			"executionId":   "",
			"sessionId":     active.sessionID,
			"cancelled":     cancel,
			"transportKind": active.transportKind,
		}); err != nil {
			return LifecycleResponse{}, err
		}
		return LifecycleResponse{
			Action:       action,
			Server:       resource,
			ExecutionID:  "",
			FailureClass: failureClass,
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}
	execution, _, err := m.sandboxes.CancelExecution(executionID)
	if err != nil {
		return LifecycleResponse{}, err
	}
	action := LifecycleActionStop
	failureClass := ""
	if cancel {
		action = LifecycleActionCancel
		failureClass = "cancelled"
	}
	resource, _ = m.GetServerResource(serverID)
	if err := m.publishEvent(ctx, "mcp", "mcp.server_stopped", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId":    serverID,
		"status":      state.Status,
		"executionId": execution.ExecutionID,
		"cancelled":   cancel,
	}); err != nil {
		return LifecycleResponse{}, err
	}
	return LifecycleResponse{
		Action:       action,
		Server:       resource,
		ExecutionID:  executionID,
		FailureClass: failureClass,
		PreflightMs:  time.Since(startedAt).Milliseconds(),
	}, nil
}

func (m *Manager) watchSession(serverID, executionID string, session Session) {
	err := <-session.Done()
	m.mu.Lock()
	active := m.sessions[serverID]
	if active != nil && active.session == session {
		delete(m.sessions, serverID)
	}
	server := m.servers[serverID]
	state := m.states[serverID]
	stopRequested := active != nil && active.stopRequested
	cancelRequested := active != nil && active.cancelRequested
	transportKind := TransportKindStdio
	if active != nil {
		transportKind = active.transportKind
	}
	m.mu.Unlock()

	if executionID == "" {
		if stopRequested || cancelRequested {
			now := time.Now().UTC()
			state.Status = LifecycleStatusStopped
			state.LastStoppedAt = &now
			state.UpdatedAt = now
			m.mu.Lock()
			m.states[serverID] = state
			m.mu.Unlock()
			_ = m.persistState(context.Background(), state)
			_ = m.publishHealthChanged(context.Background(), serverID, state.Status, state.HealthReason)
			return
		}
		if err != nil {
			if transportKind == TransportKindWebsocket && server.Enabled && server.AutoRestart {
				m.scheduleWebsocketReconnect(serverID, state, err)
				return
			}
			state = m.recordFailure(context.Background(), serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		}
		if state.Status == LifecycleStatusFailed && server.Enabled && server.AutoRestart {
			m.scheduleRestart(serverID, state)
		}
		return
	}

	execution, ok := m.sandboxes.GetExecution(executionID)
	if ok {
		state = m.updateStateFromExecution(context.Background(), serverID, state, execution, stopRequested || cancelRequested)
	} else if err != nil {
		state = m.recordFailure(context.Background(), serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
	}

	if state.Status == LifecycleStatusFailed && server.Enabled && server.AutoRestart {
		m.scheduleRestart(serverID, state)
	}
	_ = transportKind
}

func (m *Manager) scheduleRestart(serverID string, state ServerState) {
	delay := mcpBackoffDelay(state.FailureCount)
	next := time.Now().UTC().Add(delay)

	m.mu.Lock()
	state.Status = LifecycleStatusBackingOff
	state.NextRestartAt = &next
	state.UpdatedAt = time.Now().UTC()
	m.states[serverID] = state
	m.mu.Unlock()
	_ = m.persistState(context.Background(), state)
	_ = m.publishHealthChanged(context.Background(), serverID, state.Status, state.HealthReason)

	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		_, _ = m.Start(context.Background(), serverID, "mcp.auto_restart")
	}()
}

func (m *Manager) scheduleWebsocketReconnect(serverID string, state ServerState, cause error) {
	now := time.Now().UTC()
	attempt := state.ReconnectAttemptCount + 1
	reason := firstNonEmpty(strings.TrimSpace(errorString(cause)), state.HealthReason, "websocket session disconnected")
	if attempt > websocketReconnectMaxAttempts {
		state.Status = LifecycleStatusFailed
		state.HealthReason = reason
		state.LastRecoveryAt = &now
		state.LastRecoveryClass = "reconnect_failed"
		state.NextReconnectAt = nil
		state.UpdatedAt = now
		m.mu.Lock()
		m.states[serverID] = state
		m.mu.Unlock()
		_ = m.persistState(context.Background(), state)
		_ = m.publishEvent(context.Background(), "mcp", "mcp.server_reconnect_failed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
			"serverId":      serverID,
			"transportKind": TransportKindWebsocket,
			"attempt":       state.ReconnectAttemptCount,
			"reason":        reason,
			"failureClass":  "reconnect_exhausted",
		})
		_ = m.publishHealthChanged(context.Background(), serverID, state.Status, state.HealthReason)
		return
	}

	delay := mcpBackoffDelay(attempt)
	next := now.Add(delay)
	state.Status = LifecycleStatusDegraded
	state.HealthReason = reason
	state.ReconnectAttemptCount = attempt
	state.LastRecoveryAt = &now
	state.LastRecoveryClass = "reconnect_scheduled"
	state.NextReconnectAt = &next
	state.UpdatedAt = now

	m.mu.Lock()
	m.states[serverID] = state
	m.mu.Unlock()
	_ = m.persistState(context.Background(), state)
	_ = m.publishEvent(context.Background(), "mcp", "mcp.server_reconnect_scheduled", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId":      serverID,
		"transportKind": TransportKindWebsocket,
		"attempt":       attempt,
		"reason":        reason,
		"nextRetryAt":   next.UTC().Format(time.RFC3339Nano),
	})
	_ = m.publishHealthChanged(context.Background(), serverID, state.Status, state.HealthReason)

	go func(expectedAttempt int) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		response, _ := m.Start(context.Background(), serverID, "mcp.websocket_reconnect")
		if response.Server.State.Status == LifecycleStatusHealthy {
			recoveredAt := time.Now().UTC()
			m.mu.Lock()
			latest := m.states[serverID]
			latest.LastRecoveryAt = &recoveredAt
			latest.LastRecoveryClass = "reconnect_succeeded"
			latest.ReconnectAttemptCount = 0
			latest.NextReconnectAt = nil
			latest.UpdatedAt = recoveredAt
			m.states[serverID] = latest
			m.mu.Unlock()
			_ = m.persistState(context.Background(), latest)
			_ = m.publishEvent(context.Background(), "mcp", "mcp.server_reconnect_completed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
				"serverId":      serverID,
				"transportKind": TransportKindWebsocket,
				"attempt":       expectedAttempt,
				"sessionId":     response.Server.State.LastSessionID,
			})
			return
		}
		resource, ok := m.GetServerResource(serverID)
		if !ok || !resource.Enabled || !resource.AutoRestart {
			return
		}
		m.scheduleWebsocketReconnect(serverID, resource.State, fmt.Errorf("%s", firstNonEmpty(response.BlockedReason, response.Server.State.HealthReason, response.FailureClass, "websocket reconnect failed")))
	}(attempt)
}

func (m *Manager) updateStateFromExecution(ctx context.Context, serverID string, state ServerState, execution sandbox.Execution, requestedStop bool) ServerState {
	now := time.Now().UTC()
	state.LastExecutionID = execution.ExecutionID
	state.LastPolicyRecordID = ""
	if execution.Consumer != nil && execution.Consumer.PolicyRecord != nil {
		state.LastPolicyRecordID = execution.Consumer.PolicyRecord.PolicyRecordID
	}
	state.UpdatedAt = now
	switch execution.Status {
	case sandbox.ExecutionStatusDenied, sandbox.ExecutionStatusUnsupported:
		state.Status = lifecycleStatusFromExecution(execution)
		state.HealthReason = firstNonEmpty(execution.Result.Error, execution.Decision.Explanation)
	case sandbox.ExecutionStatusCancelled:
		state.Status = LifecycleStatusStopped
		if requestedStop {
			state.HealthReason = firstNonEmpty(state.HealthReason, "cancelled by operator")
		} else {
			state.HealthReason = firstNonEmpty(execution.Result.Error, "execution was cancelled")
		}
		state.LastStoppedAt = &now
	case sandbox.ExecutionStatusCompleted:
		if requestedStop {
			state.Status = LifecycleStatusStopped
			state.HealthReason = firstNonEmpty(state.HealthReason, "stopped by operator")
			state.LastStoppedAt = &now
		} else {
			state.Status = LifecycleStatusFailed
			state.HealthReason = "mcp transport exited unexpectedly"
			state.FailureCount++
		}
	case sandbox.ExecutionStatusFailed:
		state.Status = LifecycleStatusFailed
		state.HealthReason = firstNonEmpty(execution.Result.Error, execution.Result.ErrorCode, "sandbox execution failed")
		state.FailureCount++
	default:
		state.Status = LifecycleStatusDegraded
		state.HealthReason = firstNonEmpty(execution.Result.Error, "mcp server became unavailable")
	}

	m.mu.Lock()
	m.states[serverID] = state
	m.mu.Unlock()
	_ = m.persistState(ctx, state)
	_ = m.publishHealthChanged(ctx, serverID, state.Status, state.HealthReason)
	return state
}

func (m *Manager) recordFailure(ctx context.Context, serverID string, state ServerState, status LifecycleStatus, reason, failureClass string) ServerState {
	now := time.Now().UTC()
	state.Status = status
	state.HealthReason = strings.TrimSpace(reason)
	state.FailureCount++
	state.UpdatedAt = now
	m.mu.Lock()
	m.states[serverID] = state
	m.mu.Unlock()
	_ = m.persistState(ctx, state)
	_ = m.publishEvent(ctx, "mcp", "mcp.server_failed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId":     serverID,
		"status":       status,
		"reason":       state.HealthReason,
		"failureClass": failureClass,
	})
	_ = m.publishHealthChanged(ctx, serverID, state.Status, state.HealthReason)
	return state
}

func (m *Manager) recordRestoreFailure(ctx context.Context, server Server, state ServerState, status LifecycleStatus, reason, failureClass string) ServerState {
	state = m.recordFailure(ctx, server.ServerID, state, status, reason, failureClass)
	now := time.Now().UTC()
	state.LastRecoveryAt = &now
	state.LastRecoveryClass = "restore_failed"
	state.NextReconnectAt = nil
	state.UpdatedAt = now
	m.mu.Lock()
	m.states[server.ServerID] = state
	m.mu.Unlock()
	_ = m.persistState(ctx, state)
	_ = m.publishEvent(ctx, "mcp", "mcp.server_restore_failed", events.Resource{Kind: resourceKindServer, ID: server.ServerID}, map[string]any{
		"serverId":      server.ServerID,
		"transportKind": server.TransportKind,
		"reason":        state.HealthReason,
		"failureClass":  failureClass,
	})
	return state
}

func (m *Manager) publishHealthChanged(ctx context.Context, serverID string, status LifecycleStatus, reason string) error {
	resource, _ := m.GetServerResource(serverID)
	return m.publishEvent(ctx, "mcp", "mcp.server_health_changed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId":           serverID,
		"status":             status,
		"reason":             strings.TrimSpace(reason),
		"availabilityStatus": resource.AvailabilityStatus,
		"availabilityReason": resource.AvailabilityReason,
		"catalogManagement":  catalogManagementPayload(resource.Server.CatalogManagement),
	})
}

func (m *Manager) buildExecutionRequest(ctx context.Context, server Server, consumer *sandbox.ConsumerContractView, approvalID string) (sandbox.ExecutionRequest, error) {
	env, err := m.resolveSecretEnv(ctx, server)
	if err != nil {
		return sandbox.ExecutionRequest{}, err
	}
	access := sandbox.AccessRequest{
		ReadRoots:     cloneStrings(server.Declaration.ReadRoots),
		WriteRoots:    cloneStrings(server.Declaration.WriteRoots),
		NetworkMode:   server.Declaration.NetworkMode,
		AllowedHosts:  cloneStrings(server.Declaration.AllowedHosts),
		AllowedPorts:  cloneInts(server.Declaration.AllowedPorts),
		AllowLoopback: server.Declaration.AllowLoopback,
	}
	if len(access.ReadRoots) == 0 && strings.TrimSpace(server.WorkingDir) != "" {
		access.ReadRoots = []string{strings.TrimSpace(server.WorkingDir)}
	}
	if len(access.WriteRoots) == 0 && strings.TrimSpace(server.WorkingDir) != "" {
		access.WriteRoots = []string{strings.TrimSpace(server.WorkingDir)}
	}
	return sandbox.ExecutionRequest{
		ProfileID:    server.SandboxProfileID,
		Command:      server.Command,
		Args:         cloneStrings(server.Args),
		Cwd:          server.WorkingDir,
		Env:          env,
		RequestedBy:  "mcp",
		ResourceKind: resourceKindServer,
		ResourceID:   server.ServerID,
		Scope:        "mcp.lifecycle",
		ApprovalID:   approvalID,
		Reason:       "mcp lifecycle",
		Metadata: map[string]string{
			"mcpServerId":   server.ServerID,
			"transportKind": string(server.TransportKind),
		},
		Access:   access,
		Consumer: consumer,
	}, nil
}

func (m *Manager) buildLifecycleConsumerView(ctx context.Context, server Server, requestedBy string) (*sandbox.ConsumerContractView, error) {
	return m.buildConsumerView(ctx, server, firstNonEmpty(strings.TrimSpace(requestedBy), "mcp"), "lifecycle.start", server.DeclarationID, sandbox.ApprovalModeAllow, sandbox.DecisionResolutionAllow, sandbox.DecisionApprovalStatusNotApplicable, sandbox.PolicyRecordStatusPreflightAllowed)
}

func (m *Manager) buildToolConsumerView(ctx context.Context, server Server, toolName, runtimeSurface, requestedBy string, approvalMode sandbox.ApprovalMode) (*sandbox.ConsumerContractView, error) {
	declarationID := fmt.Sprintf("%s:tool:%s:%s", server.DeclarationID, strings.TrimSpace(runtimeSurface), strings.TrimSpace(toolName))
	return m.buildConsumerView(ctx, server, firstNonEmpty(strings.TrimSpace(requestedBy), "mcp"), "tool_call.execute", declarationID, approvalMode, sandbox.DecisionResolutionAllow, sandbox.DecisionApprovalStatusNotApplicable, sandbox.PolicyRecordStatusPreflightAllowed)
}

func (m *Manager) buildConsumerView(ctx context.Context, server Server, requestedBy, operationKind, declarationID string, approvalMode sandbox.ApprovalMode, decision sandbox.DecisionResolution, approvalStatus sandbox.DecisionApprovalStatus, status sandbox.PolicyRecordStatus) (*sandbox.ConsumerContractView, error) {
	secretScope, err := m.buildSecretScope(ctx, server)
	if err != nil {
		return nil, err
	}
	return &sandbox.ConsumerContractView{
		Declaration: &sandbox.ConsumerRequirementDeclaration{
			DeclarationID:               strings.TrimSpace(declarationID),
			ConsumerKind:                sandbox.ConsumerKindMCPServer,
			ConsumerID:                  server.ServerID,
			OperationKind:               operationKind,
			ProfileID:                   server.SandboxProfileID,
			ExecutionMode:               server.Declaration.ExecutionMode,
			AllowedBackendKinds:         cloneBackendKinds(server.Declaration.AllowedBackendKinds),
			ReadRoots:                   cloneStrings(server.Declaration.ReadRoots),
			WriteRoots:                  cloneStrings(server.Declaration.WriteRoots),
			NetworkMode:                 server.Declaration.NetworkMode,
			AllowedHosts:                cloneStrings(server.Declaration.AllowedHosts),
			AllowedPorts:                cloneInts(server.Declaration.AllowedPorts),
			AllowLoopback:               server.Declaration.AllowLoopback,
			SecretRefs:                  cleanStrings(server.SecretRefs),
			ApprovalMode:                approvalMode,
			RequiredEnforcementStrength: strings.TrimSpace(server.Declaration.RequiredEnforcementStrength),
			Active:                      server.Declaration.Active,
			Source:                      sandbox.SourceBuiltin,
		},
		SecretScope: secretScope,
		PolicyRecord: &sandbox.ConsumerPolicyRecord{
			PolicyRecordID:      "policy_mcp_" + server.ServerID + "_" + strings.ReplaceAll(operationKind, ".", "_") + "_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", ""),
			ConsumerKind:        sandbox.ConsumerKindMCPServer,
			ConsumerID:          server.ServerID,
			OperationKind:       operationKind,
			DeclarationID:       strings.TrimSpace(declarationID),
			RequestedBy:         firstNonEmpty(strings.TrimSpace(requestedBy), "mcp"),
			Decision:            decision,
			ApprovalStatus:      approvalStatus,
			SecretResolution:    secretResolution(secretScope),
			EnforcementStrength: firstNonEmpty(strings.TrimSpace(server.Declaration.RequiredEnforcementStrength), "declared_only"),
			StartedAt:           time.Now().UTC(),
			Status:              status,
		},
	}, nil
}

func (m *Manager) buildCatalogManagementLocked(server Server) *CatalogManagement {
	if server.OriginKind != OriginKindCatalog && server.CatalogManagement == nil {
		return nil
	}
	management := cloneCatalogManagement(server.CatalogManagement)
	if management == nil {
		management = &CatalogManagement{}
	}
	entry, ok := m.GetCatalogEntry(server.CatalogEntryID)
	if ok && management.SourceKind == "" {
		management.SourceKind = entry.SourceKind
	}
	if management.InstalledRevision == "" {
		if spec, ok := m.catalogSpecForServer(server, entry, ok); ok {
			management.InstalledRevision = fingerprintCreateServerSpec(spec)
		}
	}
	if ok {
		if spec, ok := m.catalogSpecForServer(server, entry, true); ok {
			management.CurrentRevision = fingerprintCreateServerSpec(spec)
		}
	} else {
		management.CurrentRevision = ""
	}
	management.DriftStatus, management.DriftReason = assessCatalogDrift(server, management, ok)
	return management
}

func (m *Manager) catalogSpecForServer(server Server, entry CatalogEntry, ok bool) (CreateServerInput, bool) {
	if !ok {
		return CreateServerInput{}, false
	}
	method := server.InstallMethod
	if method == "" && server.CatalogManagement != nil && server.CatalogManagement.InstallInputSnapshot.InstallMethod != "" {
		method = server.CatalogManagement.InstallInputSnapshot.InstallMethod
	}
	if method == "" {
		method = InstallMethodAPI
	}
	snapshot := CatalogInstallSnapshot{}
	if server.CatalogManagement != nil {
		snapshot = cloneCatalogInstallSnapshot(server.CatalogManagement.InstallInputSnapshot)
	}
	if snapshot.ServerID == "" {
		snapshot = installSnapshotFromCreateSpec(serverToCreateInput(server))
	}
	spec := mergeCatalogInstallInput(entry, catalogInstallInputFromSnapshot(snapshot), method, m.cfg.Environment)
	return spec, true
}

func assessCatalogDrift(server Server, management *CatalogManagement, entryPresent bool) (CatalogDriftStatus, string) {
	if server.OriginKind != OriginKindCatalog {
		return "", ""
	}
	if !entryPresent {
		return CatalogDriftStatusMissingEntry, "catalog entry is no longer available"
	}
	if server.OperatorModified {
		if management.InstalledRevision != "" && management.CurrentRevision != "" && management.InstalledRevision != management.CurrentRevision {
			return CatalogDriftStatusLocallyModified, "server has local modifications and the catalog entry has changed"
		}
		return CatalogDriftStatusLocallyModified, "server has local operator modifications"
	}
	if management.InstalledRevision != "" && management.CurrentRevision != "" && management.InstalledRevision != management.CurrentRevision {
		return CatalogDriftStatusCatalogUpdated, "installed server no longer matches the current catalog revision"
	}
	return CatalogDriftStatusInSync, ""
}

func serverToCreateInput(server Server) CreateServerInput {
	return CreateServerInput{
		ServerID:          server.ServerID,
		DisplayName:       server.DisplayName,
		OriginKind:        server.OriginKind,
		CatalogEntryID:    server.CatalogEntryID,
		InstallMethod:     server.InstallMethod,
		EnvironmentScope:  server.EnvironmentScope,
		Enabled:           server.Enabled,
		SandboxProfileID:  server.SandboxProfileID,
		DeclarationID:     server.DeclarationID,
		Declaration:       cloneDeclarationPtr(server.Declaration),
		TransportKind:     server.TransportKind,
		Command:           server.Command,
		Args:              cloneStrings(server.Args),
		Endpoint:          server.Endpoint,
		WebsocketConfig:   cloneWebsocketConfig(server.WebsocketConfig),
		WorkingDir:        server.WorkingDir,
		SecretRefs:        cleanStrings(server.SecretRefs),
		AutoRestart:       server.AutoRestart,
		OperatorModified:  server.OperatorModified,
		CatalogManagement: cloneCatalogManagement(server.CatalogManagement),
	}
}

func (m *Manager) buildServerResourceLocked(server Server) ServerResource {
	projectedServer := cloneServer(server)
	if projectedServer.TransportKind == TransportKindWebsocket {
		projectedServer.Endpoint = sanitizeWebsocketEndpointForProjection(projectedServer.Endpoint)
	}
	projectedServer.CatalogManagement = sanitizeCatalogManagementProjection(m.buildCatalogManagementLocked(server))
	state := cloneServerState(m.states[server.ServerID])
	toolCount := len(m.tools[server.ServerID])
	tools := make([]ToolResource, 0, toolCount)
	for _, tool := range m.tools[server.ServerID] {
		tools = append(tools, m.buildToolResourceLocked(server, tool))
	}
	availabilityStatus, availabilityReason := m.evaluateServerAvailabilityLocked(projectedServer)
	return ServerResource{
		Server:                 projectedServer,
		State:                  state,
		SecretSummary:          m.buildSecretSummaries(projectedServer),
		ToolCount:              toolCount,
		Tools:                  tools,
		TransportConfigSummary: m.transportConfigSummary(projectedServer),
		WebsocketAuthSummary:   m.buildWebsocketAuthSummary(projectedServer),
		AvailabilityStatus:     availabilityStatus,
		AvailabilityReason:     availabilityReason,
	}
}

func sanitizeCatalogManagementProjection(management *CatalogManagement) *CatalogManagement {
	if management == nil {
		return nil
	}
	projected := cloneCatalogManagement(management)
	projected.InstallInputSnapshot = sanitizeCatalogInstallSnapshotProjection(projected.InstallInputSnapshot)
	return projected
}

func sanitizeCatalogInstallSnapshotProjection(snapshot CatalogInstallSnapshot) CatalogInstallSnapshot {
	snapshot.Command = ""
	snapshot.Args = nil
	snapshot.Endpoint = ""
	snapshot.WorkingDir = ""
	return snapshot
}

func (m *Manager) buildToolResourceLocked(server Server, tool Tool) ToolResource {
	tool.TenantID = server.TenantID
	ruleMap := m.exposure[server.ServerID][tool.ToolName]
	exposure := make([]ToolExposureRule, 0, len(ruleMap))
	approvalRequired := false
	effective := "unavailable"
	reason := ""
	for _, rule := range ruleMap {
		rule.TenantID = server.TenantID
		exposure = append(exposure, cloneToolExposureRule(rule))
		if rule.Active && rule.ExposureMode == ExposureModeApprovalRequired {
			approvalRequired = true
		}
		if rule.Active && (rule.ExposureMode == ExposureModeAllow || rule.ExposureMode == ExposureModeApprovalRequired) {
			effective = "available"
		}
	}
	state := m.states[server.ServerID]
	if !server.Enabled {
		effective = "blocked"
		reason = "server is disabled"
	} else if state.Status != LifecycleStatusHealthy {
		effective = "unavailable"
		reason = firstNonEmpty(state.HealthReason, "server is not healthy")
	}
	if len(exposure) == 0 {
		effective = "blocked"
		reason = "tool is not allowlisted for any runtime surface"
	}
	if tool.DiscoveryStatus != DiscoveryStatusDiscovered {
		effective = "unavailable"
		reason = firstNonEmpty(reason, "tool is not currently discovered")
	}
	return ToolResource{
		Tool:                  cloneTool(tool),
		Exposure:              exposure,
		EffectiveAvailability: effective,
		ApprovalRequired:      approvalRequired,
		UnavailableReason:     reason,
	}
}

func activeTenantID(ctx context.Context) string {
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok {
		return ""
	}
	return strings.TrimSpace(tenantContext.TenantID)
}

func (m *Manager) buildSecretScope(ctx context.Context, server Server) ([]sandbox.SecretScopeOutcome, error) {
	items, err := m.listSecretBindings(ctx, server)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (m *Manager) listSecretBindings(ctx context.Context, server Server) ([]sandbox.SecretScopeOutcome, error) {
	envScope := sandbox.SecretEnvironmentScopeBoth
	switch m.cfg.Environment {
	case config.EnvironmentTest:
		envScope = sandbox.SecretEnvironmentScopeTest
	case config.EnvironmentProd:
		envScope = sandbox.SecretEnvironmentScopeProd
	}
	items := make([]sandbox.SecretScopeOutcome, 0, len(server.SecretRefs))
	for _, secretRef := range server.SecretRefs {
		items = append(items, sandbox.SecretScopeOutcome{
			ConsumerKind:     sandbox.ConsumerKindMCPServer,
			ConsumerID:       server.ServerID,
			SecretRef:        secretRef,
			EnvironmentScope: envScope,
			DefaultSource:    sandbox.SecretDefaultSourceInstanceOverride,
			DefaultRuleID:    "mcp_server:" + server.ServerID,
			DeliveryKind:     "environment_variable",
			RedactionRule:    "value_redacted",
			Resolution:       m.resolveSecretRef(ctx, secretRef, envScope),
		})
	}
	return items, nil
}

func (m *Manager) resolveSecretEnv(ctx context.Context, server Server) (map[string]string, error) {
	secretScope, err := m.buildSecretScope(ctx, server)
	if err != nil {
		return nil, err
	}
	resolvedSecrets, err := m.resolveSecretValues(ctx, server.SecretRefs)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, item := range secretScope {
		if item.Resolution != sandbox.SecretResolutionResolved {
			continue
		}
		if value, ok := resolvedSecrets[item.SecretRef]; ok {
			env[item.SecretRef] = value
		}
	}
	return env, nil
}

func (m *Manager) resolveSecretRef(ctx context.Context, secretRef string, envScope sandbox.SecretEnvironmentScope) sandbox.SecretResolution {
	switch m.cfg.Environment {
	case config.EnvironmentTest:
		if envScope != sandbox.SecretEnvironmentScopeTest && envScope != sandbox.SecretEnvironmentScopeBoth {
			return sandbox.SecretResolutionDenied
		}
	case config.EnvironmentProd:
		if envScope != sandbox.SecretEnvironmentScopeProd && envScope != sandbox.SecretEnvironmentScopeBoth {
			return sandbox.SecretResolutionDenied
		}
	}
	resolved, err := m.resolveSecretValues(ctx, []string{secretRef})
	if err != nil {
		if errors.Is(err, tenantctx.ErrTenantContextRequired) {
			return sandbox.SecretResolutionDenied
		}
		return sandbox.SecretResolutionUnavailable
	}
	if _, ok := resolved[secretRef]; ok {
		return sandbox.SecretResolutionResolved
	}
	return sandbox.SecretResolutionUnavailable
}

func (m *Manager) resolveSecretValues(ctx context.Context, secretRefs []string) (map[string]string, error) {
	refs := cleanStrings(secretRefs)
	if len(refs) == 0 {
		return map[string]string{}, nil
	}
	secretManager := m.secrets
	if secretManager == nil {
		return ResolveMCPSecrets(m.cfg.DataDir, refs)
	}
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" {
		return nil, tenantctx.ErrTenantContextRequired
	}
	resolved := make(map[string]string, len(refs))
	for _, secretRef := range refs {
		secret, err := secretManager.Resolve(ctx, secrets.ResolveInput{
			TenantID:  tenantContext.TenantID,
			SecretRef: secretRef,
		})
		if err != nil {
			if errors.Is(err, secrets.ErrSecretNotFound) ||
				errors.Is(err, secrets.ErrSecretDisabled) ||
				errors.Is(err, secrets.ErrSecretVersionNotFound) {
				continue
			}
			return nil, err
		}
		if strings.TrimSpace(secret.Value) != "" {
			resolved[secretRef] = secret.Value
		}
	}
	return resolved, nil
}

func (m *Manager) persistDeclarationView(ctx context.Context, server Server) error {
	if m.sandboxes == nil {
		return nil
	}
	view, err := m.buildLifecycleConsumerView(ctx, server, "mcp")
	if err != nil {
		return err
	}
	view.PolicyRecord = nil
	return m.persistConsumerView(ctx, view)
}

func (m *Manager) persistConsumerView(ctx context.Context, view *sandbox.ConsumerContractView) error {
	if view == nil {
		return nil
	}
	if m.sandboxes != nil {
		return m.sandboxes.PersistConsumerView(ctx, view)
	}
	return nil
}

func (m *Manager) persistApproval(ctx context.Context, approval policy.Approval) error {
	if m.store == nil {
		return nil
	}
	return m.store.UpsertApproval(ctx, approval)
}

func (m *Manager) persistDecision(ctx context.Context, decision policy.Decision) error {
	if m.store == nil {
		return nil
	}
	return m.store.UpsertDecision(ctx, decision)
}

func (m *Manager) persistServer(ctx context.Context, server Server) error {
	if m.store == nil {
		return nil
	}
	document, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("marshal mcp server %s: %w", server.ServerID, err)
	}
	record := store.MCPServerRecord{
		ServerID:  server.ServerID,
		Enabled:   server.Enabled,
		UpdatedAt: server.UpdatedAt,
		Document:  document,
	}
	return m.store.UpsertMCPServer(ctx, record)
}

func (m *Manager) persistState(ctx context.Context, state ServerState) error {
	if m.store == nil {
		return nil
	}
	document, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal mcp server state %s: %w", state.ServerID, err)
	}
	record := store.MCPServerStateRecord{
		ServerID:  state.ServerID,
		Status:    string(state.Status),
		UpdatedAt: state.UpdatedAt,
		Document:  document,
	}
	return m.store.UpsertMCPServerState(ctx, record)
}

func (m *Manager) persistTools(ctx context.Context, serverID string, tools []Tool) error {
	return m.persistToolMap(ctx, serverID, tools)
}

func (m *Manager) persistToolMap(ctx context.Context, serverID string, tools []Tool) error {
	if m.store == nil {
		return nil
	}
	records := make([]store.MCPToolRecord, 0, len(tools))
	for _, tool := range tools {
		document, err := json.Marshal(tool)
		if err != nil {
			return fmt.Errorf("marshal mcp tool %s/%s: %w", serverID, tool.ToolName, err)
		}
		records = append(records, store.MCPToolRecord{
			ServerID:         tool.ServerID,
			ToolName:         tool.ToolName,
			DiscoveryStatus:  string(tool.DiscoveryStatus),
			UpdatedAt:        tool.UpdatedAt,
			LastDiscoveredAt: tool.LastDiscoveredAt,
			Document:         document,
		})
	}
	return m.store.ReplaceMCPTools(ctx, serverID, records)
}

func (m *Manager) persistExposureRule(ctx context.Context, rule ToolExposureRule) error {
	if m.store == nil {
		return nil
	}
	document, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal mcp tool exposure rule %s/%s/%s: %w", rule.ServerID, rule.ToolName, rule.RuntimeSurface, err)
	}
	return m.store.UpsertMCPToolExposureRule(ctx, store.MCPToolExposureRuleRecord{
		ServerID:       rule.ServerID,
		ToolName:       rule.ToolName,
		RuntimeSurface: rule.RuntimeSurface,
		ExposureMode:   string(rule.ExposureMode),
		Active:         rule.Active,
		UpdatedAt:      rule.UpdatedAt,
		Document:       document,
	})
}

func (m *Manager) publishEvent(ctx context.Context, category, name string, resource events.Resource, payload map[string]any) error {
	if m.eventBus == nil && m.store == nil {
		return nil
	}
	event := events.Event{
		EventID:    fmt.Sprintf("evt_%s_%d", strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), ":", "_"), time.Now().UTC().UnixNano()),
		Category:   category,
		Name:       name,
		OccurredAt: time.Now().UTC(),
		Resource:   resource,
		Payload:    payload,
	}
	if m.store != nil {
		persisted, err := m.store.AppendEvent(ctx, event)
		if err != nil {
			return err
		}
		event = persisted
	}
	if m.eventBus != nil {
		m.eventBus.Publish(event)
	}
	return nil
}

func (m *Manager) validateServer(server Server) error {
	if strings.TrimSpace(server.ServerID) == "" {
		return ErrServerIDRequired
	}
	if strings.TrimSpace(server.DeclarationID) == "" {
		return ErrDeclarationIDRequired
	}
	if strings.TrimSpace(server.SandboxProfileID) == "" {
		return ErrProfileIDRequired
	}
	switch server.TransportKind {
	case TransportKindStdio:
		if strings.TrimSpace(server.Command) == "" {
			return ErrCommandRequired
		}
	case TransportKindStreamableHTTP:
		if strings.TrimSpace(server.Endpoint) == "" {
			return ErrTransportUnavailable
		}
	case TransportKindWebsocket:
		if err := validateWebsocketEndpoint(server.Endpoint); err != nil {
			return err
		}
	default:
		return ErrUnsupportedTransport
	}
	if server.AutoRestart && !server.Enabled {
		return ErrAutoRestartRequiresOn
	}
	if !server.Declaration.Active && server.Enabled {
		return errors.New("mcp declaration must be active before the server can be enabled")
	}
	if m.sandboxes != nil && server.Enabled {
		profile, ok := m.sandboxes.GetProfile(server.SandboxProfileID)
		if !ok {
			return fmt.Errorf("sandbox profile %s was not found", server.SandboxProfileID)
		}
		if !profile.Active {
			return fmt.Errorf("sandbox profile %s is inactive", server.SandboxProfileID)
		}
	}
	return nil
}

func (m *Manager) evaluateServerAvailabilityLocked(server Server) (AvailabilityStatus, string) {
	if server.CatalogManagement != nil && server.CatalogManagement.LastRevalidation != nil {
		snapshot := server.CatalogManagement.LastRevalidation
		if snapshot.Status != AvailabilityStatusReady {
			return snapshot.Status, firstNonEmpty(snapshot.Reason, "server requires revalidation")
		}
	}
	switch server.TransportKind {
	case TransportKindStdio:
		if strings.TrimSpace(server.Command) == "" {
			return AvailabilityStatusUnavailable, "stdio command is not configured"
		}
	case TransportKindStreamableHTTP:
		if strings.TrimSpace(server.Endpoint) == "" {
			return AvailabilityStatusUnsupported, "streamable-http endpoint is not configured"
		}
	case TransportKindWebsocket:
		if strings.TrimSpace(server.Endpoint) == "" {
			return AvailabilityStatusUnsupported, "websocket endpoint is not configured"
		}
		if summary := m.buildWebsocketAuthSummary(server); summary != nil && summary.Configured && !summary.Resolved {
			return AvailabilityStatusBlocked, firstNonEmpty(summary.BlockedReason, "websocket auth secret is unavailable")
		}
	default:
		return AvailabilityStatusUnsupported, "transport kind is unsupported"
	}
	for _, summary := range m.buildSecretSummaries(server) {
		if summary.Resolution != string(sandbox.SecretResolutionResolved) {
			return AvailabilityStatusBlocked, fmt.Sprintf("%s is unavailable in %s", summary.SecretRef, summary.EnvironmentScope)
		}
	}
	state := m.states[server.ServerID]
	switch state.Status {
	case LifecycleStatusUnsupported:
		return AvailabilityStatusUnsupported, firstNonEmpty(state.HealthReason, "transport is unsupported")
	case LifecycleStatusFailed, LifecycleStatusDenied, LifecycleStatusDegraded:
		return AvailabilityStatusUnavailable, firstNonEmpty(state.HealthReason, "server is not healthy")
	case LifecycleStatusDisabled:
		return AvailabilityStatusBlocked, "server is disabled"
	default:
	}
	return AvailabilityStatusReady, ""
}

func (m *Manager) transportConfigSummary(server Server) string {
	switch server.TransportKind {
	case TransportKindStreamableHTTP:
		return strings.TrimSpace(server.Endpoint)
	case TransportKindWebsocket:
		summary := strings.TrimSpace(server.Endpoint)
		if auth := m.buildWebsocketAuthSummary(server); auth != nil && auth.Mode != "" {
			summary = strings.TrimSpace(summary + " (" + string(auth.Mode) + ")")
		}
		return summary
	default:
		if strings.TrimSpace(server.Command) == "" {
			return ""
		}
		if len(server.Args) == 0 {
			return strings.TrimSpace(server.Command)
		}
		return strings.TrimSpace(server.Command) + " " + strings.Join(cloneStrings(server.Args), " ")
	}
}

func (m *Manager) buildWebsocketAuthSummary(server Server) *WebsocketAuthSummary {
	if server.TransportKind != TransportKindWebsocket || server.WebsocketConfig == nil || server.WebsocketConfig.Auth == nil {
		return nil
	}
	auth := server.WebsocketConfig.Auth
	summary := &WebsocketAuthSummary{
		Mode:       auth.Mode,
		HeaderName: defaultWebsocketHeaderName(auth),
		Scheme:     defaultWebsocketScheme(auth),
		SecretRef:  strings.TrimSpace(auth.SecretRef),
		Configured: true,
	}
	if summary.SecretRef == "" {
		summary.BlockedReason = "websocket auth secret ref is not configured"
		return summary
	}
	for _, item := range m.buildSecretSummaries(server) {
		if item.SecretRef != summary.SecretRef {
			continue
		}
		summary.Resolved = item.Resolution == string(sandbox.SecretResolutionResolved)
		if !summary.Resolved {
			summary.BlockedReason = fmt.Sprintf("%s is unavailable in %s", item.SecretRef, item.EnvironmentScope)
		}
		return summary
	}
	summary.BlockedReason = fmt.Sprintf("%s is unavailable in %s", summary.SecretRef, m.cfg.Environment)
	return summary
}

func defaultWebsocketHeaderName(auth *WebsocketAuthConfig) string {
	if auth == nil {
		return ""
	}
	if auth.Mode == WebsocketAuthModeBearerHeader && strings.TrimSpace(auth.HeaderName) == "" {
		return "Authorization"
	}
	return strings.TrimSpace(auth.HeaderName)
}

func defaultWebsocketScheme(auth *WebsocketAuthConfig) string {
	if auth == nil {
		return ""
	}
	if auth.Mode == WebsocketAuthModeBearerHeader && strings.TrimSpace(auth.Scheme) == "" {
		return "Bearer"
	}
	return strings.TrimSpace(auth.Scheme)
}

func (m *Manager) resolveWebsocketHeaders(ctx context.Context, server Server) (map[string]string, error) {
	if server.TransportKind != TransportKindWebsocket || server.WebsocketConfig == nil || server.WebsocketConfig.Auth == nil {
		return nil, nil
	}
	auth := server.WebsocketConfig.Auth
	secretRef := strings.TrimSpace(auth.SecretRef)
	if secretRef == "" {
		return nil, fmt.Errorf("websocket auth secret ref is not configured")
	}
	resolved, err := m.resolveSecretValues(ctx, []string{secretRef})
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(resolved[secretRef])
	if value == "" {
		return nil, fmt.Errorf("%s is unavailable in %s", secretRef, m.cfg.Environment)
	}
	headerName := defaultWebsocketHeaderName(auth)
	if headerName == "" {
		return nil, fmt.Errorf("websocket auth header name is not configured")
	}
	if auth.Mode == WebsocketAuthModeBearerHeader {
		return map[string]string{headerName: strings.TrimSpace(defaultWebsocketScheme(auth) + " " + value)}, nil
	}
	return map[string]string{headerName: value}, nil
}

func validateWebsocketEndpoint(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrTransportUnavailable
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("websocket endpoint is invalid: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("websocket endpoint must use ws or wss")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("websocket endpoint must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("websocket endpoint must not include inline credentials; use websocketConfig.auth instead")
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return fmt.Errorf("websocket endpoint must not include inline query parameters; use websocketConfig.auth instead")
	}
	return nil
}

func sanitizeWebsocketEndpointForProjection(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func defaultStateForServer(server Server) ServerState {
	now := time.Now().UTC()
	status := LifecycleStatusStopped
	if !server.Enabled {
		status = LifecycleStatusDisabled
	}
	return ServerState{
		ServerID:  server.ServerID,
		Status:    status,
		UpdatedAt: now,
	}
}

func lifecycleStatusFromExecution(execution sandbox.Execution) LifecycleStatus {
	if execution.Status == sandbox.ExecutionStatusUnsupported || execution.Decision.SelectionOutcome == sandbox.BackendSelectionOutcomeUnsupported {
		return LifecycleStatusUnsupported
	}
	if execution.Result.ErrorCode == "sandbox_profile_not_found" || execution.Result.ErrorClass == sandbox.ErrorClassInvalidProfile {
		return LifecycleStatusDenied
	}
	return LifecycleStatusDenied
}

func classifyExecutionFailure(execution sandbox.Execution) string {
	switch execution.Result.ErrorClass {
	case sandbox.ErrorClassLaunchFailed:
		return "launch_failed"
	case sandbox.ErrorClassTimeout:
		return "timeout"
	case sandbox.ErrorClassCancelled:
		return "cancelled"
	case sandbox.ErrorClassProcessFailed:
		return "transport_runtime_failure"
	case sandbox.ErrorClassPolicyDenied, sandbox.ErrorClassApprovalRequired, sandbox.ErrorClassApprovalRejected, sandbox.ErrorClassInvalidProfile:
		return "policy_denied"
	default:
		if execution.Status == sandbox.ExecutionStatusDenied {
			return "policy_denied"
		}
		return strings.TrimSpace(string(execution.Result.ErrorClass))
	}
}

func restartBackoffDelay(failureCount int) time.Duration {
	if failureCount <= 0 {
		return 5 * time.Second
	}
	delay := 5 * time.Second
	for i := 1; i < failureCount; i++ {
		delay *= 2
		if delay >= 5*time.Minute {
			return 5 * time.Minute
		}
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func mergeCatalogInstallInput(entry CatalogEntry, input CatalogInstallInput, method InstallMethod, environment config.Environment) CreateServerInput {
	spec := entry.DefaultInstallSpec
	serverID := strings.TrimSpace(input.ServerID)
	if serverID == "" {
		serverID = entry.ID
	}
	spec.ServerID = serverID
	spec.OriginKind = OriginKindCatalog
	spec.CatalogEntryID = entry.ID
	spec.InstallMethod = method
	spec.EnvironmentScope = string(environment)
	if displayName := strings.TrimSpace(input.DisplayName); displayName != "" {
		spec.DisplayName = displayName
	}
	if input.Enabled != nil {
		spec.Enabled = *input.Enabled
	}
	if profileID := strings.TrimSpace(input.SandboxProfileID); profileID != "" {
		spec.SandboxProfileID = profileID
	}
	if command := strings.TrimSpace(input.Command); command != "" {
		spec.Command = command
	}
	if input.Args != nil {
		spec.Args = cloneStrings(input.Args)
	}
	if endpoint := strings.TrimSpace(input.Endpoint); endpoint != "" {
		spec.Endpoint = endpoint
	}
	if workingDir := strings.TrimSpace(input.WorkingDir); workingDir != "" {
		spec.WorkingDir = workingDir
	}
	if input.SecretRefs != nil {
		spec.SecretRefs = cleanStrings(input.SecretRefs)
	}
	return spec
}

func (m *Manager) publishAuditEvent(ctx context.Context, name string, resource events.Resource, payload map[string]any) (events.Event, error) {
	if m.eventBus == nil && m.store == nil {
		return events.Event{}, nil
	}
	event := events.Event{
		EventID:    fmt.Sprintf("evt_%s_%d", strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), ":", "_"), time.Now().UTC().UnixNano()),
		Category:   "mcp",
		Name:       name,
		OccurredAt: time.Now().UTC(),
		Resource:   resource,
		Payload:    payload,
	}
	if m.store != nil {
		persisted, err := m.store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, err
		}
		event = persisted
	}
	if m.eventBus != nil {
		m.eventBus.Publish(event)
	}
	return event, nil
}

func (m *Manager) redactValue(ctx context.Context, server Server, value any) any {
	secrets, err := m.resolveSecretValues(ctx, server.SecretRefs)
	if err != nil || len(secrets) == 0 {
		return value
	}
	switch typed := value.(type) {
	case string:
		return redactString(typed, secrets)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, m.redactValue(ctx, server, item))
		}
		return items
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, item := range typed {
			cloned[key] = m.redactValue(ctx, server, item)
		}
		return cloned
	default:
		return value
	}
}

func redactString(input string, secrets map[string]string) string {
	redacted := input
	for _, value := range secrets {
		for _, candidate := range redactionCandidates(value) {
			redacted = strings.ReplaceAll(redacted, candidate, "[REDACTED]")
		}
	}
	return redacted
}

func catalogInstallConflictReason(existing Server, entryID string) (string, bool) {
	if existing.OriginKind != OriginKindCatalog {
		return "server id is already owned by a manual MCP server", true
	}
	if existing.CatalogEntryID != entryID {
		return fmt.Sprintf("server id is already owned by catalog entry %s", existing.CatalogEntryID), true
	}
	if existing.OperatorModified {
		return "existing installed server has operator modifications", true
	}
	return "", false
}

func redactionCandidates(secret string) []string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		seen[value] = struct{}{}
	}
	add(trimmed)
	add(url.QueryEscape(trimmed))
	add(base64.StdEncoding.EncodeToString([]byte(trimmed)))
	add(base64.RawStdEncoding.EncodeToString([]byte(trimmed)))
	add(base64.URLEncoding.EncodeToString([]byte(trimmed)))
	add(base64.RawURLEncoding.EncodeToString([]byte(trimmed)))
	add(hex.EncodeToString([]byte(trimmed)))
	add(strings.ToUpper(hex.EncodeToString([]byte(trimmed))))

	items := make([]string, 0, len(seen))
	for value := range seen {
		items = append(items, value)
	}
	return items
}

func stringFromMap(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	if value, ok := input[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func sessionID(active *sessionState) string {
	if active == nil {
		return ""
	}
	return strings.TrimSpace(active.sessionID)
}

func (m *Manager) buildSecretSummaries(server Server) []SecretSummary {
	if len(server.SecretRefs) == 0 {
		return nil
	}
	items := make([]SecretSummary, 0, len(server.SecretRefs))
	environmentScope := "both"
	switch m.cfg.Environment {
	case config.EnvironmentTest:
		environmentScope = "test"
	case config.EnvironmentProd:
		environmentScope = "prod"
	}
	resolutionByRef := map[string]string{}
	if bindings, err := m.listSecretBindings(context.Background(), server); err == nil {
		for _, item := range bindings {
			resolutionByRef[item.SecretRef] = string(item.Resolution)
		}
	}
	for _, secretRef := range server.SecretRefs {
		resolution := firstNonEmpty(resolutionByRef[secretRef], "unavailable")
		items = append(items, SecretSummary{
			ConsumerID:       server.ServerID,
			SecretRef:        secretRef,
			EnvironmentScope: environmentScope,
			DefaultRuleID:    "mcp_server:" + server.ServerID,
			Resolution:       resolution,
			DeliveryKind:     "environment_variable",
			RedactionRule:    "value_redacted",
		})
	}
	return items
}

func secretResolution(items []sandbox.SecretScopeOutcome) sandbox.SecretResolution {
	if len(items) == 0 {
		return sandbox.SecretResolutionNotApplicable
	}
	for _, item := range items {
		if item.Resolution == sandbox.SecretResolutionUnavailable {
			return sandbox.SecretResolutionUnavailable
		}
		if item.Resolution == sandbox.SecretResolutionDenied {
			return sandbox.SecretResolutionDenied
		}
	}
	return sandbox.SecretResolutionResolved
}

func cloneServer(server Server) Server {
	server.Args = cloneStrings(server.Args)
	server.SecretRefs = cloneStrings(server.SecretRefs)
	server.Declaration = cloneDeclaration(server.Declaration)
	server.WebsocketConfig = cloneWebsocketConfig(server.WebsocketConfig)
	server.ResolvedWebsocketHeaders = cloneStringMap(server.ResolvedWebsocketHeaders)
	server.CatalogManagement = cloneCatalogManagement(server.CatalogManagement)
	return server
}

func cloneServerState(state ServerState) ServerState {
	return state
}

func cloneTool(tool Tool) Tool {
	return tool
}

func cloneToolExposureRule(rule ToolExposureRule) ToolExposureRule {
	return rule
}

func cloneDeclaration(declaration Declaration) Declaration {
	declaration.AllowedBackendKinds = cloneBackendKinds(declaration.AllowedBackendKinds)
	declaration.ReadRoots = cloneStrings(declaration.ReadRoots)
	declaration.WriteRoots = cloneStrings(declaration.WriteRoots)
	declaration.AllowedHosts = cloneStrings(declaration.AllowedHosts)
	declaration.AllowedPorts = cloneInts(declaration.AllowedPorts)
	return declaration
}

func cloneDeclarationPtr(declaration Declaration) *Declaration {
	cloned := cloneDeclaration(declaration)
	return &cloned
}

func cloneCatalogManagement(management *CatalogManagement) *CatalogManagement {
	if management == nil {
		return nil
	}
	cloned := *management
	cloned.InstallInputSnapshot = cloneCatalogInstallSnapshot(management.InstallInputSnapshot)
	cloned.LastRevalidation = cloneRevalidationSnapshot(management.LastRevalidation)
	return &cloned
}

func cloneCatalogInstallSnapshot(snapshot CatalogInstallSnapshot) CatalogInstallSnapshot {
	snapshot.Args = cloneStrings(snapshot.Args)
	snapshot.SecretRefs = cleanStrings(snapshot.SecretRefs)
	snapshot.Enabled = cloneBoolPtr(snapshot.Enabled)
	return snapshot
}

func cloneWebsocketConfig(config *WebsocketConfig) *WebsocketConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	cloned.Subprotocols = cloneStrings(config.Subprotocols)
	cloned.Auth = cloneWebsocketAuthConfig(config.Auth)
	return &cloned
}

func cloneWebsocketAuthConfig(config *WebsocketAuthConfig) *WebsocketAuthConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	return &cloned
}

func cloneRevalidationSnapshot(snapshot *RevalidationSnapshot) *RevalidationSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	if len(snapshot.Issues) > 0 {
		cloned.Issues = append([]RevalidationIssue(nil), snapshot.Issues...)
	}
	return &cloned
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(items))
	for key, value := range items {
		cloned[key] = value
	}
	return cloned
}

func cloneInts(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	return append([]int(nil), items...)
}

func cloneBackendKinds(items []sandbox.BackendKind) []sandbox.BackendKind {
	if len(items) == 0 {
		return nil
	}
	return append([]sandbox.BackendKind(nil), items...)
}

func cloneToolMap(items map[string]Tool) []Tool {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Tool, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneTool(item))
	}
	return cloned
}

func consumerViewMap(view *sandbox.ConsumerContractView) map[string]any {
	if view == nil {
		return nil
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

func cleanStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func optionalSingleRoot(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return []string{root}
}

func defaultDeclaration() Declaration {
	return Declaration{
		ExecutionMode:               sandbox.ExecutionModeSubprocess,
		AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
		NetworkMode:                 sandbox.NetworkModeDeny,
		ApprovalMode:                sandbox.ApprovalModeAllow,
		RequiredEnforcementStrength: "declared_only",
		Active:                      true,
	}
}

func normalizeDeclaration(declaration Declaration) Declaration {
	if declaration.ExecutionMode == "" {
		declaration.ExecutionMode = sandbox.ExecutionModeSubprocess
	}
	if len(declaration.AllowedBackendKinds) == 0 {
		declaration.AllowedBackendKinds = []sandbox.BackendKind{sandbox.BackendKindSubprocess}
	}
	if declaration.NetworkMode == "" {
		declaration.NetworkMode = sandbox.NetworkModeDeny
	}
	if declaration.ApprovalMode == "" {
		declaration.ApprovalMode = sandbox.ApprovalModeAllow
	}
	if strings.TrimSpace(declaration.RequiredEnforcementStrength) == "" {
		declaration.RequiredEnforcementStrength = "declared_only"
	}
	if !declaration.Active {
		declaration.Active = false
	}
	return cloneDeclaration(declaration)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
