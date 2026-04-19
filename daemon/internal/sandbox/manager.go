package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var (
	ErrCommandRequired   = errors.New("sandbox command is required")
	ErrExecutionNotFound = errors.New("sandbox execution not found")
)

const (
	sandboxApprovalAction  = "sandbox.execute"
	sandboxResourceKind    = "sandbox_profile"
	eventCategory          = "sandbox"
	resourceKindExecution  = "sandbox_execution"
	backendMetaProcessKind = "process"
)

type Manager struct {
	cfg      config.Config
	store    *store.SQLiteStore
	eventBus *events.Bus
	policy   *policy.Engine

	mu           sync.RWMutex
	profiles     map[string]Profile
	profileIDs   []string
	executions   map[string]Execution
	executionIDs []string
	cancels      map[string]context.CancelFunc
}

func NewManager(cfg config.Config, sqliteStore *store.SQLiteStore, eventBus *events.Bus, policyEngine *policy.Engine) *Manager {
	manager := &Manager{
		cfg:        cfg,
		store:      sqliteStore,
		eventBus:   eventBus,
		policy:     policyEngine,
		profiles:   map[string]Profile{},
		executions: map[string]Execution{},
		cancels:    map[string]context.CancelFunc{},
	}
	manager.reloadBuiltins()
	return manager
}

func (m *Manager) Reload() []Profile {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadBuiltinsLocked()
	return m.listProfilesLocked()
}

func (m *Manager) ListProfiles() []Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listProfilesLocked()
}

func (m *Manager) GetProfile(profileID string) (Profile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profile, ok := m.profiles[normalizeProfileID(profileID)]
	return profile, ok
}

func (m *Manager) ListExecutions() []Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Execution, 0, len(m.executionIDs))
	for _, executionID := range m.executionIDs {
		items = append(items, cloneExecution(m.executions[executionID]))
	}
	return items
}

func (m *Manager) GetExecution(executionID string) (Execution, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.executions[strings.TrimSpace(executionID)]
	if !ok {
		return Execution{}, false
	}
	return cloneExecution(item), true
}

func (m *Manager) Explain(ctx context.Context, request ExecutionRequest) (Decision, error) {
	_, decision, _, _, _, err := m.prepare(ctx, request, false)
	if err != nil {
		return Decision{}, err
	}
	return decision, nil
}

func (m *Manager) StartExecution(ctx context.Context, request ExecutionRequest) (Execution, error) {
	execution, decision, launch, approval, createdDecision, err := m.prepare(ctx, request, true)
	if err != nil {
		return Execution{}, err
	}

	execution.Decision = decision
	execution.ApprovalID = approval
	execution.Status = decisionToStatus(decision)
	execution.Result.Status = execution.Status
	if execution.Status == ExecutionStatusDenied {
		execution.Result.ErrorClass = decisionErrorClass(decision)
		execution.Result.ErrorCode = decisionErrorCode(decision)
		execution.Result.Error = decision.Explanation
	}

	m.mu.Lock()
	if _, exists := m.executions[execution.ExecutionID]; !exists {
		m.executionIDs = append(m.executionIDs, execution.ExecutionID)
	}
	m.executions[execution.ExecutionID] = cloneExecution(execution)
	m.mu.Unlock()

	if createdDecision != nil {
		if err := m.persistApprovalArtifacts(ctx, approval, *createdDecision); err != nil {
			return Execution{}, err
		}
		if err := m.publishApprovalRequested(ctx, approval, *createdDecision); err != nil {
			return Execution{}, err
		}
	}
	if err := m.persistExecution(ctx, execution); err != nil {
		return Execution{}, err
	}
	if err := m.publishExecutionRequested(ctx, execution); err != nil {
		return Execution{}, err
	}
	if err := m.publishDecisionRecorded(ctx, execution); err != nil {
		return Execution{}, err
	}
	if execution.Status == ExecutionStatusDenied {
		if err := m.publishExecutionTerminal(ctx, execution); err != nil {
			return Execution{}, err
		}
		return cloneExecution(execution), nil
	}

	runCtx, cancel := context.WithTimeout(context.Background(), launch.Timeout)
	m.mu.Lock()
	m.cancels[execution.ExecutionID] = cancel
	m.mu.Unlock()
	go m.runExecution(runCtx, cancel, execution, launch)
	return cloneExecution(execution), nil
}

func (m *Manager) CancelExecution(executionID string) (Execution, bool, error) {
	m.mu.RLock()
	execution, ok := m.executions[strings.TrimSpace(executionID)]
	cancel := m.cancels[strings.TrimSpace(executionID)]
	m.mu.RUnlock()
	if !ok {
		return Execution{}, false, ErrExecutionNotFound
	}
	if IsTerminal(execution.Status) {
		return cloneExecution(execution), true, nil
	}
	if cancel != nil {
		cancel()
	}
	return cloneExecution(execution), false, nil
}

func (m *Manager) WaitExecution(ctx context.Context, executionID string) (Execution, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		execution, ok := m.GetExecution(executionID)
		if !ok {
			return Execution{}, ErrExecutionNotFound
		}
		if IsTerminal(execution.Status) {
			return execution, nil
		}
		select {
		case <-ctx.Done():
			return Execution{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) Close(ctx context.Context) error {
	m.mu.RLock()
	executionIDs := append([]string(nil), m.executionIDs...)
	m.mu.RUnlock()

	var firstErr error
	for _, executionID := range executionIDs {
		_, _, err := m.CancelExecution(executionID)
		if err != nil && !errors.Is(err, ErrExecutionNotFound) && firstErr == nil {
			firstErr = err
		}
	}
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if firstErr != nil {
				return firstErr
			}
			return ctx.Err()
		case <-deadline:
			return firstErr
		case <-ticker.C:
			if !m.hasActiveExecutions() {
				return firstErr
			}
		}
	}
}

