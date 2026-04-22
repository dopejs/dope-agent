package computeruse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

var (
	ErrSessionNotFound = errors.New("computer-use session not found")
	ErrActionNotFound  = errors.New("computer-use action not found")
	ErrUnsupportedMode = errors.New("computer-use request is outside the browser-first phase 26 scope")
)

type Store interface {
	UpsertComputerUseSession(context.Context, Session) error
	ListComputerUseSessions(context.Context, string, string) ([]Session, error)
	GetComputerUseSession(context.Context, string, string, string) (Session, bool, error)
	UpsertComputerUseAction(context.Context, Action) error
	ListComputerUseActions(context.Context, string, string, string) ([]Action, error)
	GetComputerUseAction(context.Context, string, string, string, string) (Action, bool, error)
	FindPendingComputerUseActionByApproval(context.Context, string, string) (Action, bool, error)
	UpsertComputerUseArtifact(context.Context, Artifact) error
	ListComputerUseArtifactsForAction(context.Context, string, string, string) ([]Artifact, error)
	GetComputerUseArtifact(context.Context, string, string) (Artifact, bool, error)
	MarkInFlightComputerUseInterrupted(context.Context, string, time.Time) ([]Session, []Action, error)
}

type ArtifactRecorder interface {
	SaveComputerUseArtifact(context.Context, ArtifactCaptureRequest) (Artifact, error)
	ReadComputerUseArtifactContent(context.Context, string) ([]byte, error)
}

type Manager struct {
	environment string
	runtime     *runtime.Manager
	policy      *policy.Engine
	store       Store
	driver      Driver
	artifacts   ArtifactRecorder
}

type Dependencies struct {
	EnvironmentScope string
	Runtime          *runtime.Manager
	Policy           *policy.Engine
	Store            Store
	Driver           Driver
	Artifacts        ArtifactRecorder
}

func NewManager(deps Dependencies) *Manager {
	driver := deps.Driver
	if driver == nil {
		driver = NewMemoryDriver()
	}
	return &Manager{
		environment: strings.TrimSpace(deps.EnvironmentScope),
		runtime:     deps.Runtime,
		policy:      deps.Policy,
		store:       deps.Store,
		driver:      driver,
		artifacts:   deps.Artifacts,
	}
}

func (m *Manager) AcquireSession(ctx context.Context, runID string, input CreateSessionInput) (Session, bool, error) {
	if strings.TrimSpace(input.WorkflowID) != "" {
		sessions, err := m.store.ListComputerUseSessions(ctx, m.environment, runID)
		if err != nil {
			return Session{}, false, err
		}
		for _, session := range sessions {
			if session.WorkflowID != strings.TrimSpace(input.WorkflowID) {
				continue
			}
			switch session.Status {
			case SessionStatusStarting, SessionStatusActive, SessionStatusBlocked:
				enriched, err := m.enrichSession(ctx, session)
				return enriched, true, err
			}
		}
	}
	session, err := m.CreateSession(ctx, runID, input)
	return session, false, err
}

func (m *Manager) CreateSession(ctx context.Context, runID string, input CreateSessionInput) (Session, error) {
	if m.runtime == nil {
		return Session{}, errors.New("runtime manager is not configured")
	}
	if _, ok := m.runtime.GetRun(runID); !ok {
		return Session{}, runtime.ErrRunNotFound
	}
	if driverKind := firstNonEmpty(input.DriverKind, "browser"); driverKind != "browser" {
		return Session{}, fmt.Errorf("%w: phase 26 is browser-first and does not support driver kind %q", ErrUnsupportedMode, input.DriverKind)
	}
	now := time.Now().UTC()
	session := Session{
		ComputerUseSessionID: newComputerUseID("cusess"),
		EnvironmentScope:     m.environment,
		RunID:                strings.TrimSpace(runID),
		WorkflowID:           strings.TrimSpace(input.WorkflowID),
		WorkflowStepID:       strings.TrimSpace(input.WorkflowStepID),
		Status:               SessionStatusStarting,
		DriverKind:           firstNonEmpty(input.DriverKind, "browser"),
		StartedAt:            now,
		UpdatedAt:            now,
	}
	started, err := m.driver.StartSession(ctx, session, input)
	if err != nil {
		return Session{}, err
	}
	if err := m.store.UpsertComputerUseSession(ctx, started); err != nil {
		return Session{}, err
	}
	return m.enrichSession(ctx, started)
}

