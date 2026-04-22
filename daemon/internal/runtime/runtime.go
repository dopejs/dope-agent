package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

var (
	ErrEntrypointRequired    = errors.New("entrypoint is required")
	ErrRunNotFound           = errors.New("run not found")
	ErrTitleRequired         = errors.New("title is required")
	ErrStepNotFound          = errors.New("step not found")
	ErrInvalidStepTransition = errors.New("invalid step transition")
	ErrRunTerminal           = errors.New("run is in a terminal state")
	ErrStepTerminal          = errors.New("step is in a terminal state")
	ErrToolNameRequired      = errors.New("tool name is required")
	ErrToolTargetRequired    = errors.New("capability id or skill id is required")
	ErrToolCallNotFound      = errors.New("tool call not found")
	ErrInvalidToolCallStatus = errors.New("invalid tool call status transition")
)

type RunStatus string

const (
	RunStatusQueued       RunStatus = "queued"
	RunStatusRunning      RunStatus = "running"
	RunStatusWaitingInput RunStatus = "waiting_input"
	RunStatusBlocked      RunStatus = "blocked"
	RunStatusCompleted    RunStatus = "completed"
	RunStatusFailed       RunStatus = "failed"
	RunStatusCancelled    RunStatus = "cancelled"
)

type Run struct {
	RunID             string    `json:"runId"`
	SessionID         string    `json:"sessionId,omitempty"`
	ScheduleID        string    `json:"scheduleId,omitempty"`
	ScheduleAttemptID string    `json:"scheduleAttemptId,omitempty"`
	Entrypoint        string    `json:"entrypoint"`
	Status            RunStatus `json:"status"`
	Goal              string    `json:"goal"`
	ActiveWorkflowID  string    `json:"activeWorkflowId,omitempty"`
	WorkflowCount     int       `json:"workflowCount,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type CreateRunInput struct {
	SessionID         string `json:"sessionId"`
	ScheduleID        string `json:"scheduleId,omitempty"`
	ScheduleAttemptID string `json:"scheduleAttemptId,omitempty"`
	Entrypoint        string `json:"entrypoint"`
	Goal              string `json:"goal"`
}

type StepStatus string

const (
	StepStatusQueued        StepStatus = "queued"
	StepStatusPlanning      StepStatus = "planning"
	StepStatusCallingModel  StepStatus = "calling_model"
	StepStatusExecutingTool StepStatus = "executing_tool"
	StepStatusWaitingInput  StepStatus = "waiting_input"
	StepStatusBlocked       StepStatus = "blocked"
	StepStatusCompleted     StepStatus = "completed"
	StepStatusFailed        StepStatus = "failed"
	StepStatusCancelled     StepStatus = "cancelled"
)

type Step struct {
	StepID         string     `json:"stepId"`
	RunID          string     `json:"runId"`
	WorkflowID     string     `json:"workflowId,omitempty"`
	WorkflowStepID string     `json:"workflowStepId,omitempty"`
	Attempt        int        `json:"attempt,omitempty"`
	Title          string     `json:"title"`
	Kind           string     `json:"kind"`
	Status         StepStatus `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Input          any        `json:"input,omitempty"`
	Output         any        `json:"output,omitempty"`
}

type CreateStepInput struct {
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	WorkflowID     string `json:"workflowId,omitempty"`
	WorkflowStepID string `json:"workflowStepId,omitempty"`
	Attempt        int    `json:"attempt,omitempty"`
	Input          any    `json:"input"`
}

type UpdateStepStatusInput struct {
	Status StepStatus `json:"status"`
	Output any        `json:"output"`
}

type ToolCallStatus string

const (
	ToolCallStatusRequested ToolCallStatus = "requested"
	ToolCallStatusRunning   ToolCallStatus = "running"
	ToolCallStatusCompleted ToolCallStatus = "completed"
	ToolCallStatusFailed    ToolCallStatus = "failed"
	ToolCallStatusCancelled ToolCallStatus = "cancelled"
	ToolCallStatusDenied    ToolCallStatus = "denied"
)

type ToolCallInvocationKind string