func (m *Manager) Restore(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	records, err := m.store.ListSandboxExecutions(ctx)
	if err != nil {
		return fmt.Errorf("list sandbox executions: %w", err)
	}

	m.mu.Lock()
	m.executions = map[string]Execution{}
	m.executionIDs = m.executionIDs[:0]
	m.cancels = map[string]context.CancelFunc{}
	for _, record := range records {
		var execution Execution
		if err := json.Unmarshal(record.Document, &execution); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("decode sandbox execution %s: %w", record.ExecutionID, err)
		}
		m.executions[execution.ExecutionID] = execution
		m.executionIDs = append(m.executionIDs, execution.ExecutionID)
	}
	m.mu.Unlock()

	for _, record := range records {
		if IsTerminal(ExecutionStatus(record.Status)) {
			continue
		}
		execution, ok := m.GetExecution(record.ExecutionID)
		if !ok {
			continue
		}
		now := time.Now().UTC()
		execution.Status = ExecutionStatusCancelled
		execution.Result.Status = ExecutionStatusCancelled
		execution.Result.ErrorClass = ErrorClassCancelled
		execution.Result.ErrorCode = "daemon_restarted"
		execution.Result.Error = "execution was interrupted by daemon restart recovery"
		execution.Result.CompletedAt = &now
		execution.CompletedAt = &now
		execution.UpdatedAt = now
		m.storeExecution(execution)
		if err := m.persistExecution(ctx, execution); err != nil {
			return err
		}
		if err := m.publishExecutionTerminal(ctx, execution); err != nil {
			return err
		}
	}
	return nil
}

type launchSpec struct {
	Command        string
	Args           []string
	Cwd            string
	Env            []string
	Stdin          string
	Timeout        time.Duration
	KillGrace      time.Duration
	CaptureStdout  bool
	CaptureStderr  bool
	MaxOutputBytes int
}

func (m *Manager) prepare(ctx context.Context, request ExecutionRequest, createApproval bool) (Execution, Decision, *launchSpec, string, *policy.Decision, error) {
	profile, ok := m.GetProfile(firstNonEmpty(request.ProfileID, ProfileIDSubprocessDefault))
	if !ok {
		now := time.Now().UTC()
		executionID := newID("sandbox_exec")
		decision := Decision{
			DecisionID:           newID("sandbox_decision"),
			ExecutionID:          executionID,
			Resolution:           DecisionResolutionDeny,
			MatchedRules:         []string{"profile:not_found"},
			ApprovalStatus:       DecisionApprovalStatusNotApplicable,
			EffectiveProfileID:   strings.TrimSpace(request.ProfileID),
			EffectiveBackendKind: BackendKindSubprocess,
			Explanation:          "sandbox profile was not found",
		}
		execution := Execution{
			ExecutionID:   executionID,
			ProfileID:     strings.TrimSpace(request.ProfileID),
			BackendKind:   BackendKindSubprocess,
			Command:       strings.TrimSpace(request.Command),
			Args:          cloneStrings(request.Args),
			Cwd:           strings.TrimSpace(request.Cwd),
			EnvKeys:       sortedKeys(request.Env),
			StdinProvided: request.Stdin != "",
			TimeoutMs:     request.TimeoutMs,
			RequestedBy:   strings.TrimSpace(request.RequestedBy),
			ResourceKind:  strings.TrimSpace(request.ResourceKind),
			ResourceID:    strings.TrimSpace(request.ResourceID),
			Scope:         strings.TrimSpace(request.Scope),
			Reason:        strings.TrimSpace(request.Reason),
			Metadata:      cloneStringMap(request.Metadata),
			Access:        cloneAccess(request.Access),
			RequestedAt:   now,
			UpdatedAt:     now,
			Result: Result{
				Status:          ExecutionStatusDenied,
				ErrorClass:      ErrorClassInvalidProfile,
				ErrorCode:       "sandbox_profile_not_found",
				Error:           "sandbox profile was not found",
				Partial:         false,
				OutputTruncated: false,
			},
		}
		return execution, decision, nil, "", nil, nil
	}
	if strings.TrimSpace(request.Command) == "" {
		return Execution{}, Decision{}, nil, "", nil, ErrCommandRequired
	}

	executionID := newID("sandbox_exec")
	now := time.Now().UTC()
	cwd, err := normalizePath(profile.DefaultWorkDir, request.Cwd)
	if err != nil {
		return Execution{}, Decision{}, nil, "", nil, fmt.Errorf("resolve sandbox cwd: %w", err)
	}
	readRoots, err := normalizePaths(cwd, request.Access.ReadRoots)
	if err != nil {
		return Execution{}, Decision{}, nil, "", nil, fmt.Errorf("resolve sandbox read roots: %w", err)
	}
	writeRoots, err := normalizePaths(cwd, request.Access.WriteRoots)
	if err != nil {
		return Execution{}, Decision{}, nil, "", nil, fmt.Errorf("resolve sandbox write roots: %w", err)
	}
	timeoutMs := effectiveTimeout(profile, request.TimeoutMs)
	env := buildEnvironment(profile, request.Env)

	execution := Execution{
		ExecutionID:   executionID,
		ProfileID:     profile.ProfileID,
		BackendKind:   profile.BackendKind,
		Command:       strings.TrimSpace(request.Command),
		Args:          cloneStrings(request.Args),
		Cwd:           cwd,
		EnvKeys:       sortedKeys(request.Env),
		StdinProvided: request.Stdin != "",
		TimeoutMs:     timeoutMs,
		RequestedBy:   strings.TrimSpace(request.RequestedBy),
		ResourceKind:  strings.TrimSpace(request.ResourceKind),
		ResourceID:    strings.TrimSpace(request.ResourceID),
		Scope:         strings.TrimSpace(request.Scope),
		ApprovalID:    strings.TrimSpace(request.ApprovalID),
		Reason:        strings.TrimSpace(request.Reason),
		Metadata:      cloneStringMap(request.Metadata),
		Access: AccessRequest{
			ReadRoots:     readRoots,
			WriteRoots:    writeRoots,
			NetworkMode:   request.Access.NetworkMode,
			AllowedHosts:  cloneStrings(request.Access.AllowedHosts),
			AllowedPorts:  cloneInts(request.Access.AllowedPorts),
			AllowLoopback: request.Access.AllowLoopback,
		},
		Status:      ExecutionStatusPending,
		RequestedAt: now,
		UpdatedAt:   now,
		Result: Result{
			Status:          ExecutionStatusPending,
			OutputTruncated: false,
			Partial:         false,
		},
	}
	decision, approvalID, createdDecision, err := m.evaluate(ctx, profile, execution, createApproval)
	if err != nil {
		return Execution{}, Decision{}, nil, "", nil, err
	}
	decision.ExecutionID = executionID
	execution.Decision = decision
	execution.ApprovalID = approvalID

	launch := &launchSpec{
		Command:        execution.Command,
		Args:           cloneStrings(execution.Args),
		Cwd:            execution.Cwd,
		Env:            env,
		Stdin:          request.Stdin,
		Timeout:        time.Duration(timeoutMs) * time.Millisecond,
		KillGrace:      time.Duration(profile.ProcessPolicy.KillGraceMs) * time.Millisecond,
		CaptureStdout:  profile.ProcessPolicy.CaptureStdout,
		CaptureStderr:  profile.ProcessPolicy.CaptureStderr,
		MaxOutputBytes: profile.ProcessPolicy.MaxOutputBytes,
	}
	return execution, decision, launch, approvalID, createdDecision, nil
}

