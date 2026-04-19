package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestExplainRequiresApprovalForNetwork(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		Command: "echo",
		Args:    []string{"hello"},
		Access: AccessRequest{
			NetworkMode: NetworkModeFull,
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.Resolution != DecisionResolutionAsk {
		t.Fatalf("expected ask decision, got %s", decision.Resolution)
	}
	if !decision.ApprovalRequired {
		t.Fatal("expected approval requirement")
	}
}

func TestStartExecutionCompletesAndPersists(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		Command: testShell(),
		Args:    testShellArgs("printf 'hello sandbox'"),
		Cwd:     cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager, execution.ExecutionID)
	if execution.Status != ExecutionStatusCompleted {
		t.Fatalf("expected completed execution, got %s (%s)", execution.Status, execution.Result.Error)
	}
	if execution.Result.Stdout != "hello sandbox" {
		t.Fatalf("expected stdout hello sandbox, got %q", execution.Result.Stdout)
	}

	records, err := manager.store.ListSandboxExecutions(context.Background())
	if err != nil {
		t.Fatalf("ListSandboxExecutions returned error: %v", err)
	}
	if len(records) != 1 || records[0].ExecutionID != execution.ExecutionID {
		t.Fatalf("expected persisted sandbox execution, got %+v", records)
	}

	sandboxEvents := manager.eventBus.List(events.Filter{Category: "sandbox"})
	if len(sandboxEvents) < 4 {
		t.Fatalf("expected sandbox lifecycle events, got %d", len(sandboxEvents))
	}
	if sandboxEvents[0].Name != "sandbox.execution_requested" {
		t.Fatalf("expected sandbox.execution_requested, got %s", sandboxEvents[0].Name)
	}
}

func TestStartExecutionCreatesApprovalAndDeniesUntilApproved(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		Command: "echo",
		Args:    []string{"needs-approval"},
		Cwd:     cwd,
		Access: AccessRequest{
			ReadRoots:   []string{cwd},
			NetworkMode: NetworkModeFull,
		},
		Reason: "network access requested",
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if execution.Status != ExecutionStatusDenied {
		t.Fatalf("expected denied execution, got %s", execution.Status)
	}
	if execution.ApprovalID == "" {
		t.Fatal("expected approval id on denied execution")
	}
	if execution.Result.ErrorClass != ErrorClassApprovalRequired {
		t.Fatalf("expected approval_required error class, got %s", execution.Result.ErrorClass)
	}

	approvals, err := manager.store.ListApprovals(context.Background())
	if err != nil {
		t.Fatalf("ListApprovals returned error: %v", err)
	}
	if len(approvals) != 1 || approvals[0].ApprovalID != execution.ApprovalID {
		t.Fatalf("expected persisted approval, got %+v", approvals)
	}
}

func TestEvaluateAccessDistinguishesDeclaredManagedProviderRoots(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	homeDir := t.TempDir()
	previousHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("Setenv returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("HOME", previousHome)
	})

	manager.Reload()

	allowedPath := filepath.Join(homeDir, ".codex", "auth.json")
	deniedPath := filepath.Join(string(filepath.Separator), "etc", "passwd")

	allowed, err := manager.EvaluateAccess(ProfileIDManagedProviderCodex, "", AccessRequest{
		ReadRoots: []string{allowedPath},
	})
	if err != nil {
		t.Fatalf("EvaluateAccess allowed returned error: %v", err)
	}
	if allowed.Resolution != DecisionResolutionAllow {
		t.Fatalf("expected allowed managed-provider access, got %+v", allowed)
	}

	denied, err := manager.EvaluateAccess(ProfileIDManagedProviderCodex, "", AccessRequest{
		ReadRoots: []string{deniedPath},
	})
	if err != nil {
		t.Fatalf("EvaluateAccess denied returned error: %v", err)
	}
	if denied.Resolution != DecisionResolutionDeny {
		t.Fatalf("expected denied managed-provider access, got %+v", denied)
	}
}