const (
	ToolCallInvocationKindLocalTool ToolCallInvocationKind = "local_tool"
	ToolCallInvocationKindSkill     ToolCallInvocationKind = "skill"
	ToolCallInvocationKindMCPTool   ToolCallInvocationKind = "mcp_tool"
)

type ToolCall struct {
	ToolCallID           string                        `json:"toolCallId"`
	RunID                string                        `json:"runId"`
	StepID               string                        `json:"stepId"`
	WorkflowID           string                        `json:"workflowId,omitempty"`
	WorkflowStepID       string                        `json:"workflowStepId,omitempty"`
	Attempt              int                           `json:"attempt,omitempty"`
	ComputerUseSessionID string                        `json:"computerUseSessionId,omitempty"`
	ComputerUseActionID  string                        `json:"computerUseActionId,omitempty"`
	InvocationKind       ToolCallInvocationKind        `json:"invocationKind,omitempty"`
	CapabilityID         string                        `json:"capabilityId,omitempty"`
	SkillID              string                        `json:"skillId,omitempty"`
	MCPServerID          string                        `json:"mcpServerId,omitempty"`
	MCPServerName        string                        `json:"mcpServerName,omitempty"`
	MCPToolName          string                        `json:"mcpToolName,omitempty"`
	MCPTransportKind     string                        `json:"mcpTransportKind,omitempty"`
	MCPSessionID         string                        `json:"mcpSessionId,omitempty"`
	AuthorizationResult  string                        `json:"authorizationResult,omitempty"`
	ToolName             string                        `json:"toolName"`
	Status               ToolCallStatus                `json:"status"`
	SandboxExecutionID   string                        `json:"sandboxExecutionId,omitempty"`
	FailureClass         string                        `json:"failureClass,omitempty"`
	IntegrationBindings  []integrations.BindingSummary `json:"integrationBindings,omitempty"`
	CreatedAt            time.Time                     `json:"createdAt"`
	UpdatedAt            time.Time                     `json:"updatedAt"`
	Input                any                           `json:"input,omitempty"`
	Output               any                           `json:"output,omitempty"`
	Error                string                        `json:"error,omitempty"`
	Sandbox              map[string]any                `json:"sandbox,omitempty"`
}

type RunCheckpoint struct {
	Run        Run        `json:"run"`
	Steps      []Step     `json:"steps"`
	ToolCalls  []ToolCall `json:"toolCalls"`
	CapturedAt time.Time  `json:"capturedAt"`
}

type CreateToolCallInput struct {
	WorkflowID           string                        `json:"workflowId,omitempty"`
	WorkflowStepID       string                        `json:"workflowStepId,omitempty"`
	Attempt              int                           `json:"attempt,omitempty"`
	ComputerUseSessionID string                        `json:"computerUseSessionId,omitempty"`
	ComputerUseActionID  string                        `json:"computerUseActionId,omitempty"`
	InvocationKind       ToolCallInvocationKind        `json:"invocationKind,omitempty"`
	CapabilityID         string                        `json:"capabilityId"`
	SkillID              string                        `json:"skillId"`
	MCPServerID          string                        `json:"mcpServerId"`
	MCPServerName        string                        `json:"mcpServerName"`
	MCPToolName          string                        `json:"mcpToolName"`
	MCPTransportKind     string                        `json:"mcpTransportKind"`
	MCPSessionID         string                        `json:"mcpSessionId"`
	AuthorizationResult  string                        `json:"authorizationResult"`
	ToolName             string                        `json:"toolName"`
	Input                any                           `json:"input"`
	SandboxExecutionID   string                        `json:"sandboxExecutionId,omitempty"`
	FailureClass         string                        `json:"failureClass,omitempty"`
	Sandbox              map[string]any                `json:"sandbox,omitempty"`
	IntegrationBindings  []integrations.BindingSummary `json:"integrationBindings,omitempty"`
}

type CompleteToolCallInput struct {
	Output             any            `json:"output"`
	SandboxExecutionID string         `json:"sandboxExecutionId,omitempty"`
	Sandbox            map[string]any `json:"sandbox,omitempty"`
}