func (m *Manager) evaluate(ctx context.Context, profile Profile, execution Execution, createApproval bool) (Decision, string, *policy.Decision, error) {
	decision := Decision{
		DecisionID:           newID("sandbox_decision"),
		Resolution:           DecisionResolutionAllow,
		MatchedRules:         []string{"profile:" + profile.ProfileID},
		ApprovalStatus:       DecisionApprovalStatusNotApplicable,
		EffectiveProfileID:   profile.ProfileID,
		EffectiveBackendKind: profile.BackendKind,
		Explanation:          "execution is allowed by sandbox profile",
	}

	requestedApproval := strings.TrimSpace(execution.ApprovalID)
	approvalRequired := false
	reasons := make([]string, 0, 4)

	if profile.BackendKind != BackendKindSubprocess {
		if profile.ApprovalPolicy.RequiredForUnknownBackends {
			approvalRequired = true
			reasons = append(reasons, "backend:approval_required")
		} else {
			decision.Resolution = DecisionResolutionDeny
			decision.MatchedRules = append(decision.MatchedRules, "backend:unsupported")
			decision.Explanation = "sandbox backend is not available"
			return decision, "", nil, nil
		}
	}

	if rule := commandApprovalRule(profile, execution.Command); rule != "" {
		approvalRequired = true
		reasons = append(reasons, rule)
	}

	fsDecision, fsRule, err := evaluateFilesystem(profile, execution.Cwd, execution.Access)
	if err != nil {
		return Decision{}, "", nil, err
	}
	if fsDecision == DecisionResolutionDeny {
		decision.Resolution = DecisionResolutionDeny
		decision.MatchedRules = append(decision.MatchedRules, fsRule)
		decision.Explanation = "filesystem access is denied by sandbox profile"
		return decision, "", nil, nil
	}
	if fsDecision == DecisionResolutionAsk {
		approvalRequired = true
		reasons = append(reasons, fsRule)
	}

	netDecision, netRule := evaluateNetwork(profile, execution.Access)
	if netDecision == DecisionResolutionDeny {
		decision.Resolution = DecisionResolutionDeny
		decision.MatchedRules = append(decision.MatchedRules, netRule)
		decision.Explanation = "network access is denied by sandbox profile"
		return decision, "", nil, nil
	}
	if netDecision == DecisionResolutionAsk {
		approvalRequired = true
		reasons = append(reasons, netRule)
	}

	if profile.ApprovalPolicy.Mode == ApprovalModeDeny && approvalRequired {
		decision.Resolution = DecisionResolutionDeny
		decision.MatchedRules = append(decision.MatchedRules, reasons...)
		decision.Explanation = "sandbox profile denies requested escalation"
		return decision, "", nil, nil
	}
	if profile.ApprovalPolicy.Mode == ApprovalModeAllow {
		approvalRequired = false
		reasons = nil
	}

	if !approvalRequired {
		return decision, "", nil, nil
	}

	decision.ApprovalRequired = true
	decision.Resolution = DecisionResolutionAsk
	decision.MatchedRules = append(decision.MatchedRules, reasons...)
	decision.Explanation = "sandbox execution requires approval"

	if requestedApproval != "" {
		approval, ok := m.policy.GetApproval(requestedApproval)
		if ok && approval.Action == sandboxApprovalAction && approval.ResourceKind == sandboxResourceKind && approval.ResourceID == profile.ProfileID {
			switch approval.Status {
			case policy.ApprovalStatusApproved:
				decision.Resolution = DecisionResolutionAllow
				decision.ApprovalStatus = DecisionApprovalStatusApproved
				decision.Explanation = "sandbox execution is allowed by approved escalation"
				return decision, requestedApproval, nil, nil
			case policy.ApprovalStatusRejected:
				decision.Resolution = DecisionResolutionDeny
				decision.ApprovalStatus = DecisionApprovalStatusRejected
				decision.Explanation = "sandbox execution was rejected by approval policy"
				return decision, requestedApproval, nil, nil
			default:
				decision.ApprovalStatus = DecisionApprovalStatusPending
				return decision, requestedApproval, nil, nil
			}
		}
	}

	if !createApproval {
		decision.ApprovalStatus = DecisionApprovalStatusPending
		return decision, "", nil, nil
	}

	approval, createdDecision, err := m.policy.RequestApproval(policy.RequestApprovalInput{
		Action:       sandboxApprovalAction,
		ResourceKind: sandboxResourceKind,
		ResourceID:   profile.ProfileID,
		Reason:       firstNonEmpty(execution.Reason, "sandbox execution requires approval"),
		RequestedBy:  firstNonEmpty(execution.RequestedBy, "sandbox"),
	})
	if err != nil {
		return Decision{}, "", nil, fmt.Errorf("request sandbox approval: %w", err)
	}
	decision.ApprovalStatus = DecisionApprovalStatusPending
	return decision, approval.ApprovalID, &createdDecision, nil
}

func (m *Manager) runExecution(ctx context.Context, cancel context.CancelFunc, execution Execution, launch *launchSpec) {
	if launch == nil {
		return
	}
	defer cancel()

	m.mu.Lock()
	now := time.Now().UTC()
	execution.Status = ExecutionStatusRunning
	execution.StartedAt = &now
	execution.UpdatedAt = now
	execution.Result.Status = ExecutionStatusRunning
	execution.Result.StartedAt = &now
	execution.Result.BackendMetadata = map[string]any{
		"backend":               string(BackendKindSubprocess),
		"networkEnforcement":    execution.Decision.EffectiveBackendKind == BackendKindSubprocess,
		"networkPolicyStrength": "declared_only",
		"processType":           backendMetaProcessKind,
	}
	m.executions[execution.ExecutionID] = cloneExecution(execution)
	m.mu.Unlock()
	if err := m.persistExecution(context.Background(), execution); err == nil {
		_ = m.publishExecutionStarted(context.Background(), execution)
	}

	result := executeSubprocess(ctx, *launch)

	completedAt := time.Now().UTC()
	execution.Status = result.Status
	execution.UpdatedAt = completedAt
	execution.CompletedAt = &completedAt
	execution.Result = Result{
		ExecutionID:     execution.ExecutionID,
		Status:          result.Status,
		StartedAt:       execution.StartedAt,
		CompletedAt:     &completedAt,
		ExitCode:        result.ExitCode,
		Signal:          result.Signal,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		OutputTruncated: result.OutputTruncated,
		ErrorClass:      result.ErrorClass,
		ErrorCode:       result.ErrorCode,
		Error:           result.Error,
		Partial:         false,
		BackendMetadata: result.BackendMetadata,
	}
	execution.StartedAt = execution.Result.StartedAt

	m.mu.Lock()
	delete(m.cancels, execution.ExecutionID)
	m.executions[execution.ExecutionID] = cloneExecution(execution)
	m.mu.Unlock()

	if err := m.persistExecution(context.Background(), execution); err == nil {
		_ = m.publishExecutionTerminal(context.Background(), execution)
	}
}

