package sandbox

import (
	"context"
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