type FailToolCallInput struct {
	Output             any            `json:"output"`
	Error              string         `json:"error"`
	FailureClass       string         `json:"failureClass"`
	SandboxExecutionID string         `json:"sandboxExecutionId,omitempty"`
	Sandbox            map[string]any `json:"sandbox,omitempty"`
}

type DenyToolCallInput struct {
	Output             any            `json:"output"`
	Error              string         `json:"error"`
	FailureClass       string         `json:"failureClass"`
	SandboxExecutionID string         `json:"sandboxExecutionId,omitempty"`
	Sandbox            map[string]any `json:"sandbox,omitempty"`
}

type CancelToolCallInput struct {
	Output             any            `json:"output"`
	Error              string         `json:"error"`
	FailureClass       string         `json:"failureClass"`
	SandboxExecutionID string         `json:"sandboxExecutionId,omitempty"`
	Sandbox            map[string]any `json:"sandbox,omitempty"`
}

type Manager struct {
	mu              sync.RWMutex
	byID            map[string]Run
	runIDs          []string
	stepsByID       map[string]Step
	stepsByRun      map[string][]string
	toolCallsByID   map[string]ToolCall
	toolCallsByStep map[string][]string
}

func NewManager() *Manager {
	return &Manager{
		byID:            make(map[string]Run),
		stepsByID:       make(map[string]Step),
		stepsByRun:      make(map[string][]string),
		toolCallsByID:   make(map[string]ToolCall),
		toolCallsByStep: make(map[string][]string),
	}
}

func (m *Manager) CreateRun(input CreateRunInput) (Run, error) {
	if input.Entrypoint == "" {
		return Run{}, ErrEntrypointRequired
	}

	now := time.Now().UTC()
	run := Run{
		RunID:             newRunID(),
		SessionID:         input.SessionID,
		ScheduleID:        input.ScheduleID,
		ScheduleAttemptID: input.ScheduleAttemptID,
		Entrypoint:        input.Entrypoint,
		Status:            RunStatusQueued,
		Goal:              input.Goal,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.byID[run.RunID] = run
	m.runIDs = append(m.runIDs, run.RunID)

	return run, nil
}

func (m *Manager) SnapshotRun(runID string) (RunCheckpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	run, ok := m.byID[runID]
	if !ok {
		return RunCheckpoint{}, ErrRunNotFound
	}

	stepIDs := m.stepsByRun[runID]
	steps := make([]Step, 0, len(stepIDs))
	toolCalls := make([]ToolCall, 0)

	for _, stepID := range stepIDs {
		step := m.stepsByID[stepID]
		steps = append(steps, step)

		toolCallIDs := m.toolCallsByStep[stepID]
		for _, toolCallID := range toolCallIDs {
			toolCalls = append(toolCalls, m.toolCallsByID[toolCallID])
		}
	}

	return RunCheckpoint{
		Run:        run,
		Steps:      steps,
		ToolCalls:  toolCalls,
		CapturedAt: time.Now().UTC(),
	}, nil
}

func (m *Manager) ListRuns() []Run {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runs := make([]Run, 0, len(m.runIDs))
	for _, runID := range m.runIDs {
		runs = append(runs, m.byID[runID])
	}

	return runs
}

func (m *Manager) GetRun(runID string) (Run, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	run, ok := m.byID[runID]
	return run, ok
}

func IsRunTerminal(status RunStatus) bool {
	switch status {
	case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return true
	default:
		return false
	}
}

func IsStepTerminal(status StepStatus) bool {
	switch status {
	case StepStatusCompleted, StepStatusFailed, StepStatusCancelled:
		return true
	default:
		return false
	}
}

func (m *Manager) CreateStep(runID string, input CreateStepInput) (Step, error) {
	if input.Title == "" {
		return Step{}, ErrTitleRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byID[runID]; !ok {
		return Step{}, ErrRunNotFound
	}

	now := time.Now().UTC()
	kind := input.Kind
	if kind == "" {
		kind = "task"
	}

	step := Step{
		StepID:         newStepID(),
		RunID:          runID,
		WorkflowID:     strings.TrimSpace(input.WorkflowID),
		WorkflowStepID: strings.TrimSpace(input.WorkflowStepID),
		Attempt:        input.Attempt,
		Title:          input.Title,
		Kind:           kind,
		Status:         StepStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
		Input:          input.Input,
	}

	m.stepsByID[step.StepID] = step
	m.stepsByRun[runID] = append(m.stepsByRun[runID], step.StepID)

	return step, nil
}

func (m *Manager) ListSteps(runID string) ([]Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.byID[runID]; !ok {
		return nil, errors.New("run not found")
	}

	stepIDs := m.stepsByRun[runID]
	steps := make([]Step, 0, len(stepIDs))
	for _, stepID := range stepIDs {
		steps = append(steps, m.stepsByID[stepID])
	}

	return steps, nil
}

func (m *Manager) GetStep(runID, stepID string) (Step, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	step, ok := m.stepsByID[stepID]
	if !ok {
		return Step{}, false
	}
	if step.RunID != runID {
		return Step{}, false
	}

	return step, true
}

func (m *Manager) UpdateStepStatus(runID, stepID string, input UpdateStepStatusInput) (Step, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byID[runID]; !ok {
		return Step{}, ErrRunNotFound
	}

	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return Step{}, ErrStepNotFound
	}

	if !canTransition(step.Status, input.Status) {
		return Step{}, ErrInvalidStepTransition
	}

	step.Status = input.Status
	step.UpdatedAt = time.Now().UTC()
	if input.Output != nil {
		step.Output = input.Output
	}

	m.stepsByID[stepID] = step

	return step, nil
}

func (m *Manager) UpdateStepStatusAndReconcileRun(runID, stepID string, input UpdateStepStatusInput) (Step, *Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	run, ok := m.byID[runID]
	if !ok {
		return Step{}, nil, ErrRunNotFound
	}

	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return Step{}, nil, ErrStepNotFound
	}

	if !canTransition(step.Status, input.Status) {
		if step.Status == input.Status {
			return step, nil, nil
		}
		if IsStepTerminal(step.Status) {
			return Step{}, nil, ErrStepTerminal
		}
		if IsRunTerminal(run.Status) {
			return Step{}, nil, ErrRunTerminal
		}
		return Step{}, nil, ErrInvalidStepTransition
	}

	step.Status = input.Status
	step.UpdatedAt = time.Now().UTC()
	if input.Output != nil {
		step.Output = input.Output
	}
	m.stepsByID[stepID] = step

	nextRunStatus := m.deriveRunStatusLocked(runID)
	if nextRunStatus == run.Status {
		return step, nil, nil
	}

	run.Status = nextRunStatus
	run.UpdatedAt = time.Now().UTC()
	m.byID[runID] = run

	return step, &run, nil
}