type subprocessResult struct {
	Status          ExecutionStatus
	ExitCode        *int
	Signal          string
	Stdout          string
	Stderr          string
	OutputTruncated bool
	ErrorClass      ErrorClass
	ErrorCode       string
	Error           string
	BackendMetadata map[string]any
}

func executeSubprocess(ctx context.Context, launch launchSpec) subprocessResult {
	command := exec.Command(launch.Command, launch.Args...)
	command.Dir = launch.Cwd
	command.Env = launch.Env

	stdoutCapture := newCaptureBuffer(launch.MaxOutputBytes)
	stderrCapture := newCaptureBuffer(launch.MaxOutputBytes)
	if launch.CaptureStdout {
		command.Stdout = stdoutCapture
	}
	if launch.CaptureStderr {
		command.Stderr = stderrCapture
	}
	if launch.Stdin != "" {
		command.Stdin = strings.NewReader(launch.Stdin)
	}

	startedAt := time.Now().UTC()
	if err := command.Start(); err != nil {
		return subprocessResult{
			Status:     ExecutionStatusFailed,
			ErrorClass: ErrorClassLaunchFailed,
			ErrorCode:  "sandbox_launch_failed",
			Error:      err.Error(),
			BackendMetadata: map[string]any{
				"backend": "subprocess",
			},
		}
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- command.Wait()
	}()

	select {
	case err := <-waitCh:
		completedAt := time.Now().UTC()
		metadata := map[string]any{
			"backend":      "subprocess",
			"pid":          command.Process.Pid,
			"startedAt":    startedAt.Format(time.RFC3339Nano),
			"completedAt":  completedAt.Format(time.RFC3339Nano),
			"platform":     runtime.GOOS,
			"architecture": runtime.GOARCH,
		}
		if err == nil {
			exitCode := 0
			return subprocessResult{
				Status:          ExecutionStatusCompleted,
				ExitCode:        &exitCode,
				Stdout:          stdoutCapture.String(),
				Stderr:          stderrCapture.String(),
				OutputTruncated: stdoutCapture.Truncated() || stderrCapture.Truncated(),
				BackendMetadata: metadata,
			}
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode := exitErr.ExitCode()
			return subprocessResult{
				Status:          ExecutionStatusFailed,
				ExitCode:        &exitCode,
				Stdout:          stdoutCapture.String(),
				Stderr:          stderrCapture.String(),
				OutputTruncated: stdoutCapture.Truncated() || stderrCapture.Truncated(),
				ErrorClass:      ErrorClassProcessFailed,
				ErrorCode:       "sandbox_process_failed",
				Error:           err.Error(),
				BackendMetadata: metadata,
			}
		}
		return subprocessResult{
			Status:          ExecutionStatusFailed,
			Stdout:          stdoutCapture.String(),
			Stderr:          stderrCapture.String(),
			OutputTruncated: stdoutCapture.Truncated() || stderrCapture.Truncated(),
			ErrorClass:      ErrorClassIOCaptureFailed,
			ErrorCode:       "sandbox_wait_failed",
			Error:           err.Error(),
			BackendMetadata: metadata,
		}
	case <-ctx.Done():
		signal := ""
		if command.Process != nil {
			_ = command.Process.Signal(os.Interrupt)
			signal = "interrupt"
		}
		select {
		case err := <-waitCh:
			_ = err
		case <-time.After(maxDuration(launch.KillGrace, time.Second)):
			if command.Process != nil {
				_ = command.Process.Kill()
				signal = "kill"
			}
			<-waitCh
		}
		errorClass := ErrorClassCancelled
		errorCode := "sandbox_cancelled"
		errorText := "execution was cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			errorClass = ErrorClassTimeout
			errorCode = "sandbox_timeout"
			errorText = "execution timed out"
		}
		return subprocessResult{
			Status:          statusForContext(ctx),
			Stdout:          stdoutCapture.String(),
			Stderr:          stderrCapture.String(),
			Signal:          signal,
			OutputTruncated: stdoutCapture.Truncated() || stderrCapture.Truncated(),
			ErrorClass:      errorClass,
			ErrorCode:       errorCode,
			Error:           errorText,
			BackendMetadata: map[string]any{"backend": "subprocess"},
		}
	}
}

type captureBuffer struct {
	limit     int
	size      int
	truncated bool
	buf       bytes.Buffer
}

func newCaptureBuffer(limit int) *captureBuffer {
	if limit <= 0 {
		limit = 64 * 1024
	}
	return &captureBuffer{limit: limit}
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	b.size += len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
			b.truncated = true
		} else {
			_, _ = b.buf.Write(p)
		}
	} else {
		b.truncated = true
	}
	return len(p), nil
}

func (b *captureBuffer) String() string {
	return strings.TrimSpace(b.buf.String())
}

func (b *captureBuffer) Truncated() bool {
	return b.truncated || b.size > b.limit
}

func (m *Manager) persistExecution(ctx context.Context, execution Execution) error {
	if m.store == nil {
		return nil
	}
	document, err := json.Marshal(execution)
	if err != nil {
		return fmt.Errorf("marshal sandbox execution %s: %w", execution.ExecutionID, err)
	}
	return m.store.UpsertSandboxExecution(ctx, store.SandboxExecutionRecord{
		ExecutionID: execution.ExecutionID,
		ProfileID:   execution.ProfileID,
		BackendKind: string(execution.BackendKind),
		Status:      string(execution.Status),
		ApprovalID:  execution.ApprovalID,
		RequestedAt: execution.RequestedAt,
		UpdatedAt:   execution.UpdatedAt,
		StartedAt:   execution.StartedAt,
		CompletedAt: execution.CompletedAt,
		Document:    document,
	})
}

