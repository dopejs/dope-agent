package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
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

const (
	resourceKindServer = "mcp_server"
	resourceKindTool   = "mcp_tool"
)

type attachedExecutionStarter interface {
	StartAttachedExecution(context.Context, sandbox.ExecutionRequest) (sandbox.Execution, *sandbox.AttachedExecution, error)
	CancelExecution(executionID string) (sandbox.Execution, bool, error)
	GetExecution(executionID string) (sandbox.Execution, bool)
	PersistConsumerView(context.Context, *sandbox.ConsumerContractView) error
	GetProfile(profileID string) (sandbox.Profile, bool)
}

type sessionState struct {
	executionID     string
	session         Session
	stopRequested   bool
	cancelRequested bool
}

type Manager struct {
	cfg       config.Config
	store     *store.SQLiteStore
	eventBus  *events.Bus
	policy    *policy.Engine
	sandboxes attachedExecutionStarter
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
		transport = NewStdioTransport()
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

func (m *Manager) GetServer(serverID string) (Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, ok := m.servers[strings.TrimSpace(serverID)]
	if !ok {
		return Server{}, false
	}
	return cloneServer(server), true
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
			Status:  ToolAuthorizationStatusAllowed,
			Tool:    resource,
			Sandbox: consumer,
			Message: "tool use is allowed",
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
			Status:   ToolAuthorizationStatusPending,
			Tool:     resource,
			Message:  "tool use requires approval",
			Approval: &approval,
			Decision: &decision,
			Sandbox:  consumer,
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
			Status:  ToolAuthorizationStatusAllowed,
			Tool:    resource,
			Message: "tool use is allowed by approval",
			Sandbox: consumer,
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
			Status:   ToolAuthorizationStatusRejected,
			Tool:     resource,
			Message:  "approval was rejected",
			Approval: &approval,
			Sandbox:  consumer,
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
			Status:   ToolAuthorizationStatusPending,
			Tool:     resource,
			Message:  "approval is still pending",
			Approval: &approval,
			Sandbox:  consumer,
		}, nil
	}
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
	if m.sandboxes == nil {
		return LifecycleResponse{}, ErrSandboxManagerMissing
	}

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
	state.UpdatedAt = time.Now().UTC()
	m.states[serverID] = state
	resource := m.buildServerResourceLocked(server)
	m.mu.Unlock()
	if err := m.persistState(ctx, state); err != nil {
		return LifecycleResponse{}, err
	}

	consumer, err := m.buildLifecycleConsumerView(ctx, server, firstNonEmpty(strings.TrimSpace(requestedBy), "mcp"))
	if err != nil {
		state = m.recordFailure(ctx, serverID, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
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
	request, err := m.buildExecutionRequest(ctx, server, consumer, "")
	if err != nil {
		state = m.recordFailure(ctx, serverID, state, LifecycleStatusDenied, err.Error(), "invalid_configuration")
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
	execution, attached, err := m.sandboxes.StartAttachedExecution(ctx, request)
	if err != nil {
		state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "launch_failed")
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:       LifecycleActionStart,
			Server:       resource,
			FailureClass: "launch_failed",
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}
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

	session, err := m.transport.Open(ctx, server, SessionPipes{
		Stdin:  attached.Stdin,
		Stdout: attached.Stdout,
		Stderr: attached.Stderr,
	})
	if err != nil {
		_, _, _ = m.sandboxes.CancelExecution(execution.ExecutionID)
		state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:       LifecycleActionStart,
			Server:       resource,
			ExecutionID:  execution.ExecutionID,
			FailureClass: "transport_runtime_failure",
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}

	tools, err := session.ListTools(ctx)
	if err != nil {
		_ = session.Close()
		_, _, _ = m.sandboxes.CancelExecution(execution.ExecutionID)
		state = m.recordFailure(ctx, serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
		resource, _ = m.GetServerResource(serverID)
		return LifecycleResponse{
			Action:       LifecycleActionStart,
			Server:       resource,
			ExecutionID:  execution.ExecutionID,
			FailureClass: "transport_runtime_failure",
			PreflightMs:  time.Since(startedAt).Milliseconds(),
		}, nil
	}

	now := time.Now().UTC()
	if state.LastStartedAt != nil {
		state.RestartCount++
	}
	state.Status = LifecycleStatusHealthy
	state.LastExecutionID = execution.ExecutionID
	state.LastStartedAt = &now
	state.LastHeartbeatAt = &now
	state.HealthReason = ""
	state.FailureCount = 0
	state.UpdatedAt = now

	m.mu.Lock()
	m.states[serverID] = state
	m.sessions[serverID] = &sessionState{
		executionID: execution.ExecutionID,
		session:     session,
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
		"executionId":   execution.ExecutionID,
		"toolCount":     len(tools),
		"transportKind": server.TransportKind,
	}); err != nil {
		return LifecycleResponse{}, err
	}
	if err := m.publishHealthChanged(ctx, serverID, state.Status, state.HealthReason); err != nil {
		return LifecycleResponse{}, err
	}

	go m.watchSession(serverID, execution.ExecutionID, session)

	resource, _ = m.GetServerResource(serverID)
	return LifecycleResponse{
		Action:       LifecycleActionStart,
		Server:       resource,
		ExecutionID:  execution.ExecutionID,
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
				ServerID:      serverID,
				Source:        SourceAPI,
				CreatedAt:     now,
				TransportKind: TransportKindStdio,
				Declaration:   defaultDeclaration(),
			}
			created = true
			m.serverIDs = append(m.serverIDs, serverID)
		}
		server.DisplayName = strings.TrimSpace(createInput.DisplayName)
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
		server.WorkingDir = strings.TrimSpace(createInput.WorkingDir)
		server.SecretRefs = cleanStrings(createInput.SecretRefs)
		server.AutoRestart = createInput.AutoRestart
		server.Source = SourceAPI
		server.UpdatedAt = now
	} else {
		existing, ok := m.servers[update.serverID]
		if !ok {
			return ServerResource{}, false, ErrServerNotFound
		}
		server = existing
		created = false
		if update.input.DisplayName != nil {
			server.DisplayName = strings.TrimSpace(*update.input.DisplayName)
		}
		if update.input.Enabled != nil {
			server.Enabled = *update.input.Enabled
		}
		if update.input.SandboxProfileID != nil {
			server.SandboxProfileID = strings.TrimSpace(*update.input.SandboxProfileID)
		}
		if update.input.DeclarationID != nil {
			server.DeclarationID = strings.TrimSpace(*update.input.DeclarationID)
		}
		if update.input.Declaration != nil {
			server.Declaration = normalizeDeclaration(*update.input.Declaration)
		}
		if update.input.TransportKind != nil {
			server.TransportKind = *update.input.TransportKind
		}
		if update.input.Command != nil {
			server.Command = strings.TrimSpace(*update.input.Command)
		}
		if update.input.Args != nil {
			server.Args = cloneStrings(update.input.Args)
		}
		if update.input.WorkingDir != nil {
			server.WorkingDir = strings.TrimSpace(*update.input.WorkingDir)
		}
		if update.input.SecretRefs != nil {
			server.SecretRefs = cleanStrings(update.input.SecretRefs)
		}
		if update.input.AutoRestart != nil {
			server.AutoRestart = *update.input.AutoRestart
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
		"serverId":         server.ServerID,
		"displayName":      server.DisplayName,
		"enabled":          server.Enabled,
		"sandboxProfileId": server.SandboxProfileID,
		"declarationId":    server.DeclarationID,
		"transportKind":    server.TransportKind,
		"created":          created,
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
	if active == nil || active.executionID == "" {
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
	m.mu.Unlock()

	execution, ok := m.sandboxes.GetExecution(executionID)
	if ok {
		state = m.updateStateFromExecution(context.Background(), serverID, state, execution, stopRequested || cancelRequested)
	} else if err != nil {
		state = m.recordFailure(context.Background(), serverID, state, LifecycleStatusFailed, err.Error(), "transport_runtime_failure")
	}

	if state.Status == LifecycleStatusFailed && server.Enabled && server.AutoRestart {
		m.scheduleRestart(serverID, state)
	}
}

func (m *Manager) scheduleRestart(serverID string, state ServerState) {
	delay := restartBackoffDelay(state.FailureCount)
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

func (m *Manager) updateStateFromExecution(ctx context.Context, serverID string, state ServerState, execution sandbox.Execution, requestedStop bool) ServerState {
	now := time.Now().UTC()
	state.LastExecutionID = execution.ExecutionID
	state.LastPolicyRecordID = ""
	if execution.Consumer != nil && execution.Consumer.PolicyRecord != nil {
		state.LastPolicyRecordID = execution.Consumer.PolicyRecord.PolicyRecordID
	}
	state.UpdatedAt = now
	switch execution.Status {
	case sandbox.ExecutionStatusDenied:
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

func (m *Manager) publishHealthChanged(ctx context.Context, serverID string, status LifecycleStatus, reason string) error {
	return m.publishEvent(ctx, "mcp", "mcp.server_health_changed", events.Resource{Kind: resourceKindServer, ID: serverID}, map[string]any{
		"serverId": serverID,
		"status":   status,
		"reason":   strings.TrimSpace(reason),
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

func (m *Manager) buildServerResourceLocked(server Server) ServerResource {
	state := cloneServerState(m.states[server.ServerID])
	toolCount := len(m.tools[server.ServerID])
	tools := make([]ToolResource, 0, toolCount)
	for _, tool := range m.tools[server.ServerID] {
		tools = append(tools, m.buildToolResourceLocked(server, tool))
	}
	return ServerResource{
		Server:        cloneServer(server),
		State:         state,
		SecretSummary: m.buildSecretSummaries(server),
		ToolCount:     toolCount,
		Tools:         tools,
	}
}

func (m *Manager) buildToolResourceLocked(server Server, tool Tool) ToolResource {
	ruleMap := m.exposure[server.ServerID][tool.ToolName]
	exposure := make([]ToolExposureRule, 0, len(ruleMap))
	approvalRequired := false
	effective := "unavailable"
	reason := ""
	for _, rule := range ruleMap {
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
			Resolution:       m.resolveSecretRef(secretRef, envScope),
		})
	}
	return items, nil
}

func (m *Manager) resolveSecretEnv(ctx context.Context, server Server) (map[string]string, error) {
	secretScope, err := m.buildSecretScope(ctx, server)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, item := range secretScope {
		if item.Resolution != sandbox.SecretResolutionResolved {
			continue
		}
		if value, ok := os.LookupEnv(item.SecretRef); ok {
			env[item.SecretRef] = value
		}
	}
	return env, nil
}

func (m *Manager) resolveSecretRef(secretRef string, envScope sandbox.SecretEnvironmentScope) sandbox.SecretResolution {
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
	if _, ok := os.LookupEnv(secretRef); !ok {
		return sandbox.SecretResolutionUnavailable
	}
	return sandbox.SecretResolutionResolved
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
	return m.store.UpsertMCPServer(ctx, store.MCPServerRecord{
		ServerID:  server.ServerID,
		Enabled:   server.Enabled,
		UpdatedAt: server.UpdatedAt,
		Document:  document,
	})
}

func (m *Manager) persistState(ctx context.Context, state ServerState) error {
	if m.store == nil {
		return nil
	}
	document, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal mcp server state %s: %w", state.ServerID, err)
	}
	return m.store.UpsertMCPServerState(ctx, store.MCPServerStateRecord{
		ServerID:  state.ServerID,
		Status:    string(state.Status),
		UpdatedAt: state.UpdatedAt,
		Document:  document,
	})
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
	if strings.TrimSpace(server.Command) == "" {
		return ErrCommandRequired
	}
	if server.TransportKind != TransportKindStdio {
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
	if execution.Result.ErrorCode == "sandbox_profile_not_found" || execution.Result.ErrorClass == sandbox.ErrorClassInvalidProfile {
		return LifecycleStatusDenied
	}
	if execution.Decision.Explanation == "sandbox declaration requires stronger guarantees than the current backend can provide" {
		return LifecycleStatusUnsupported
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

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
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