func (m *Manager) CancelRun(runID string) (Run, []Step, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	run, ok := m.byID[runID]
	if !ok {
		return Run{}, nil, false, ErrRunNotFound
	}
	if run.Status == RunStatusCancelled {
		return run, nil, true, nil
	}
	if IsRunTerminal(run.Status) {
		return Run{}, nil, false, ErrRunTerminal
	}

	now := time.Now().UTC()
	updatedSteps := make([]Step, 0)
	for _, stepID := range m.stepsByRun[runID] {
		step := m.stepsByID[stepID]
		if IsStepTerminal(step.Status) {
			continue
		}
		step.Status = StepStatusCancelled
		step.UpdatedAt = now
		m.stepsByID[stepID] = step
		updatedSteps = append(updatedSteps, step)
	}

	run.Status = RunStatusCancelled
	run.UpdatedAt = now
	m.byID[runID] = run

	return run, updatedSteps, false, nil
}

func (m *Manager) ResumeRun(runID string) (Run, []Step, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	run, ok := m.byID[runID]
	if !ok {
		return Run{}, nil, false, ErrRunNotFound
	}
	if run.Status != RunStatusCancelled {
		if IsRunTerminal(run.Status) {
			return Run{}, nil, false, ErrRunTerminal
		}
		return run, nil, true, nil
	}

	now := time.Now().UTC()
	updatedSteps := make([]Step, 0)
	for _, stepID := range m.stepsByRun[runID] {
		step := m.stepsByID[stepID]
		if step.Status != StepStatusCancelled {
			continue
		}
		step.Status = StepStatusQueued
		step.UpdatedAt = now
		m.stepsByID[stepID] = step
		updatedSteps = append(updatedSteps, step)
	}

	run.Status = m.deriveRunStatusLocked(runID)
	run.UpdatedAt = now
	m.byID[runID] = run

	return run, updatedSteps, false, nil
}