func (m *Manager) persistApprovalArtifacts(ctx context.Context, approvalID string, decision policy.Decision) error {
	if m.store == nil || approvalID == "" {
		return nil
	}
	approval, ok := m.policy.GetApproval(approvalID)
	if !ok {
		return nil
	}
	if err := m.store.UpsertApproval(ctx, approval); err != nil {
		return fmt.Errorf("persist sandbox approval %s: %w", approvalID, err)
	}
	if err := m.store.UpsertDecision(ctx, decision); err != nil {
		return fmt.Errorf("persist sandbox approval decision %s: %w", decision.DecisionID, err)
	}
	return nil
}

func (m *Manager) publishApprovalRequested(ctx context.Context, approvalID string, decision policy.Decision) error {
	approval, ok := m.policy.GetApproval(approvalID)
	if !ok {
		return nil
	}
	if _, err := m.publishEvent(ctx, events.Event{
		Category: "policy",
		Name:     "policy.approval_requested",
		Resource: events.Resource{Kind: "approval", ID: approval.ApprovalID},
		Payload: map[string]any{
			"action":       approval.Action,
			"resourceKind": approval.ResourceKind,
			"resourceId":   approval.ResourceID,
			"reason":       approval.Reason,
			"requestedBy":  approval.RequestedBy,
			"status":       approval.Status,
		},
	}); err != nil {
		return err
	}
	_, err := m.publishEvent(ctx, events.Event{
		Category: "policy",
		Name:     "policy.decision_recorded",
		Resource: events.Resource{Kind: "decision", ID: decision.DecisionID},
		Payload: map[string]any{
			"action":       decision.Action,
			"resourceKind": decision.ResourceKind,
			"resourceId":   decision.ResourceID,
			"outcome":      decision.Outcome,
			"reason":       decision.Reason,
			"approvalId":   decision.ApprovalID,
		},
	})
	return err
}

func (m *Manager) publishExecutionRequested(ctx context.Context, execution Execution) error {
	_, err := m.publishEvent(ctx, events.Event{
		Category: eventCategory,
		Name:     "sandbox.execution_requested",
		Resource: events.Resource{Kind: resourceKindExecution, ID: execution.ExecutionID},
		Payload: map[string]any{
			"profileId":    execution.ProfileID,
			"backendKind":  execution.BackendKind,
			"command":      execution.Command,
			"args":         execution.Args,
			"cwd":          execution.Cwd,
			"requestedBy":  execution.RequestedBy,
			"resourceKind": execution.ResourceKind,
			"resourceId":   execution.ResourceID,
			"scope":        execution.Scope,
			"status":       execution.Status,
		},
	})
	return err
}

func (m *Manager) publishDecisionRecorded(ctx context.Context, execution Execution) error {
	_, err := m.publishEvent(ctx, events.Event{
		Category: eventCategory,
		Name:     "sandbox.decision_recorded",
		Resource: events.Resource{Kind: resourceKindExecution, ID: execution.ExecutionID},
		Payload: map[string]any{
			"decisionId":           execution.Decision.DecisionID,
			"resolution":           execution.Decision.Resolution,
			"matchedRules":         execution.Decision.MatchedRules,
			"approvalRequired":     execution.Decision.ApprovalRequired,
			"approvalStatus":       execution.Decision.ApprovalStatus,
			"effectiveProfileId":   execution.Decision.EffectiveProfileID,
			"effectiveBackendKind": execution.Decision.EffectiveBackendKind,
			"explanation":          execution.Decision.Explanation,
		},
	})
	return err
}

func (m *Manager) publishExecutionStarted(ctx context.Context, execution Execution) error {
	_, err := m.publishEvent(ctx, events.Event{
		Category: eventCategory,
		Name:     "sandbox.execution_started",
		Resource: events.Resource{Kind: resourceKindExecution, ID: execution.ExecutionID},
		Payload: map[string]any{
			"profileId":   execution.ProfileID,
			"backendKind": execution.BackendKind,
			"status":      execution.Status,
			"startedAt":   execution.StartedAt,
		},
	})
	return err
}

func (m *Manager) publishExecutionTerminal(ctx context.Context, execution Execution) error {
	name := "sandbox.execution_failed"
	switch execution.Status {
	case ExecutionStatusCompleted:
		name = "sandbox.execution_completed"
	case ExecutionStatusCancelled:
		name = "sandbox.execution_cancelled"
	case ExecutionStatusDenied:
		name = "sandbox.execution_denied"
	}
	payload := map[string]any{
		"profileId":       execution.ProfileID,
		"backendKind":     execution.BackendKind,
		"status":          execution.Status,
		"exitCode":        execution.Result.ExitCode,
		"signal":          execution.Result.Signal,
		"outputTruncated": execution.Result.OutputTruncated,
		"errorClass":      execution.Result.ErrorClass,
		"errorCode":       execution.Result.ErrorCode,
		"error":           execution.Result.Error,
		"partial":         execution.Result.Partial,
	}
	if execution.Result.CompletedAt != nil {
		payload["completedAt"] = execution.Result.CompletedAt
	}
	_, err := m.publishEvent(ctx, events.Event{
		Category: eventCategory,
		Name:     name,
		Resource: events.Resource{Kind: resourceKindExecution, ID: execution.ExecutionID},
		Payload:  payload,
	})
	return err
}

func (m *Manager) publishEvent(ctx context.Context, event events.Event) (events.Event, error) {
	if event.EventID == "" {
		event.EventID = newID("evt")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	if m.store != nil {
		persisted, err := m.store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, fmt.Errorf("append sandbox event %s: %w", event.Name, err)
		}
		event = persisted
	}
	if m.eventBus != nil {
		event = m.eventBus.Publish(event)
	}
	return event, nil
}

func (m *Manager) hasActiveExecutions() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, executionID := range m.executionIDs {
		if !IsTerminal(m.executions[executionID].Status) {
			return true
		}
	}
	return false
}

func (m *Manager) storeExecution(execution Execution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.executions[execution.ExecutionID]; !ok {
		m.executionIDs = append(m.executionIDs, execution.ExecutionID)
	}
	m.executions[execution.ExecutionID] = cloneExecution(execution)
}

func (m *Manager) reloadBuiltins() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadBuiltinsLocked()
}

func (m *Manager) reloadBuiltinsLocked() {
	builtins := builtinProfiles(m.cfg)
	m.profiles = make(map[string]Profile, len(builtins))
	m.profileIDs = make([]string, 0, len(builtins))
	for _, profile := range builtins {
		m.profiles[profile.ProfileID] = profile
		m.profileIDs = append(m.profileIDs, profile.ProfileID)
	}
}

func (m *Manager) listProfilesLocked() []Profile {
	items := make([]Profile, 0, len(m.profileIDs))
	for _, profileID := range m.profileIDs {
		items = append(items, cloneProfile(m.profiles[profileID]))
	}
	return items
}