func (m *Manager) ListSessions(ctx context.Context, runID string) ([]Session, error) {
	sessions, err := m.store.ListComputerUseSessions(ctx, m.environment, runID)
	if err != nil {
		return nil, err
	}
	items := make([]Session, 0, len(sessions))
	for _, item := range sessions {
		enriched, enrichErr := m.enrichSession(ctx, item)
		if enrichErr != nil {
			return nil, enrichErr
		}
		items = append(items, enriched)
	}
	return items, nil
}

func (m *Manager) GetSession(ctx context.Context, runID, sessionID string) (Session, bool, error) {
	session, ok, err := m.store.GetComputerUseSession(ctx, m.environment, runID, sessionID)
	if err != nil || !ok {
		return Session{}, ok, err
	}
	enriched, err := m.enrichSession(ctx, session)
	return enriched, true, err
}

func (m *Manager) CloseSession(ctx context.Context, runID, sessionID string) (Session, error) {
	session, ok, err := m.store.GetComputerUseSession(ctx, m.environment, runID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	closed, err := m.driver.CloseSession(ctx, session)
	if err != nil {
		return Session{}, err
	}
	if err := m.store.UpsertComputerUseSession(ctx, closed); err != nil {
		return Session{}, err
	}
	return m.enrichSession(ctx, closed)
}

func (m *Manager) CreateAction(ctx context.Context, runID, sessionID, requestedBy string, input CreateActionInput) (ActionRequestResult, *policy.Approval, *policy.Decision, error) {
	session, ok, err := m.store.GetComputerUseSession(ctx, m.environment, runID, sessionID)
	if err != nil {
		return ActionRequestResult{}, nil, nil, err
	}
	if !ok {
		return ActionRequestResult{}, nil, nil, ErrSessionNotFound
	}
	if err := validateCreateActionInput(input); err != nil {
		return ActionRequestResult{}, nil, nil, err
	}
	step, toolCall, err := m.createRuntimeTracking(ctx, session, input)
	if err != nil {
		return ActionRequestResult{}, nil, nil, err
	}
	action := Action{
		ComputerUseActionID:  newComputerUseID("cuact"),
		EnvironmentScope:     m.environment,
		ComputerUseSessionID: session.ComputerUseSessionID,
		RunID:                session.RunID,
		StepID:               step.StepID,
		ToolCallID:           toolCall.ToolCallID,
		WorkflowID:           session.WorkflowID,
		WorkflowStepID:       session.WorkflowStepID,
		ActionKind:           input.ActionKind,
		Status:               ActionStatusRequested,
		RiskLevel:            classifyRisk(session, input),
		TargetMatchContext:   cloneTargetMatch(input.TargetMatchContext),
		PageBefore:           clonePage(session.CurrentPage),
		RequestedAt:          time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
		Input: map[string]any{
			"url":           strings.TrimSpace(input.URL),
			"value":         input.Value,
			"selectedValue": input.SelectedValue,
			"waitMs":        input.WaitMs,
			"pageTarget":    firstNonEmpty(string(input.PageTarget), string(PageTargetActivePage)),
			"rationale":     strings.TrimSpace(input.Rationale),
		},
	}
	if action.RiskLevel == RiskLevelHigh && m.policy != nil {
		approval, decision, reqErr := m.policy.RequestApproval(policy.RequestApprovalInput{
			Action:       "computer_use.action.execute",
			ResourceKind: "computer_use_action",
			ResourceID:   action.ComputerUseActionID,
			Reason:       "high-risk computer-use action requires approval",
			RequestedBy:  requestedBy,
		})
		if reqErr != nil {
			return ActionRequestResult{}, nil, nil, reqErr
		}
		action.ApprovalID = approval.ApprovalID
		action.Status = ActionStatusWaitingApproval
		if _, err := m.runtime.UpdateStepStatus(ctxRunID(step), step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusBlocked}); err != nil {
			return ActionRequestResult{}, nil, nil, err
		}
		if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
			return ActionRequestResult{}, nil, nil, err
		}
		session.Status = SessionStatusBlocked
		session.LastActionID = action.ComputerUseActionID
		session.UpdatedAt = time.Now().UTC()
		if err := m.store.UpsertComputerUseSession(ctx, session); err != nil {
			return ActionRequestResult{}, nil, nil, err
		}
		enriched, err := m.enrichAction(ctx, action)
		if err != nil {
			return ActionRequestResult{}, nil, nil, err
		}
		return ActionRequestResult{Action: enriched, Pending: true, Approved: false}, &approval, &decision, nil
	}
	enriched, execErr := m.executeAction(ctx, session, action)
	if execErr != nil {
		return ActionRequestResult{}, nil, nil, execErr
	}
	return ActionRequestResult{Action: enriched, Pending: false, Approved: true}, nil, nil, nil
}

