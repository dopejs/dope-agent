package orchestration

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

type Manager struct{}

type MCPPlanningSource interface {
	ListServers() []MCPPlanningServer
	ListTools(serverID string) ([]MCPPlanningTool, error)
}

type MCPPlanningServer struct {
	ServerID string
	Tools    []MCPPlanningTool
}

type MCPPlanningTool struct {
	ToolName string
}

type SkillPlanningSource interface {
	ListSkills() []SkillPlanningCandidate
}

type SkillPlanningCandidate struct {
	SkillID              string
	ApprovalModeExpected string
	Executable           bool
	Available            bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Plan(cfg config.Config, run runtime.Run, input CreateWorkflowInput, capabilitySupervisor *capabilities.Supervisor, skillSource SkillPlanningSource, mcpSource MCPPlanningSource) Workflow {
	now := time.Now().UTC()
	goal := strings.TrimSpace(input.Goal)
	if goal == "" {
		goal = strings.TrimSpace(run.Goal)
	}
	workflow := Workflow{
		WorkflowID:       newWorkflowID(),
		RunID:            run.RunID,
		EnvironmentScope: string(cfg.Environment),
		Goal:             goal,
		Status:           WorkflowStatusPlanning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	mcpStep, hasMCP := pickMCPWorkflowStep(goal, mcpSource, now)
	skillStep, hasSkill := pickSkillWorkflowStep(goal, skillSource, now)
	localStep, hasLocal := pickLocalWorkflowStep(cfg, goal, capabilitySupervisor, now)

	switch {
	case hasMCP && hasSkill:
		workflow.PlanSummary = "Plan one MCP step followed by one executable skill handoff."
		workflow.Steps = []WorkflowStep{mcpStep, skillStep}
		workflow.Dependencies = []Dependency{{
			DependencyID:       newWorkflowDependencyID(),
			WorkflowID:         workflow.WorkflowID,
			FromWorkflowStepID: mcpStep.WorkflowStepID,
			ToWorkflowStepID:   skillStep.WorkflowStepID,
			DependencyType:     DependencyTypeSuccess,
			Reason:             "workflow consumes MCP output before local continuation",
		}}
		workflow.Handoffs = []Handoff{{
			HandoffID:          newWorkflowHandoffID(),
			WorkflowID:         workflow.WorkflowID,
			FromWorkflowStepID: mcpStep.WorkflowStepID,
			ToWorkflowStepID:   skillStep.WorkflowStepID,
			Status:             HandoffStatusPending,
			PayloadSummary:     "MCP lookup result summary",
			SourcePath:         "step.output.result",
		}}
	case hasSkill:
		workflow.PlanSummary = "Plan one executable skill step."
		workflow.Steps = []WorkflowStep{skillStep}
	case hasMCP:
		workflow.PlanSummary = "Plan one MCP tool step."
		workflow.Steps = []WorkflowStep{mcpStep}
	case hasLocal:
		workflow.PlanSummary = "Plan one local tool step."
		workflow.Steps = []WorkflowStep{localStep}
	default:
		workflow.Status = WorkflowStatusPlanningFailed
		workflow.FailureSummary = "No executable workflow consumers are available for the current daemon state."
		return workflow
	}

	workflow.Status = WorkflowStatusPlanned
	for idx := range workflow.Steps {
		workflow.Steps[idx].WorkflowID = workflow.WorkflowID
		workflow.Steps[idx].Position = idx + 1
		workflow.Steps[idx].Status = StepStatusPlanned
		workflow.Steps[idx].CreatedAt = now
		workflow.Steps[idx].UpdatedAt = now
		workflow.Steps[idx].MaxAttempts = max(1, workflow.Steps[idx].MaxAttempts)
	}
	for idx := range workflow.Dependencies {
		workflow.Dependencies[idx].WorkflowID = workflow.WorkflowID
	}
	for idx := range workflow.Handoffs {
		workflow.Handoffs[idx].WorkflowID = workflow.WorkflowID
	}
	for idx := range workflow.Steps {
		dependencyIDs := make([]string, 0)
		for _, dep := range workflow.Dependencies {
			if dep.ToWorkflowStepID == workflow.Steps[idx].WorkflowStepID {
				dependencyIDs = append(dependencyIDs, dep.DependencyID)
			}
		}
		workflow.Steps[idx].DependencyIDs = dependencyIDs
	}
	return workflow
}

func (m *Manager) InitializeExecution(workflow Workflow, now time.Time) Workflow {
	if workflow.StartedAt == nil {
		workflow.StartedAt = &now
	}
	workflow.Status = WorkflowStatusRunning
	workflow.UpdatedAt = now
	for idx := range workflow.Steps {
		if len(m.DependenciesMissing(workflow, workflow.Steps[idx])) == 0 {
			workflow.Steps[idx].Status = StepStatusReady
		} else {
			workflow.Steps[idx].Status = StepStatusWaitingDependency
		}
		workflow.Steps[idx].UpdatedAt = now
	}
	return workflow
}

func (m *Manager) AdvanceReadySteps(workflow Workflow, now time.Time) (Workflow, bool) {
	changed := false
	for idx := range workflow.Steps {
		if workflow.Steps[idx].Status != StepStatusPlanned && workflow.Steps[idx].Status != StepStatusWaitingDependency {
			continue
		}
		if len(m.DependenciesMissing(workflow, workflow.Steps[idx])) == 0 {
			if workflow.Steps[idx].Status != StepStatusReady {
				workflow.Steps[idx].Status = StepStatusReady
				workflow.Steps[idx].UpdatedAt = now
				changed = true
			}
			continue
		}
		if workflow.Steps[idx].Status != StepStatusWaitingDependency {
			workflow.Steps[idx].Status = StepStatusWaitingDependency
			workflow.Steps[idx].UpdatedAt = now
			changed = true
		}
	}
	if changed {
		workflow.UpdatedAt = now
	}
	return workflow, changed
}

func (m *Manager) StartStepAttempt(workflow Workflow, workflowStepID, runtimeStepID string, now time.Time) Workflow {
	for idx := range workflow.Steps {
		if workflow.Steps[idx].WorkflowStepID != workflowStepID {
			continue
		}
		workflow.Steps[idx].RuntimeStepID = runtimeStepID
		workflow.Steps[idx].AttemptCount++
		workflow.Steps[idx].Status = StepStatusRunning
		workflow.Steps[idx].UpdatedAt = now
		break
	}
	for idx := range workflow.Handoffs {
		if workflow.Handoffs[idx].ToWorkflowStepID == workflowStepID && workflow.Handoffs[idx].Status == HandoffStatusAvailable {
			workflow.Handoffs[idx].Status = HandoffStatusConsumed
			workflow.Handoffs[idx].ConsumedAt = &now
		}
	}
	workflow.UpdatedAt = now
	return workflow
}

func (m *Manager) BindToolCall(workflow Workflow, workflowStepID string, toolCall runtime.ToolCall, now time.Time) Workflow {
	for idx := range workflow.Steps {
		if workflow.Steps[idx].WorkflowStepID == workflowStepID {
			workflow.Steps[idx].ActiveToolCallID = toolCall.ToolCallID
			workflow.Steps[idx].RuntimeStepID = toolCall.StepID
			workflow.Steps[idx].UpdatedAt = now
			break
		}
	}
	workflow.UpdatedAt = now
	return workflow
}

func (m *Manager) ApplyToolCallResult(workflow Workflow, toolCall runtime.ToolCall, hintedStatus StepStatus, blockedReason string, now time.Time) Workflow {
	for idx := range workflow.Steps {
		if workflow.Steps[idx].WorkflowStepID != toolCall.WorkflowStepID {
			continue
		}
		step := &workflow.Steps[idx]
		step.ActiveToolCallID = toolCall.ToolCallID
		step.RuntimeStepID = toolCall.StepID
		step.UpdatedAt = now
		switch toolCall.Status {
		case runtime.ToolCallStatusCompleted:
			step.Status = StepStatusCompleted
			step.SideEffectsVisible = true
			step.OutputSummary = SummarizeOutput(toolCall.Output)
			step.BlockedReason = ""
			for handoffIdx := range workflow.Handoffs {
				if workflow.Handoffs[handoffIdx].FromWorkflowStepID == step.WorkflowStepID {
					workflow.Handoffs[handoffIdx].Status = HandoffStatusAvailable
				}
			}
		case runtime.ToolCallStatusDenied:
			step.Status = StepStatusBlocked
			step.BlockedReason = firstNonEmpty(blockedReason, string(BlockedReasonApprovalDenied))
		case runtime.ToolCallStatusCancelled:
			step.Status = StepStatusCancelled
		case runtime.ToolCallStatusFailed:
			step.LastFailureClass = toolCall.FailureClass
			switch {
			case toolCall.FailureClass == "approval_rejected" || strings.Contains(toolCall.FailureClass, "approval"):
				step.Status = StepStatusBlocked
				step.BlockedReason = firstNonEmpty(blockedReason, string(BlockedReasonApprovalDenied))
			case toolCall.FailureClass == "consumer_unavailable":
				step.Status = StepStatusBlocked
				step.BlockedReason = string(BlockedReasonConsumerUnavailable)
			case step.AttemptCount < step.MaxAttempts:
				step.Status = StepStatusReady
				step.ActiveToolCallID = ""
				step.BlockedReason = ""
			default:
				step.Status = StepStatusFailed
			}
		default:
			if hintedStatus != "" {
				step.Status = hintedStatus
			}
		}
		break
	}
	return m.ReconcileStatus(workflow, now)
}

func (m *Manager) ReconcileStatus(workflow Workflow, now time.Time) Workflow {
	var (
		hasRunning   bool
		hasBlocked   bool
		hasFailed    bool
		hasCancelled bool
		allComplete  = len(workflow.Steps) > 0
		sideEffects  bool
	)
	for _, step := range workflow.Steps {
		if step.SideEffectsVisible {
			sideEffects = true
		}
		switch step.Status {
		case StepStatusReady, StepStatusWaitingDependency, StepStatusRunning:
			hasRunning = true
			allComplete = false
		case StepStatusBlocked:
			hasBlocked = true
			allComplete = false
		case StepStatusFailed:
			hasFailed = true
			allComplete = false
		case StepStatusCancelled:
			hasCancelled = true
			allComplete = false
		case StepStatusCompleted, StepStatusSkipped:
		default:
			allComplete = false
		}
	}
	switch {
	case workflow.Status == WorkflowStatusCancelled:
	case workflow.Status == WorkflowStatusInterrupted:
	case hasRunning:
		workflow.Status = WorkflowStatusRunning
	case hasBlocked:
		workflow.Status = WorkflowStatusBlocked
	case hasFailed && sideEffects:
		workflow.Status = WorkflowStatusPartialFailed
		workflow.CompletedAt = &now
	case hasFailed:
		workflow.Status = WorkflowStatusFailed
		workflow.CompletedAt = &now
	case hasCancelled:
		workflow.Status = WorkflowStatusCancelled
		workflow.CompletedAt = &now
	case allComplete:
		workflow.Status = WorkflowStatusCompleted
		workflow.CompletedAt = &now
	default:
		workflow.Status = WorkflowStatusRunning
	}
	workflow.UpdatedAt = now
	return workflow
}

func (m *Manager) DependenciesMissing(workflow Workflow, step WorkflowStep) []string {
	missing := make([]string, 0)
	for _, dependency := range workflow.Dependencies {
		if dependency.ToWorkflowStepID != step.WorkflowStepID {
			continue
		}
		from := WorkflowStepByID(workflow, dependency.FromWorkflowStepID)
		if from == nil {
			missing = append(missing, dependency.DependencyID)
			continue
		}
		switch dependency.DependencyType {
		case DependencyTypeSuccess:
			if from.Status != StepStatusCompleted {
				missing = append(missing, dependency.DependencyID)
			}
		case DependencyTypeFailure:
			if from.Status != StepStatusFailed {
				missing = append(missing, dependency.DependencyID)
			}
		default:
			if !IsTerminalStepStatus(from.Status) {
				missing = append(missing, dependency.DependencyID)
			}
		}
	}
	return missing
}

func WorkflowStepByID(workflow Workflow, workflowStepID string) *WorkflowStep {
	for idx := range workflow.Steps {
		if workflow.Steps[idx].WorkflowStepID == workflowStepID {
			return &workflow.Steps[idx]
		}
	}
	return nil
}

func SummarizeOutput(value any) string {
	if value == nil {
		return ""
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	text := string(payload)
	if len(text) > 160 {
		return text[:160]
	}
	return text
}

func pickMCPWorkflowStep(goal string, mcpSource MCPPlanningSource, now time.Time) (WorkflowStep, bool) {
	if mcpSource == nil {
		return WorkflowStep{}, false
	}
	servers := mcpSource.ListServers()
	for _, server := range servers {
		tools := server.Tools
		if len(tools) == 0 {
			listed, err := mcpSource.ListTools(server.ServerID)
			if err == nil {
				tools = listed
			}
		}
		if len(tools) == 0 {
			continue
		}
		toolName := strings.TrimSpace(tools[0].ToolName)
		if toolName == "" {
			toolName = "lookup"
		}
		return WorkflowStep{
			WorkflowStepID:       newWorkflowStepID(),
			Title:                "Use MCP tool " + toolName,
			ConsumerKind:         string(runtime.ToolCallInvocationKindMCPTool),
			ConsumerID:           server.ServerID,
			ToolName:             toolName,
			Input:                map[string]any{"query": goal},
			SelectionRationale:   "Selected the first available MCP tool to satisfy the goal through the existing MCP runtime plane.",
			ApprovalModeExpected: "allow",
			MaxAttempts:          1,
			CreatedAt:            now,
			UpdatedAt:            now,
		}, true
	}
	return WorkflowStep{}, false
}

func pickSkillWorkflowStep(goal string, skillSource SkillPlanningSource, now time.Time) (WorkflowStep, bool) {
	if skillSource == nil {
		return WorkflowStep{}, false
	}
	for _, skill := range skillSource.ListSkills() {
		if !skill.Executable || !skill.Available {
			continue
		}
		approval := skill.ApprovalModeExpected
		if approval == "" {
			approval = "allow"
		}
		return WorkflowStep{
			WorkflowStepID:       newWorkflowStepID(),
			Title:                "Run executable skill " + skill.SkillID,
			ConsumerKind:         string(runtime.ToolCallInvocationKindSkill),
			ConsumerID:           skill.SkillID,
			ToolName:             skill.SkillID,
			Input:                map[string]any{"args": []string{goal}},
			SelectionRationale:   "Selected the first available executable skill to continue the workflow without a new execution boundary.",
			ApprovalModeExpected: approval,
			MaxAttempts:          2,
			CreatedAt:            now,
			UpdatedAt:            now,
		}, true
	}
	return WorkflowStep{}, false
}

func pickLocalWorkflowStep(cfg config.Config, goal string, capabilitySupervisor *capabilities.Supervisor, now time.Time) (WorkflowStep, bool) {
	if capabilitySupervisor == nil {
		return WorkflowStep{}, false
	}
	for _, capability := range capabilitySupervisor.List() {
		if capability.Status == capabilities.StatusFailed {
			continue
		}
		switch capability.Kind {
		case "shell":
			return WorkflowStep{
				WorkflowStepID:       newWorkflowStepID(),
				Title:                "Run local shell capability",
				ConsumerKind:         string(runtime.ToolCallInvocationKindLocalTool),
				ConsumerID:           capability.CapabilityID,
				ToolName:             "shell",
				Input:                map[string]any{"cmd": "printf %s " + shellEscape(goal), "cwd": cfg.DataDir},
				SelectionRationale:   "Selected a local shell capability because no better allow-mode executable consumer was available.",
				ApprovalModeExpected: "ask",
				MaxAttempts:          1,
				CreatedAt:            now,
				UpdatedAt:            now,
			}, true
		case "exec":
			return WorkflowStep{
				WorkflowStepID:       newWorkflowStepID(),
				Title:                "Run local exec capability",
				ConsumerKind:         string(runtime.ToolCallInvocationKindLocalTool),
				ConsumerID:           capability.CapabilityID,
				ToolName:             "exec",
				Input:                map[string]any{"command": "echo", "args": []string{goal}, "cwd": cfg.DataDir},
				SelectionRationale:   "Selected a local exec capability because no better allow-mode executable consumer was available.",
				ApprovalModeExpected: "ask",
				MaxAttempts:          1,
				CreatedAt:            now,
				UpdatedAt:            now,
			}, true
		}
	}
	return WorkflowStep{}, false
}

func shellEscape(value string) string {
	return strings.ReplaceAll(value, "'", "'\"'\"'")
}

func newWorkflowID() string {
	return "wf_" + newWorkflowToken()
}

func newWorkflowStepID() string {
	return "wfstep_" + newWorkflowToken()
}

func newWorkflowDependencyID() string {
	return "wfdep_" + newWorkflowToken()
}

func newWorkflowHandoffID() string {
	return "wfhandoff_" + newWorkflowToken()
}

func newWorkflowToken() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