func builtinProfiles(cfg config.Config) []Profile {
	dataDir := strings.TrimSpace(cfg.DataDir)
	homeDir, _ := os.UserHomeDir()
	agentsDir := filepath.Join(homeDir, ".agents")
	tempRoot := os.TempDir()

	return []Profile{
		{
			ProfileID:      ProfileIDSubprocessDefault,
			Title:          "Default Subprocess Sandbox",
			Description:    "Conservative local subprocess execution for the harness control plane.",
			BackendKind:    BackendKindSubprocess,
			DefaultWorkDir: dataDir,
			FilesystemPolicy: FilesystemPolicy{
				Mode:               FilesystemModeScoped,
				ReadRoots:          []string{dataDir},
				WriteRoots:         []string{dataDir},
				TempRoots:          []string{tempRoot},
				AllowDataDir:       true,
				AllowUserAgentsDir: true,
				AllowHomeRead:      false,
				AllowHomeWrite:     false,
			},
			NetworkPolicy: NetworkPolicy{
				Mode:            NetworkModeDeny,
				AllowedHosts:    []string{},
				AllowedPorts:    []int{},
				AllowLoopback:   false,
				EnforcementMode: "declared_only",
			},
			EnvPolicy: EnvironmentPolicy{
				Mode:        EnvironmentModeInheritSafe,
				AllowedVars: []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TMP", "TEMP", "TERM"},
				InjectedVars: map[string]string{
					"DOPE_DATA_DIR":   dataDir,
					"DOPE_ENV":        string(cfg.Environment),
					"HOME":            homeDir,
					"DOPE_AGENTS_DIR": agentsDir,
				},
				RedactedVars: []string{},
			},
			ApprovalPolicy: ApprovalPolicy{
				Mode:                          ApprovalModeAsk,
				RequiredForCommands:           []string{"curl", "ssh", "scp", "rm"},
				RequiredForWritesOutsideRoots: true,
				RequiredForNetwork:            true,
				RequiredForUnknownBackends:    true,
			},
			ProcessPolicy: ProcessPolicy{
				TimeoutMs:        30000,
				MaxTimeoutMs:     300000,
				KillGraceMs:      1000,
				CaptureStdout:    true,
				CaptureStderr:    true,
				MaxOutputBytes:   65536,
				AllowStreaming:   false,
				RestartOnFailure: false,
			},
			DefaultTimeoutMs: 30000,
			MaxTimeoutMs:     300000,
			Restartable:      false,
			Source:           SourceBuiltin,
			Active:           true,
		},
		{
			ProfileID:      ProfileIDManagedProviderClaude,
			Title:          "Claude Managed Provider",
			Description:    "Sandbox policy for the Claude managed CLI bridge.",
			BackendKind:    BackendKindSubprocess,
			DefaultWorkDir: resolveManagedWorkDir(homeDir, cfg.LLM.Claude.WorkDir),
			FilesystemPolicy: FilesystemPolicy{
				Mode:               FilesystemModeScoped,
				ReadRoots:          normalizeRootList([]string{resolveManagedWorkDir(homeDir, cfg.LLM.Claude.WorkDir), filepath.Join(homeDir, ".claude")}),
				WriteRoots:         normalizeRootList([]string{resolveManagedWorkDir(homeDir, cfg.LLM.Claude.WorkDir), filepath.Join(homeDir, ".claude")}),
				TempRoots:          []string{tempRoot},
				AllowDataDir:       false,
				AllowUserAgentsDir: false,
				AllowHomeRead:      false,
				AllowHomeWrite:     false,
			},
			NetworkPolicy: NetworkPolicy{
				Mode:            NetworkModeFull,
				AllowedHosts:    []string{},
				AllowedPorts:    []int{},
				AllowLoopback:   true,
				EnforcementMode: "declared_only",
			},
			EnvPolicy: EnvironmentPolicy{
				Mode: EnvironmentModeInheritSafe,
				AllowedVars: []string{
					"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TMP", "TEMP", "TERM", "LANG", "LC_ALL",
				},
				InjectedVars: map[string]string{
					"HOME":     homeDir,
					"DOPE_ENV": string(cfg.Environment),
				},
				RedactedVars: []string{},
			},
			ApprovalPolicy: ApprovalPolicy{
				Mode:                          ApprovalModeAllow,
				RequiredForCommands:           []string{},
				RequiredForWritesOutsideRoots: false,
				RequiredForNetwork:            false,
				RequiredForUnknownBackends:    false,
			},
			ProcessPolicy: ProcessPolicy{
				TimeoutMs:        120000,
				MaxTimeoutMs:     300000,
				KillGraceMs:      1000,
				CaptureStdout:    true,
				CaptureStderr:    true,
				MaxOutputBytes:   262144,
				AllowStreaming:   false,
				RestartOnFailure: false,
			},
			DefaultTimeoutMs: 120000,
			MaxTimeoutMs:     300000,
			Restartable:      false,
			Source:           SourceBuiltin,
			Active:           true,
		},
		{
			ProfileID:      ProfileIDManagedProviderCodex,
			Title:          "Codex Managed Provider",
			Description:    "Sandbox policy for the Codex managed CLI bridge.",
			BackendKind:    BackendKindSubprocess,
			DefaultWorkDir: resolveManagedWorkDir(homeDir, cfg.LLM.Codex.WorkDir),
			FilesystemPolicy: FilesystemPolicy{
				Mode:               FilesystemModeScoped,
				ReadRoots:          normalizeRootList([]string{resolveManagedWorkDir(homeDir, cfg.LLM.Codex.WorkDir), filepath.Join(homeDir, ".codex")}),
				WriteRoots:         normalizeRootList([]string{resolveManagedWorkDir(homeDir, cfg.LLM.Codex.WorkDir), filepath.Join(homeDir, ".codex")}),
				TempRoots:          []string{tempRoot},
				AllowDataDir:       false,
				AllowUserAgentsDir: false,
				AllowHomeRead:      false,
				AllowHomeWrite:     false,
			},
			NetworkPolicy: NetworkPolicy{
				Mode:            NetworkModeFull,
				AllowedHosts:    []string{},
				AllowedPorts:    []int{},
				AllowLoopback:   true,
				EnforcementMode: "declared_only",
			},
			EnvPolicy: EnvironmentPolicy{
				Mode: EnvironmentModeInheritSafe,
				AllowedVars: []string{
					"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TMP", "TEMP", "TERM", "LANG", "LC_ALL",
				},
				InjectedVars: map[string]string{
					"HOME":     homeDir,
					"DOPE_ENV": string(cfg.Environment),
				},
				RedactedVars: []string{},
			},
			ApprovalPolicy: ApprovalPolicy{
				Mode:                          ApprovalModeAllow,
				RequiredForCommands:           []string{},
				RequiredForWritesOutsideRoots: false,
				RequiredForNetwork:            false,
				RequiredForUnknownBackends:    false,
			},
			ProcessPolicy: ProcessPolicy{
				TimeoutMs:        120000,
				MaxTimeoutMs:     300000,
				KillGraceMs:      1000,
				CaptureStdout:    true,
				CaptureStderr:    true,
				MaxOutputBytes:   262144,
				AllowStreaming:   false,
				RestartOnFailure: false,
			},
			DefaultTimeoutMs: 120000,
			MaxTimeoutMs:     300000,
			Restartable:      false,
			Source:           SourceBuiltin,
			Active:           true,
		},
	}
}