func (m *Manager) ResumePendingAction(ctx context.Context, approvalID string) (Action, bool, error) {
	if strings.TrimSpace(approvalID) == "" {
		return Action{}, false, nil
	}
	if m.policy == nil {
		return Action{}, false, nil
	}
	approval, ok := m.policy.GetApproval(strings.TrimSpace(approvalID))
	if !ok {
		return Action{}, false, nil
	}
	action, ok, err := m.store.FindPendingComputerUseActionByApproval(ctx, m.environment, approvalID)
	if err != nil || !ok {
		return Action{}, ok, err
	}
	session, ok, err := m.store.GetComputerUseSession(ctx, m.environment, action.RunID, action.ComputerUseSessionID)
	if err != nil || !ok {
		return Action{}, ok, err
	}
	switch approval.Status {
	case policy.ApprovalStatusRejected:
		now := time.Now().UTC()
		action.Status = ActionStatusDenied
		action.FailureClass = string(FailureClassPolicyDenied)
		action.FailureReason = "approval was rejected"
		action.UpdatedAt = now
		action.CompletedAt = &now
		if _, err := m.runtime.DenyToolCall(action.RunID, action.StepID, action.ToolCallID, runtime.DenyToolCallInput{
			Output:       map[string]any{"approvalId": approvalID},
			Error:        "approval was rejected",
			FailureClass: string(FailureClassPolicyDenied),
		}); err != nil {
			return Action{}, false, err
		}
		if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusBlocked,
			Output: map[string]any{"approvalId": approvalID},
		}); err != nil {
			return Action{}, false, err
		}
		session.Status = SessionStatusActive
		session.UpdatedAt = now
		if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
			return Action{}, false, err
		}
		if err := m.store.UpsertComputerUseSession(ctx, session); err != nil {
			return Action{}, false, err
		}
		enriched, err := m.enrichAction(ctx, action)
		return enriched, true, err
	case policy.ApprovalStatusApproved:
		enriched, err := m.executeAction(ctx, session, action)
		return enriched, true, err
	default:
		return Action{}, true, nil
	}
}

func (m *Manager) GetArtifact(ctx context.Context, artifactID string) (Artifact, bool, error) {
	return m.store.GetComputerUseArtifact(ctx, m.environment, artifactID)
}

func (m *Manager) ReadArtifactContent(ctx context.Context, artifactID string) (Artifact, []byte, bool, error) {
	artifact, ok, err := m.GetArtifact(ctx, artifactID)
	if err != nil || !ok {
		return Artifact{}, nil, ok, err
	}
	if m.artifacts == nil {
		return artifact, nil, true, nil
	}
	content, err := m.artifacts.ReadComputerUseArtifactContent(ctx, artifact.StorageKey)
	return artifact, content, true, err
}