func (m *Manager) CancelStep(runID, stepID string) (Step, *Run, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	run, ok := m.byID[runID]
	if !ok {
		return Step{}, nil, false, ErrRunNotFound
	}

	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return Step{}, nil, false, ErrStepNotFound
	}
	if step.Status == StepStatusCancelled {
		return step, nil, true, nil
	}
	if IsStepTerminal(step.Status) {
		return Step{}, nil, false, ErrStepTerminal
	}
	if IsRunTerminal(run.Status) && run.Status != RunStatusCancelled {
		return Step{}, nil, false, ErrRunTerminal
	}

	now := time.Now().UTC()
	step.Status = StepStatusCancelled
	step.UpdatedAt = now
	m.stepsByID[stepID] = step

	nextRunStatus := m.deriveRunStatusLocked(runID)
	if nextRunStatus == run.Status {
		return step, nil, false, nil
	}

	run.Status = nextRunStatus
	run.UpdatedAt = now
	m.byID[runID] = run

	return step, &run, false, nil
}

func (m *Manager) CreateToolCall(runID, stepID string, input CreateToolCallInput) (ToolCall, error) {
	if input.ToolName == "" {
		return ToolCall{}, ErrToolNameRequired
	}
	if strings.TrimSpace(input.CapabilityID) == "" && strings.TrimSpace(input.SkillID) == "" && strings.TrimSpace(input.MCPServerID) == "" {
		return ToolCall{}, ErrToolTargetRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byID[runID]; !ok {
		return ToolCall{}, ErrRunNotFound
	}
	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return ToolCall{}, ErrStepNotFound
	}

	now := time.Now().UTC()
	invocationKind := input.InvocationKind
	switch {
	case invocationKind != "":
	case strings.TrimSpace(input.SkillID) != "":
		invocationKind = ToolCallInvocationKindSkill
	case strings.TrimSpace(input.MCPServerID) != "":
		invocationKind = ToolCallInvocationKindMCPTool
	default:
		invocationKind = ToolCallInvocationKindLocalTool
	}
	toolCall := ToolCall{
		ToolCallID:           newToolCallID(),
		RunID:                runID,
		StepID:               stepID,
		WorkflowID:           strings.TrimSpace(input.WorkflowID),
		WorkflowStepID:       strings.TrimSpace(input.WorkflowStepID),
		Attempt:              input.Attempt,
		ComputerUseSessionID: strings.TrimSpace(input.ComputerUseSessionID),
		ComputerUseActionID:  strings.TrimSpace(input.ComputerUseActionID),
		InvocationKind:       invocationKind,
		CapabilityID:         strings.TrimSpace(input.CapabilityID),
		SkillID:              strings.TrimSpace(input.SkillID),
		MCPServerID:          strings.TrimSpace(input.MCPServerID),
		MCPServerName:        strings.TrimSpace(input.MCPServerName),
		MCPToolName:          strings.TrimSpace(input.MCPToolName),
		MCPTransportKind:     strings.TrimSpace(input.MCPTransportKind),
		MCPSessionID:         strings.TrimSpace(input.MCPSessionID),
		AuthorizationResult:  strings.TrimSpace(input.AuthorizationResult),
		ToolName:             input.ToolName,
		Status:               ToolCallStatusRequested,
		SandboxExecutionID:   strings.TrimSpace(input.SandboxExecutionID),
		FailureClass:         strings.TrimSpace(input.FailureClass),
		IntegrationBindings:  integrations.CloneBindingSummaries(input.IntegrationBindings),
		CreatedAt:            now,
		UpdatedAt:            now,
		Input:                input.Input,
		Sandbox:              cloneAnyMap(input.Sandbox),
	}

	m.toolCallsByID[toolCall.ToolCallID] = toolCall
	m.toolCallsByStep[stepID] = append(m.toolCallsByStep[stepID], toolCall.ToolCallID)

	return toolCall, nil
}