func commandApprovalRule(profile Profile, command string) string {
	base := filepath.Base(strings.TrimSpace(command))
	for _, required := range profile.ApprovalPolicy.RequiredForCommands {
		if strings.EqualFold(strings.TrimSpace(required), strings.TrimSpace(command)) || strings.EqualFold(strings.TrimSpace(required), base) {
			return "command:approval_required"
		}
	}
	return ""
}

func evaluateFilesystem(profile Profile, cwd string, access AccessRequest) (DecisionResolution, string, error) {
	switch profile.FilesystemPolicy.Mode {
	case FilesystemModeFull:
		return DecisionResolutionAllow, "filesystem:full_access", nil
	case FilesystemModeNone:
		if cwd != "" || len(access.ReadRoots) > 0 || len(access.WriteRoots) > 0 {
			return DecisionResolutionDeny, "filesystem:none", nil
		}
		return DecisionResolutionAllow, "filesystem:none", nil
	}

	readRoots := effectiveReadRoots(profile)
	writeRoots := effectiveWriteRoots(profile)
	if cwd != "" && !withinAny(cwd, append(readRoots, writeRoots...)) {
		return DecisionResolutionDeny, "filesystem:cwd_outside_scoped_roots", nil
	}
	for _, root := range access.ReadRoots {
		if !withinAny(root, readRoots) && !withinAny(root, writeRoots) {
			return DecisionResolutionDeny, "filesystem:read_outside_scoped_roots", nil
		}
	}
	for _, root := range access.WriteRoots {
		if !withinAny(root, writeRoots) {
			if profile.ApprovalPolicy.RequiredForWritesOutsideRoots {
				return DecisionResolutionAsk, "filesystem:write_outside_roots_requires_approval", nil
			}
			return DecisionResolutionDeny, "filesystem:write_outside_scoped_roots", nil
		}
	}
	if cwd != "" && !withinAny(cwd, writeRoots) && profile.ApprovalPolicy.RequiredForWritesOutsideRoots {
		return DecisionResolutionAsk, "filesystem:cwd_write_scope_requires_approval", nil
	}
	return DecisionResolutionAllow, "filesystem:scoped", nil
}

func evaluateNetwork(profile Profile, access AccessRequest) (DecisionResolution, string) {
	if access.NetworkMode == "" || access.NetworkMode == NetworkModeDeny {
		return DecisionResolutionAllow, "network:none"
	}

	switch profile.NetworkPolicy.Mode {
	case NetworkModeFull:
		return DecisionResolutionAllow, "network:full"
	case NetworkModeDeny:
		if profile.ApprovalPolicy.RequiredForNetwork {
			return DecisionResolutionAsk, "network:approval_required"
		}
		return DecisionResolutionDeny, "network:denied"
	case NetworkModeAllowList:
		if access.NetworkMode == NetworkModeFull {
			if profile.ApprovalPolicy.RequiredForNetwork {
				return DecisionResolutionAsk, "network:approval_required"
			}
			return DecisionResolutionDeny, "network:mode_exceeds_profile"
		}
		if access.AllowLoopback && !profile.NetworkPolicy.AllowLoopback {
			if profile.ApprovalPolicy.RequiredForNetwork {
				return DecisionResolutionAsk, "network:loopback_requires_approval"
			}
			return DecisionResolutionDeny, "network:loopback_denied"
		}
		if !subsetStrings(access.AllowedHosts, profile.NetworkPolicy.AllowedHosts) || !subsetInts(access.AllowedPorts, profile.NetworkPolicy.AllowedPorts) {
			if profile.ApprovalPolicy.RequiredForNetwork {
				return DecisionResolutionAsk, "network:allow_list_requires_approval"
			}
			return DecisionResolutionDeny, "network:allow_list_denied"
		}
		return DecisionResolutionAllow, "network:allow_list"
	default:
		return DecisionResolutionDeny, "network:unknown_mode"
	}
}

func buildEnvironment(profile Profile, requestEnv map[string]string) []string {
	env := map[string]string{}
	switch profile.EnvPolicy.Mode {
	case EnvironmentModeInheritAll:
		for _, item := range os.Environ() {
			key, value, ok := strings.Cut(item, "=")
			if ok {
				env[key] = value
			}
		}
	case EnvironmentModeInheritSafe:
		allowed := make(map[string]struct{}, len(profile.EnvPolicy.AllowedVars))
		for _, key := range profile.EnvPolicy.AllowedVars {
			allowed[key] = struct{}{}
		}
		for _, item := range os.Environ() {
			key, value, ok := strings.Cut(item, "=")
			if !ok {
				continue
			}
			if _, ok := allowed[key]; ok {
				env[key] = value
			}
		}
	}
	for key, value := range profile.EnvPolicy.InjectedVars {
		env[key] = value
	}
	for key, value := range requestEnv {
		env[key] = value
	}
	items := make([]string, 0, len(env))
	for _, key := range sortedKeys(env) {
		items = append(items, key+"="+env[key])
	}
	return items
}

func effectiveTimeout(profile Profile, requested int) int {
	timeout := profile.DefaultTimeoutMs
	if timeout <= 0 {
		timeout = profile.ProcessPolicy.TimeoutMs
	}
	if timeout <= 0 {
		timeout = 30000
	}
	if requested > 0 {
		timeout = requested
	}
	maxTimeout := profile.MaxTimeoutMs
	if maxTimeout <= 0 {
		maxTimeout = profile.ProcessPolicy.MaxTimeoutMs
	}
	if maxTimeout > 0 && timeout > maxTimeout {
		return maxTimeout
	}
	return timeout
}