func (m *Manager) executeAction(ctx context.Context, session Session, action Action) (Action, error) {
	now := time.Now().UTC()
	history, err := m.store.ListComputerUseActions(ctx, m.environment, action.RunID, action.ComputerUseSessionID)
	if err != nil {
		return Action{}, err
	}
	session.Actions = history
	if step, ok := m.runtime.GetStep(action.RunID, action.StepID); ok {
		switch step.Status {
		case runtime.StepStatusBlocked:
			if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusPlanning}); err != nil {
				return Action{}, err
			}
			if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
				return Action{}, err
			}
		case runtime.StepStatusPlanning:
			if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
				return Action{}, err
			}
		}
	}
	if action.TargetMatchContext != nil {
		action.TargetMatchContext.EvaluatedAt = now
		action.TargetMatchContext.MatchResult = MatchResultMatched
		action.TargetMatchContext.ObservedPageURL = firstPageField(session.CurrentPage, session.CurrentPage, "url")
	}
	if mismatchArtifacts, mismatched := evaluateTargetMatch(session, &action); mismatched {
		action.Status = ActionStatusFailed
		action.FailureClass = string(FailureClassTargetMismatch)
		action.FailureReason = "approved target no longer matches current page"
		action.UpdatedAt = now
		action.CompletedAt = &now
		if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
			return Action{}, err
		}
		for _, capture := range mismatchArtifacts {
			if m.artifacts == nil {
				continue
			}
			artifact, saveErr := m.artifacts.SaveComputerUseArtifact(ctx, capture)
			if saveErr != nil {
				continue
			}
			artifact.EnvironmentScope = m.environment
			action.Artifacts = append(action.Artifacts, artifact)
			if persistErr := m.store.UpsertComputerUseArtifact(ctx, artifact); persistErr != nil {
				return Action{}, persistErr
			}
		}
		if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
			return Action{}, err
		}
		if err := m.store.UpsertComputerUseSession(ctx, session); err != nil {
			return Action{}, err
		}
		if _, err := m.runtime.FailToolCall(action.RunID, action.StepID, action.ToolCallID, runtime.FailToolCallInput{
			Output: map[string]any{
				"computerUseSessionId": action.ComputerUseSessionID,
				"computerUseActionId":  action.ComputerUseActionID,
			},
			Error:        action.FailureReason,
			FailureClass: action.FailureClass,
		}); err != nil {
			return Action{}, err
		}
		if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"computerUseActionId": action.ComputerUseActionID,
				"failureClass":        action.FailureClass,
			},
		}); err != nil {
			return Action{}, err
		}
		return m.enrichAction(ctx, action)
	}
	if _, err := m.runtime.MarkToolCallRunning(action.RunID, action.StepID, action.ToolCallID, "", nil); err != nil {
		return Action{}, err
	}
	runningSession, executedAction, captures, err := m.driver.ExecuteAction(ctx, session, action)
	if err != nil && executedAction.Status == "" {
		executedAction = action
		executedAction.Status = ActionStatusFailed
		executedAction.FailureClass = string(failureClassForDriverError(action.ActionKind))
		executedAction.FailureReason = err.Error()
		executedAction.UpdatedAt = now
		executedAction.CompletedAt = &now
	}
	action = executedAction
	session = runningSession
	if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
		return Action{}, err
	}
	for _, capture := range captures {
		if m.artifacts == nil {
			continue
		}
		artifact, saveErr := m.artifacts.SaveComputerUseArtifact(ctx, capture)
		if saveErr != nil {
			continue
		}
		artifact.EnvironmentScope = m.environment
		action.Artifacts = append(action.Artifacts, artifact)
		if persistErr := m.store.UpsertComputerUseArtifact(ctx, artifact); persistErr != nil {
			return Action{}, persistErr
		}
	}
	if err := m.store.UpsertComputerUseAction(ctx, action); err != nil {
		return Action{}, err
	}
	if err := m.store.UpsertComputerUseSession(ctx, session); err != nil {
		return Action{}, err
	}
	switch action.Status {
	case ActionStatusCompleted:
		if _, err := m.runtime.CompleteToolCall(action.RunID, action.StepID, action.ToolCallID, runtime.CompleteToolCallInput{
			Output: map[string]any{
				"computerUseSessionId": action.ComputerUseSessionID,
				"computerUseActionId":  action.ComputerUseActionID,
			},
		}); err != nil {
			return Action{}, err
		}
		if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusCompleted,
			Output: map[string]any{
				"computerUseActionId": action.ComputerUseActionID,
			},
		}); err != nil {
			return Action{}, err
		}
	default:
		if _, err := m.runtime.FailToolCall(action.RunID, action.StepID, action.ToolCallID, runtime.FailToolCallInput{
			Output: map[string]any{
				"computerUseSessionId": action.ComputerUseSessionID,
				"computerUseActionId":  action.ComputerUseActionID,
			},
			Error:        action.FailureReason,
			FailureClass: action.FailureClass,
		}); err != nil {
			return Action{}, err
		}
		if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{
			Status: runtime.StepStatusFailed,
			Output: map[string]any{
				"computerUseActionId": action.ComputerUseActionID,
				"failureClass":        action.FailureClass,
			},
		}); err != nil {
			return Action{}, err
		}
	}
	return m.enrichAction(ctx, action)
}

func (m *Manager) createRuntimeTracking(ctx context.Context, session Session, input CreateActionInput) (runtime.Step, runtime.ToolCall, error) {
	step, err := m.runtime.CreateStep(session.RunID, runtime.CreateStepInput{
		Title:          fmt.Sprintf("Computer-use %s", input.ActionKind),
		Kind:           "computer_use",
		WorkflowID:     session.WorkflowID,
		WorkflowStepID: session.WorkflowStepID,
		Input: map[string]any{
			"actionKind": input.ActionKind,
			"url":        strings.TrimSpace(input.URL),
		},
	})
	if err != nil {
		return runtime.Step{}, runtime.ToolCall{}, err
	}
	if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(session.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusPlanning}); err != nil {
		return runtime.Step{}, runtime.ToolCall{}, err
	}
	if _, _, err := m.runtime.UpdateStepStatusAndReconcileRun(session.RunID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err != nil {
		return runtime.Step{}, runtime.ToolCall{}, err
	}
	toolCall, err := m.runtime.CreateToolCall(session.RunID, step.StepID, runtime.CreateToolCallInput{
		WorkflowID:           session.WorkflowID,
		WorkflowStepID:       session.WorkflowStepID,
		InvocationKind:       runtime.ToolCallInvocationKindLocalTool,
		CapabilityID:         "browser",
		ToolName:             string(input.ActionKind),
		Input:                map[string]any{"actionKind": input.ActionKind, "url": strings.TrimSpace(input.URL)},
		ComputerUseSessionID: session.ComputerUseSessionID,
	})
	if err != nil {
		return runtime.Step{}, runtime.ToolCall{}, err
	}
	return step, toolCall, nil
}