func TestManagedProviderProfilesUseIsolatedHomeInTestEnvironment(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	profile, ok := manager.GetProfile(ProfileIDManagedProviderClaude)
	if !ok {
		t.Fatal("expected claude managed-provider profile")
	}
	wantHome := filepath.Join(manager.cfg.DataDir, "managed-provider-home")
	if profile.DefaultWorkDir != wantHome {
		t.Fatalf("expected isolated managed-provider workdir %s, got %s", wantHome, profile.DefaultWorkDir)
	}
	if !withinAny(filepath.Join(wantHome, ".claude"), profile.FilesystemPolicy.ReadRoots) {
		t.Fatalf("expected managed-provider home roots, got %+v", profile.FilesystemPolicy.ReadRoots)
	}
}

func TestRestoreCancelsPendingManagedProviderFinalization(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	store1, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	manager1 := NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
	}, store1, events.NewBus(), policy.NewEngine())

	cwd := t.TempDir()
	execution, err := manager1.StartExecution(context.Background(), ExecutionRequest{
		Command: testShell(),
		Args:    testShellArgs("printf 'hello managed provider'"),
		Cwd:     cwd,
		Metadata: map[string]string{
			"managedProviderId":          "claude_managed",
			"managedProviderAction":      "prompt_execution",
			"managedProviderOperationId": "managed_provider_op_1",
		},
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager1, execution.ExecutionID)
	if execution.Status != ExecutionStatusCompleted {
		t.Fatalf("expected subprocess completion before finalization, got %s", execution.Status)
	}
	if !awaitsManagedProviderFinalization(execution) {
		t.Fatalf("expected pending managed-provider finalization metadata, got %+v", execution.Metadata)
	}
	for _, event := range manager1.eventBus.List(events.Filter{Category: "sandbox"}) {
		if event.Name == "sandbox.execution_completed" {
			t.Fatal("did not expect terminal completion event before provider finalization")
		}
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store2, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("reopen SQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store2.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	eventBus2 := events.NewBus()
	manager2 := NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
	}, store2, eventBus2, policy.NewEngine())
	if err := manager2.Restore(context.Background()); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	restored, ok := manager2.GetExecution(execution.ExecutionID)
	if !ok {
		t.Fatalf("expected restored execution %s", execution.ExecutionID)
	}
	if restored.Status != ExecutionStatusCancelled {
		t.Fatalf("expected cancelled execution after recovery, got %s", restored.Status)
	}
	if restored.Result.ErrorCode != "daemon_restarted_before_consumer_finalization" {
		t.Fatalf("expected finalization recovery code, got %+v", restored.Result)
	}
	if awaitsManagedProviderFinalization(restored) {
		t.Fatalf("expected pending finalization marker cleared, got %+v", restored.Metadata)
	}
	foundCancelled := false
	for _, event := range eventBus2.List(events.Filter{Category: "sandbox"}) {
		if event.Name == "sandbox.execution_cancelled" {
			foundCancelled = true
		}
	}
	if !foundCancelled {
		t.Fatal("expected sandbox.execution_cancelled event on recovery")
	}
}

func TestCancelExecutionTransitionsToCancelled(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		Command: testShell(),
		Args:    testShellArgs(testSleepScript()),
		Cwd:     cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, _, err := manager.CancelExecution(execution.ExecutionID); err != nil {
		t.Fatalf("CancelExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager, execution.ExecutionID)
	if execution.Status != ExecutionStatusCancelled {
		t.Fatalf("expected cancelled execution, got %s (%s)", execution.Status, execution.Result.Error)
	}
}

func newSandboxManagerForTest(t *testing.T) *Manager {
	t.Helper()

	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	return NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
	}, sqliteStore, events.NewBus(), policy.NewEngine())
}

func waitForTerminalExecution(t *testing.T, manager *Manager, executionID string) Execution {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		execution, ok := manager.GetExecution(executionID)
		if !ok {
			t.Fatalf("expected execution %s", executionID)
		}
		if IsTerminal(execution.Status) {
			return execution
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("execution %s did not reach terminal state", executionID)
	return Execution{}
}

func testShell() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "/bin/sh"
}

func testShellArgs(script string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/c", script}
	}
	return []string{"-c", script}
}

func testSleepScript() string {
	if runtime.GOOS == "windows" {
		return "ping -n 6 127.0.0.1 >NUL"
	}
	return "sleep 5"
}