func cloneAnyMap(view map[string]any) map[string]any {
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

func (m *Manager) ListToolCalls(runID, stepID string) ([]ToolCall, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.byID[runID]; !ok {
		return nil, ErrRunNotFound
	}
	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return nil, ErrStepNotFound
	}

	ids := m.toolCallsByStep[stepID]
	toolCalls := make([]ToolCall, 0, len(ids))
	for _, id := range ids {
		toolCalls = append(toolCalls, m.toolCallsByID[id])
	}

	return toolCalls, nil
}

func (m *Manager) GetToolCall(runID, stepID, toolCallID string) (ToolCall, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	toolCall, ok := m.toolCallsByID[toolCallID]
	if !ok {
		return ToolCall{}, false
	}
	if toolCall.RunID != runID || toolCall.StepID != stepID {
		return ToolCall{}, false
	}

	return toolCall, true
}

func (m *Manager) CompleteToolCall(runID, stepID, toolCallID string, input CompleteToolCallInput) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolCall, err := m.requireMutableToolCall(runID, stepID, toolCallID)
	if err != nil {
		return ToolCall{}, err
	}
	if toolCall.Status != ToolCallStatusRequested && toolCall.Status != ToolCallStatusRunning {
		return ToolCall{}, ErrInvalidToolCallStatus
	}

	toolCall.Status = ToolCallStatusCompleted
	toolCall.UpdatedAt = time.Now().UTC()
	toolCall.Output = input.Output
	if trimmed := strings.TrimSpace(input.SandboxExecutionID); trimmed != "" {
		toolCall.SandboxExecutionID = trimmed
	}
	if input.Sandbox != nil {
		toolCall.Sandbox = cloneAnyMap(input.Sandbox)
	}
	m.toolCallsByID[toolCallID] = toolCall

	return toolCall, nil
}

func (m *Manager) FailToolCall(runID, stepID, toolCallID string, input FailToolCallInput) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolCall, err := m.requireMutableToolCall(runID, stepID, toolCallID)
	if err != nil {
		return ToolCall{}, err
	}
	if toolCall.Status != ToolCallStatusRequested && toolCall.Status != ToolCallStatusRunning {
		return ToolCall{}, ErrInvalidToolCallStatus
	}

	toolCall.Status = ToolCallStatusFailed
	toolCall.UpdatedAt = time.Now().UTC()
	toolCall.Output = input.Output
	toolCall.Error = input.Error
	toolCall.FailureClass = strings.TrimSpace(input.FailureClass)
	if trimmed := strings.TrimSpace(input.SandboxExecutionID); trimmed != "" {
		toolCall.SandboxExecutionID = trimmed
	}
	if input.Sandbox != nil {
		toolCall.Sandbox = cloneAnyMap(input.Sandbox)
	}
	m.toolCallsByID[toolCallID] = toolCall

	return toolCall, nil
}

func (m *Manager) DenyToolCall(runID, stepID, toolCallID string, input DenyToolCallInput) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolCall, err := m.requireMutableToolCall(runID, stepID, toolCallID)
	if err != nil {
		return ToolCall{}, err
	}
	if toolCall.Status != ToolCallStatusRequested && toolCall.Status != ToolCallStatusRunning {
		return ToolCall{}, ErrInvalidToolCallStatus
	}

	toolCall.Status = ToolCallStatusDenied
	toolCall.UpdatedAt = time.Now().UTC()
	toolCall.Output = input.Output
	toolCall.Error = input.Error
	toolCall.FailureClass = strings.TrimSpace(input.FailureClass)
	if trimmed := strings.TrimSpace(input.SandboxExecutionID); trimmed != "" {
		toolCall.SandboxExecutionID = trimmed
	}
	if input.Sandbox != nil {
		toolCall.Sandbox = cloneAnyMap(input.Sandbox)
	}
	m.toolCallsByID[toolCallID] = toolCall

	return toolCall, nil
}