func effectiveReadRoots(profile Profile) []string {
	roots := cloneStrings(profile.FilesystemPolicy.ReadRoots)
	if profile.FilesystemPolicy.AllowDataDir {
		roots = append(roots, strings.TrimSpace(profile.DefaultWorkDir))
	}
	if profile.FilesystemPolicy.AllowUserAgentsDir {
		homeDir, _ := os.UserHomeDir()
		roots = append(roots, filepath.Join(homeDir, ".agents"))
	}
	if profile.FilesystemPolicy.AllowHomeRead {
		homeDir, _ := os.UserHomeDir()
		roots = append(roots, homeDir)
	}
	for _, root := range profile.FilesystemPolicy.TempRoots {
		roots = append(roots, root)
	}
	return normalizeRootList(roots)
}

func effectiveWriteRoots(profile Profile) []string {
	roots := cloneStrings(profile.FilesystemPolicy.WriteRoots)
	if profile.FilesystemPolicy.AllowDataDir {
		roots = append(roots, strings.TrimSpace(profile.DefaultWorkDir))
	}
	if profile.FilesystemPolicy.AllowHomeWrite {
		homeDir, _ := os.UserHomeDir()
		roots = append(roots, homeDir)
	}
	for _, root := range profile.FilesystemPolicy.TempRoots {
		roots = append(roots, root)
	}
	return normalizeRootList(roots)
}

func normalizePaths(base string, values []string) ([]string, error) {
	items := make([]string, 0, len(values))
	for _, value := range values {
		path, err := normalizePath(base, value)
		if err != nil {
			return nil, err
		}
		if path != "" {
			items = append(items, path)
		}
	}
	return normalizeRootList(items), nil
}

func resolveManagedWorkDir(homeDir, configured string) string {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" {
		if strings.TrimSpace(homeDir) == "" {
			return "."
		}
		return filepath.Clean(homeDir)
	}
	resolved, err := config.ResolveDir(trimmed)
	if err != nil {
		if strings.TrimSpace(homeDir) == "" {
			return "."
		}
		return filepath.Clean(homeDir)
	}
	return filepath.Clean(resolved)
}

func normalizePath(base string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return strings.TrimSpace(base), nil
	}
	resolved, err := config.ResolveDir(trimmed)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(resolved) {
		if strings.TrimSpace(base) == "" {
			return filepath.Clean(resolved), nil
		}
		return filepath.Clean(filepath.Join(base, resolved)), nil
	}
	return filepath.Clean(resolved), nil
}

func normalizeRootList(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := filepath.Clean(strings.TrimSpace(value))
		if trimmed == "." || trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	slices.Sort(items)
	return items
}

func withinAny(path string, roots []string) bool {
	if path == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if cleanPath == cleanRoot {
			return true
		}
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func subsetStrings(values []string, allowed []string) bool {
	allowedSet := map[string]struct{}{}
	for _, item := range allowed {
		allowedSet[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range values {
		if _, ok := allowedSet[strings.TrimSpace(item)]; !ok {
			return false
		}
	}
	return true
}

func subsetInts(values []int, allowed []int) bool {
	allowedSet := map[int]struct{}{}
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	for _, item := range values {
		if _, ok := allowedSet[item]; !ok {
			return false
		}
	}
	return true
}

func decisionToStatus(decision Decision) ExecutionStatus {
	if decision.Resolution == DecisionResolutionAllow {
		return ExecutionStatusPending
	}
	return ExecutionStatusDenied
}

func decisionErrorClass(decision Decision) ErrorClass {
	switch {
	case decision.ApprovalStatus == DecisionApprovalStatusRejected:
		return ErrorClassApprovalRejected
	case decision.ApprovalRequired:
		return ErrorClassApprovalRequired
	default:
		return ErrorClassPolicyDenied
	}
}

func decisionErrorCode(decision Decision) string {
	switch {
	case decision.ApprovalStatus == DecisionApprovalStatusRejected:
		return "sandbox_approval_rejected"
	case decision.ApprovalRequired:
		return "sandbox_approval_required"
	default:
		return "sandbox_policy_denied"
	}
}

func statusForContext(ctx context.Context) ExecutionStatus {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ExecutionStatusFailed
	}
	return ExecutionStatusCancelled
}

func maxDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeProfileID(value string) string {
	return strings.TrimSpace(value)
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func cloneProfile(profile Profile) Profile {
	profile.FilesystemPolicy.ReadRoots = cloneStrings(profile.FilesystemPolicy.ReadRoots)
	profile.FilesystemPolicy.WriteRoots = cloneStrings(profile.FilesystemPolicy.WriteRoots)
	profile.FilesystemPolicy.TempRoots = cloneStrings(profile.FilesystemPolicy.TempRoots)
	profile.NetworkPolicy.AllowedHosts = cloneStrings(profile.NetworkPolicy.AllowedHosts)
	profile.NetworkPolicy.AllowedPorts = cloneInts(profile.NetworkPolicy.AllowedPorts)
	profile.EnvPolicy.AllowedVars = cloneStrings(profile.EnvPolicy.AllowedVars)
	profile.EnvPolicy.InjectedVars = cloneStringMap(profile.EnvPolicy.InjectedVars)
	profile.EnvPolicy.RedactedVars = cloneStrings(profile.EnvPolicy.RedactedVars)
	profile.ApprovalPolicy.RequiredForCommands = cloneStrings(profile.ApprovalPolicy.RequiredForCommands)
	return profile
}

func cloneExecution(execution Execution) Execution {
	execution.Args = cloneStrings(execution.Args)
	execution.EnvKeys = cloneStrings(execution.EnvKeys)
	execution.Metadata = cloneStringMap(execution.Metadata)
	execution.Access = cloneAccess(execution.Access)
	execution.Decision.MatchedRules = cloneStrings(execution.Decision.MatchedRules)
	if execution.Result.BackendMetadata != nil {
		copyMap := make(map[string]any, len(execution.Result.BackendMetadata))
		for key, value := range execution.Result.BackendMetadata {
			copyMap[key] = value
		}
		execution.Result.BackendMetadata = copyMap
	}
	return execution
}

func cloneAccess(access AccessRequest) AccessRequest {
	access.ReadRoots = cloneStrings(access.ReadRoots)
	access.WriteRoots = cloneStrings(access.WriteRoots)
	access.AllowedHosts = cloneStrings(access.AllowedHosts)
	access.AllowedPorts = cloneInts(access.AllowedPorts)
	return access
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func cloneInts(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	return append([]int(nil), values...)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func sortedKeys[V any](values map[string]V) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

var _ io.Writer = (*captureBuffer)(nil)
var _ = syscall.Signal(0)