func (m *Manager) enrichSession(ctx context.Context, session Session) (Session, error) {
	actions, err := m.store.ListComputerUseActions(ctx, m.environment, session.RunID, session.ComputerUseSessionID)
	if err != nil {
		return Session{}, err
	}
	session.Actions = make([]Action, 0, len(actions))
	for _, action := range actions {
		enriched, enrichErr := m.enrichAction(ctx, action)
		if enrichErr != nil {
			return Session{}, enrichErr
		}
		session.Actions = append(session.Actions, enriched)
	}
	return session, nil
}

func (m *Manager) enrichAction(ctx context.Context, action Action) (Action, error) {
	artifacts, err := m.store.ListComputerUseArtifactsForAction(ctx, m.environment, action.RunID, action.ComputerUseActionID)
	if err != nil {
		return Action{}, err
	}
	action.Artifacts = artifacts
	return action, nil
}

func classifyRisk(session Session, input CreateActionInput) RiskLevel {
	switch input.ActionKind {
	case ActionKindClick, ActionKindInput, ActionKindSelect, ActionKindDownload:
		return RiskLevelHigh
	case ActionKindNavigate:
		if session.TrustedPageScope == nil || strings.TrimSpace(input.URL) == "" {
			return RiskLevelLow
		}
		if originFromURL(input.URL) != "" && originFromURL(input.URL) != session.TrustedPageScope.Origin {
			return RiskLevelHigh
		}
		return RiskLevelLow
	default:
		return RiskLevelLow
	}
}

func validateCreateActionInput(input CreateActionInput) error {
	if !isSupportedActionKind(input.ActionKind) {
		return fmt.Errorf("%w: unsupported action kind %q", ErrUnsupportedMode, input.ActionKind)
	}
	switch firstNonEmpty(string(input.PageTarget), string(PageTargetActivePage)) {
	case string(PageTargetActivePage):
		return nil
	case string(PageTargetNewTab), string(PageTargetNewWindow):
		return fmt.Errorf("%w: phase 26 supports only a single active page and rejects %q requests", ErrUnsupportedMode, input.PageTarget)
	default:
		return fmt.Errorf("%w: unsupported page target %q", ErrUnsupportedMode, input.PageTarget)
	}
}

func isSupportedActionKind(kind ActionKind) bool {
	switch kind {
	case ActionKindNavigate, ActionKindBack, ActionKindForward, ActionKindWait, ActionKindScreenshot, ActionKindSnapshot, ActionKindClick, ActionKindInput, ActionKindSelect, ActionKindDownload, ActionKindCloseSession:
		return true
	default:
		return false
	}
}

func evaluateTargetMatch(session Session, action *Action) ([]ArtifactCaptureRequest, bool) {
	if action == nil || action.TargetMatchContext == nil {
		return nil, false
	}
	now := time.Now().UTC()
	action.TargetMatchContext.EvaluatedAt = now
	action.TargetMatchContext.ObservedPageURL = firstPageField(session.CurrentPage, session.CurrentPage, "url")
	if !mismatched(action.TargetMatchContext) {
		action.TargetMatchContext.MatchResult = MatchResultMatched
		return nil, false
	}
	action.TargetMatchContext.MatchResult = MatchResultMismatched
	return []ArtifactCaptureRequest{buildPageEvidenceCapture(session, *action, ActionKindSnapshot)}, true
}

func failureClassForDriverError(kind ActionKind) FailureClass {
	if kind == ActionKindNavigate {
		return FailureClassNavigationFailure
	}
	return FailureClassUnavailableConsumer
}

func cloneTargetMatch(input *TargetMatchContext) *TargetMatchContext {
	if input == nil {
		return nil
	}
	cloned := *input
	return &cloned
}

func newComputerUseID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func ctxRunID(step runtime.Step) string {
	return step.RunID
}