func (m *Manager) CancelToolCall(runID, stepID, toolCallID string, input CancelToolCallInput) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolCall, err := m.requireMutableToolCall(runID, stepID, toolCallID)
	if err != nil {
		return ToolCall{}, err
	}
	if toolCall.Status != ToolCallStatusRequested && toolCall.Status != ToolCallStatusRunning {
		return ToolCall{}, ErrInvalidToolCallStatus
	}

	toolCall.Status = ToolCallStatusCancelled
	toolCall.UpdatedAt = time.Now().UTC()
	toolCall.Output = input.Output
	toolCall.Error = input.Error
	toolCall.FailureClass = strings.TrimSpace(input.FailureClass)
	if trimmed := strings.TrimSpace(input.SandboxExecutionID); trimmed != "" {
		toolCall.SandboxExecutionID = trimmed
	}
	if input.Sandbox != nil {
		toolCall.Sandbox = cloneAnyMap(input.Sandbox)
	}
	m.toolCallsByID[toolCallID] = toolCall

	return toolCall, nil
}

func (m *Manager) MarkToolCallRunning(runID, stepID, toolCallID, sandboxExecutionID string, sandboxView map[string]any) (ToolCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolCall, err := m.requireMutableToolCall(runID, stepID, toolCallID)
	if err != nil {
		return ToolCall{}, err
	}
	if toolCall.Status != ToolCallStatusRequested {
		return ToolCall{}, ErrInvalidToolCallStatus
	}

	toolCall.Status = ToolCallStatusRunning
	toolCall.UpdatedAt = time.Now().UTC()
	if trimmed := strings.TrimSpace(sandboxExecutionID); trimmed != "" {
		toolCall.SandboxExecutionID = trimmed
	}
	if sandboxView != nil {
		toolCall.Sandbox = cloneAnyMap(sandboxView)
	}
	m.toolCallsByID[toolCallID] = toolCall

	return toolCall, nil
}

func (m *Manager) RestoreCheckpoints(checkpoints []RunCheckpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.byID = make(map[string]Run, len(checkpoints))
	m.runIDs = make([]string, 0, len(checkpoints))
	m.stepsByID = make(map[string]Step)
	m.stepsByRun = make(map[string][]string, len(checkpoints))
	m.toolCallsByID = make(map[string]ToolCall)
	m.toolCallsByStep = make(map[string][]string)

	for _, checkpoint := range checkpoints {
		run := checkpoint.Run
		m.byID[run.RunID] = run
		m.runIDs = append(m.runIDs, run.RunID)

		for _, step := range checkpoint.Steps {
			m.stepsByID[step.StepID] = step
			m.stepsByRun[run.RunID] = append(m.stepsByRun[run.RunID], step.StepID)
		}

		for _, toolCall := range checkpoint.ToolCalls {
			m.toolCallsByID[toolCall.ToolCallID] = toolCall
			m.toolCallsByStep[toolCall.StepID] = append(m.toolCallsByStep[toolCall.StepID], toolCall.ToolCallID)
		}
	}
}

func (m *Manager) RestoreRunCheckpoint(checkpoint RunCheckpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	run := checkpoint.Run
	if _, exists := m.byID[run.RunID]; !exists {
		m.runIDs = append(m.runIDs, run.RunID)
	}
	m.byID[run.RunID] = run

	existingStepIDs := m.stepsByRun[run.RunID]
	for _, stepID := range existingStepIDs {
		delete(m.stepsByID, stepID)
		delete(m.toolCallsByStep, stepID)
	}
	delete(m.stepsByRun, run.RunID)

	for toolCallID, toolCall := range m.toolCallsByID {
		if toolCall.RunID == run.RunID {
			delete(m.toolCallsByID, toolCallID)
		}
	}

	for _, step := range checkpoint.Steps {
		m.stepsByID[step.StepID] = step
		m.stepsByRun[run.RunID] = append(m.stepsByRun[run.RunID], step.StepID)
	}

	for _, toolCall := range checkpoint.ToolCalls {
		m.toolCallsByID[toolCall.ToolCallID] = toolCall
		m.toolCallsByStep[toolCall.StepID] = append(m.toolCallsByStep[toolCall.StepID], toolCall.ToolCallID)
	}
}

func (m *Manager) requireMutableToolCall(runID, stepID, toolCallID string) (ToolCall, error) {
	if _, ok := m.byID[runID]; !ok {
		return ToolCall{}, ErrRunNotFound
	}
	step, ok := m.stepsByID[stepID]
	if !ok || step.RunID != runID {
		return ToolCall{}, ErrStepNotFound
	}
	toolCall, ok := m.toolCallsByID[toolCallID]
	if !ok || toolCall.RunID != runID || toolCall.StepID != stepID {
		return ToolCall{}, ErrToolCallNotFound
	}
	return toolCall, nil
}

func (m *Manager) deriveRunStatusLocked(runID string) RunStatus {
	stepIDs := m.stepsByRun[runID]
	if len(stepIDs) == 0 {
		return RunStatusQueued
	}

	var (
		hasPlanningOrExecution bool
		hasWaitingInput        bool
		hasBlocked             bool
		hasFailed              bool
		hasQueued              bool
		allCompleted           = true
		allCancelled           = true
	)

	for _, stepID := range stepIDs {
		step := m.stepsByID[stepID]

		switch step.Status {
		case StepStatusPlanning, StepStatusCallingModel, StepStatusExecutingTool:
			hasPlanningOrExecution = true
		case StepStatusWaitingInput:
			hasWaitingInput = true
		case StepStatusBlocked:
			hasBlocked = true
		case StepStatusFailed:
			hasFailed = true
		case StepStatusQueued:
			hasQueued = true
		}

		if step.Status != StepStatusCompleted {
			allCompleted = false
		}
		if step.Status != StepStatusCancelled {
			allCancelled = false
		}
	}

	switch {
	case hasFailed:
		return RunStatusFailed
	case allCompleted:
		return RunStatusCompleted
	case allCancelled:
		return RunStatusCancelled
	case hasBlocked:
		return RunStatusBlocked
	case hasWaitingInput:
		return RunStatusWaitingInput
	case hasPlanningOrExecution:
		return RunStatusRunning
	case hasQueued:
		return RunStatusQueued
	default:
		return RunStatusQueued
	}
}

func canTransition(from, to StepStatus) bool {
	allowed, ok := stepTransitions[from]
	if !ok {
		return false
	}

	_, ok = allowed[to]
	return ok
}

var stepTransitions = map[StepStatus]map[StepStatus]struct{}{
	StepStatusQueued: {
		StepStatusPlanning:  {},
		StepStatusCancelled: {},
	},
	StepStatusPlanning: {
		StepStatusCallingModel:  {},
		StepStatusExecutingTool: {},
		StepStatusWaitingInput:  {},
		StepStatusBlocked:       {},
		StepStatusFailed:        {},
		StepStatusCancelled:     {},
	},
	StepStatusCallingModel: {
		StepStatusPlanning:      {},
		StepStatusExecutingTool: {},
		StepStatusWaitingInput:  {},
		StepStatusBlocked:       {},
		StepStatusCompleted:     {},
		StepStatusFailed:        {},
		StepStatusCancelled:     {},
	},
	StepStatusExecutingTool: {
		StepStatusPlanning:     {},
		StepStatusWaitingInput: {},
		StepStatusBlocked:      {},
		StepStatusCompleted:    {},
		StepStatusFailed:       {},
		StepStatusCancelled:    {},
	},
	StepStatusWaitingInput: {
		StepStatusPlanning:  {},
		StepStatusCancelled: {},
		StepStatusFailed:    {},
	},
	StepStatusBlocked: {
		StepStatusPlanning:  {},
		StepStatusCancelled: {},
		StepStatusFailed:    {},
	},
	StepStatusCompleted: {},
	StepStatusFailed:    {},
	StepStatusCancelled: {},
}

func newRunID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "run_fallback"
	}

	return "run_" + hex.EncodeToString(buf)
}

func newStepID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "step_fallback"
	}

	return "step_" + hex.EncodeToString(buf)
}

func newToolCallID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "toolcall_fallback"
	}

	return "toolcall_" + hex.EncodeToString(buf)
}
