package sandbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
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

func TestListProfilesIncludesBackendCapabilityAndDockerAvailability(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	profiles := manager.ListProfiles()
	if len(profiles) == 0 {
		t.Fatal("expected builtin sandbox profiles")
	}

	var dockerProfile Profile
	foundDocker := false
	for _, profile := range profiles {
		if profile.BackendCapability.BackendKind != profile.BackendKind {
			t.Fatalf("expected backend capability on profile %+v", profile)
		}
		if profile.BackendKind == BackendKindDocker {
			dockerProfile = profile
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Fatal("expected docker builtin profile")
	}
	if dockerProfile.BackendCapability.DisplayName == "" || len(dockerProfile.BackendCapability.HostPrerequisites) == 0 {
		t.Fatalf("expected docker capability metadata, got %+v", dockerProfile.BackendCapability)
	}
}

func TestExplainReturnsUnsupportedForUnavailableDockerBackend(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   "/workspace/scripts/run.sh",
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.SelectionOutcome != BackendSelectionOutcomeUnsupported {
		t.Fatalf("expected unsupported selection outcome, got %+v", decision)
	}
	if decision.HostStatus != BackendHostStatusMissingPrerequisite {
		t.Fatalf("expected missing prerequisite host status, got %+v", decision)
	}
	if decision.MismatchReason != "backend_unavailable" {
		t.Fatalf("expected backend_unavailable mismatch reason, got %+v", decision)
	}
}

func TestExplainReturnsUnsupportedForDockerAccessRuleMismatch(t *testing.T) {
	fakeDockerOnPath(t, "available")
	manager := newSandboxManagerForTest(t)
	setBackendCapabilityForTest(t, manager, BackendKindDocker, BackendCapabilityProfile{
		BackendKind:           BackendKindDocker,
		DisplayName:           "Docker",
		FilesystemEnforcement: "container_mount_scoped",
		NetworkEnforcement:    "container_network_mode",
		EnvInjectionMode:      "container_env_injection",
		ApprovalBehavior:      "profile_and_command_policy",
		RestartBehavior:       "interrupted_execution_recovers_as_cancelled",
		HostPrerequisites:     []string{"docker CLI available on PATH"},
		AvailabilityStatus:    BackendAvailabilityStatusAvailable,
	})
	cwd := t.TempDir()

	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   "/workspace/scripts/run.sh",
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:    []string{cwd},
			WriteRoots:   []string{cwd},
			NetworkMode:  NetworkModeAllowList,
			AllowedHosts: []string{"example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.SelectionOutcome != BackendSelectionOutcomeUnsupported || decision.MismatchReason != "backend_capability_mismatch" {
		t.Fatalf("expected backend mismatch, got %+v", decision)
	}
}

func TestStartDockerExecutionUsesFakeDockerRunner(t *testing.T) {
	fakeDockerOnPath(t, "available")
	manager := newSandboxManagerForTest(t)
	setBackendCapabilityForTest(t, manager, BackendKindDocker, testAvailableDockerCapability())
	cwd := t.TempDir()
	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   filepath.Join(cwd, "scripts", "run.sh"),
		Args:      []string{"alpha"},
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:   []string{cwd},
			WriteRoots:  []string{cwd},
			NetworkMode: NetworkModeDeny,
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager, execution.ExecutionID)
	if execution.Status != ExecutionStatusCompleted {
		t.Fatalf("expected completed docker execution, got %+v", execution)
	}
	if execution.BackendKind != BackendKindDocker {
		t.Fatalf("expected docker backend kind, got %+v", execution)
	}
	if execution.Result.BackendMetadata["backend"] != string(BackendKindDocker) {
		t.Fatalf("expected docker backend metadata, got %+v", execution.Result.BackendMetadata)
	}
}

func TestExplainBackendSelectionStaysUnder100ms(t *testing.T) {
	fakeDockerOnPath(t, "available")
	manager := newSandboxManagerForTest(t)
	setBackendCapabilityForTest(t, manager, BackendKindDocker, testAvailableDockerCapability())
	cwd := t.TempDir()

	started := time.Now()
	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   "/workspace/scripts/run.sh",
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected backend selection preflight <=100ms, got %s", elapsed)
	}
	if decision.SelectionOutcome != BackendSelectionOutcomeSelected {
		t.Fatalf("expected selected decision, got %+v", decision)
	}
}

func TestExplainReturnsUnsupportedForUnavailableDockerRuntime(t *testing.T) {
	fakeDockerOnPath(t, "runtime_unavailable")
	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   "/workspace/scripts/run.sh",
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.SelectionOutcome != BackendSelectionOutcomeUnsupported {
		t.Fatalf("expected unsupported selection outcome, got %+v", decision)
	}
	if decision.HostStatus != BackendHostStatusRuntimeUnavailable {
		t.Fatalf("expected runtime_unavailable host status, got %+v", decision)
	}
	if decision.MismatchReason != "backend_unavailable" {
		t.Fatalf("expected backend_unavailable mismatch reason, got %+v", decision)
	}
}

func TestExplainReturnsUnsupportedForMissingDockerImage(t *testing.T) {
	fakeDockerOnPath(t, "image_missing")
	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()

	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDDockerDefault,
		Command:   "/workspace/scripts/run.sh",
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.SelectionOutcome != BackendSelectionOutcomeUnsupported {
		t.Fatalf("expected unsupported selection outcome, got %+v", decision)
	}
	if decision.HostStatus != BackendHostStatusMissingPrerequisite {
		t.Fatalf("expected missing_prerequisite host status, got %+v", decision)
	}
	if decision.MismatchReason != "backend_unavailable" {
		t.Fatalf("expected backend_unavailable mismatch reason, got %+v", decision)
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

	sandboxEvents := waitForSandboxEvents(t, manager.eventBus, 4)
	if sandboxEvents[0].Name != "sandbox.execution_requested" {
		t.Fatalf("expected sandbox.execution_requested, got %s", sandboxEvents[0].Name)
	}
}

func waitForSandboxEvents(t *testing.T, eventBus *events.Bus, minCount int) []events.Event {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		sandboxEvents := eventBus.List(events.Filter{Category: "sandbox"})
		if len(sandboxEvents) >= minCount {
			return sandboxEvents
		}
		time.Sleep(25 * time.Millisecond)
	}
	sandboxEvents := eventBus.List(events.Filter{Category: "sandbox"})
	t.Fatalf("expected at least %d sandbox lifecycle events, got %d", minCount, len(sandboxEvents))
	return nil
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

func TestRestorePreservesToolCallLinkageForCancelledExecution(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	store1, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	startedAt := time.Now().UTC().Add(-time.Minute)
	execution := Execution{
		ExecutionID:  "sandbox_exec_linked_restore_1",
		ProfileID:    ProfileIDSubprocessDefault,
		BackendKind:  BackendKindSubprocess,
		Command:      testShell(),
		Args:         testShellArgs(testSleepScript()),
		Cwd:          t.TempDir(),
		RequestedBy:  "test",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Scope:        "tool_call",
		Status:       ExecutionStatusRunning,
		RequestedAt:  startedAt,
		UpdatedAt:    startedAt,
		Result: Result{
			Status: ExecutionStatusRunning,
		},
		Consumer: &ConsumerContractView{
			Declaration: &ConsumerRequirementDeclaration{
				DeclarationID:               "local_tool:shell:tool_call.execute",
				ConsumerKind:                ConsumerKindLocalTool,
				ConsumerID:                  "shell",
				OperationKind:               "tool_call.execute",
				ProfileID:                   ProfileIDSubprocessDefault,
				ExecutionMode:               ExecutionModeAccessOnly,
				AllowedBackendKinds:         []BackendKind{BackendKindSubprocess},
				NetworkMode:                 NetworkModeDeny,
				ApprovalMode:                ApprovalModeAsk,
				RequiredEnforcementStrength: "declared_only",
				Active:                      true,
				Source:                      "builtin",
			},
			PolicyRecord: &ConsumerPolicyRecord{
				PolicyRecordID:     "policy_restore_linked_1",
				ConsumerKind:       ConsumerKindLocalTool,
				ConsumerID:         "shell",
				OperationKind:      "tool_call.execute",
				DeclarationID:      "local_tool:shell:tool_call.execute",
				Decision:           DecisionResolutionAllow,
				ApprovalStatus:     DecisionApprovalStatusApproved,
				SecretResolution:   SecretResolutionNotApplicable,
				SandboxExecutionID: "sandbox_exec_linked_restore_1",
				ToolCallID:         "tool_call_linked_restore_1",
				StartedAt:          startedAt,
				Status:             PolicyRecordStatusRunning,
			},
		},
	}
	document, err := json.Marshal(execution)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := store1.UpsertSandboxExecution(context.Background(), store.SandboxExecutionRecord{
		ExecutionID: execution.ExecutionID,
		Status:      string(execution.Status),
		StartedAt:   &startedAt,
		Document:    document,
	}); err != nil {
		t.Fatalf("UpsertSandboxExecution returned error: %v", err)
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
	if restored.Status != ExecutionStatusCancelled || restored.Consumer == nil || restored.Consumer.PolicyRecord == nil {
		t.Fatalf("expected cancelled restored execution with consumer linkage, got %+v", restored)
	}
	if restored.Consumer.PolicyRecord.ToolCallID != "tool_call_linked_restore_1" || restored.Consumer.PolicyRecord.SandboxExecutionID != execution.ExecutionID {
		t.Fatalf("expected restored consumer linkage, got %+v", restored.Consumer.PolicyRecord)
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

func TestExplainIncludesConsumerSecretScopeMetadata(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()
	decision, err := manager.Explain(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDSubprocessDefault,
		Command:   "echo",
		Args:      []string{"hello"},
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
		Consumer: testManagedProviderConsumerView("codex_managed", ManagedProviderActionAuthStatus, []SecretScopeOutcome{
			testSecretScopeOutcome(ConsumerKindManagedProvider, "codex_managed", "auth_file", SecretEnvironmentScopeTest, SecretResolutionUnavailable),
			testSecretScopeOutcome(ConsumerKindManagedProvider, "codex_managed", "auth_file", SecretEnvironmentScopeProd, SecretResolutionResolved),
		}),
	})
	if err != nil {
		t.Fatalf("Explain returned error: %v", err)
	}
	if decision.Consumer == nil {
		t.Fatal("expected decision consumer metadata")
	}
	if decision.Consumer.Declaration == nil {
		t.Fatalf("expected consumer declaration, got %+v", decision.Consumer)
	}
	if decision.Consumer.Declaration.ConsumerKind != ConsumerKindManagedProvider {
		t.Fatalf("expected managed provider consumer kind, got %+v", decision.Consumer.Declaration)
	}
	if len(decision.Consumer.SecretScope) != 2 {
		t.Fatalf("expected 2 secret scope outcomes, got %+v", decision.Consumer.SecretScope)
	}
	if decision.Consumer.SecretScope[0].EnvironmentScope != SecretEnvironmentScopeTest || decision.Consumer.SecretScope[0].Resolution != SecretResolutionUnavailable {
		t.Fatalf("expected test unavailable secret outcome, got %+v", decision.Consumer.SecretScope[0])
	}
	if decision.Consumer.SecretScope[1].EnvironmentScope != SecretEnvironmentScopeProd || decision.Consumer.SecretScope[1].Resolution != SecretResolutionResolved {
		t.Fatalf("expected prod resolved secret outcome, got %+v", decision.Consumer.SecretScope[1])
	}
}

func TestStartExecutionPersistsEnvironmentScopedSecretScopeBindings(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()
	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDSubprocessDefault,
		Command:   testShell(),
		Args:      testShellArgs("printf 'secret-scope-ok'"),
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
		Consumer: testManagedProviderConsumerView("claude_managed", ManagedProviderActionPromptExecution, []SecretScopeOutcome{
			testSecretScopeOutcome(ConsumerKindManagedProvider, "claude_managed", "settings_file", SecretEnvironmentScopeTest, SecretResolutionUnavailable),
			testSecretScopeOutcome(ConsumerKindManagedProvider, "claude_managed", "settings_file", SecretEnvironmentScopeProd, SecretResolutionResolved),
		}),
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager, execution.ExecutionID)
	if execution.Status != ExecutionStatusCompleted {
		t.Fatalf("expected completed execution, got %+v", execution)
	}
	if execution.Result.Consumer == nil || execution.Result.Consumer.PolicyRecord == nil {
		t.Fatalf("expected execution result consumer policy record, got %+v", execution.Result.Consumer)
	}
	if execution.Result.Consumer.PolicyRecord.SecretResolution != SecretResolutionUnavailable {
		t.Fatalf("expected unavailable secret resolution, got %+v", execution.Result.Consumer.PolicyRecord)
	}

	bindings, err := manager.store.ListSecretScopeBindings(context.Background(), string(ConsumerKindManagedProvider), "claude_managed")
	if err != nil {
		t.Fatalf("ListSecretScopeBindings returned error: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 persisted secret scope bindings, got %+v", bindings)
	}
	foundTest := false
	foundProd := false
	for _, binding := range bindings {
		switch binding.EnvironmentScope {
		case string(SecretEnvironmentScopeTest):
			foundTest = true
		case string(SecretEnvironmentScopeProd):
			foundProd = true
		}
		if string(binding.Document) == "" || strings.Contains(string(binding.Document), "secret-scope-ok") {
			t.Fatalf("expected redacted binding document, got %s", string(binding.Document))
		}
	}
	if !foundTest || !foundProd {
		t.Fatalf("expected test and prod environment bindings, got %+v", bindings)
	}
}

func TestSandboxSecretScopeUsesActiveTenantAndFailsClosed(t *testing.T) {
	manager := newSandboxManagerForTest(t)
	backend, err := secrets.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalBackend returned error: %v", err)
	}
	secretManager := secrets.NewManager(manager.store, backend)
	manager.SetSecretManager(secretManager)
	if _, err := secretManager.Create(context.Background(), secrets.CreateInput{TenantID: "ten_a", SecretRef: "SANDBOX_TOKEN", Value: "tenant-a"}); err != nil {
		t.Fatalf("create tenant A secret: %v", err)
	}
	if _, err := secretManager.Create(context.Background(), secrets.CreateInput{TenantID: "ten_b", SecretRef: "SANDBOX_TOKEN", Value: "tenant-b"}); err != nil {
		t.Fatalf("create tenant B secret: %v", err)
	}
	cwd := t.TempDir()
	request := ExecutionRequest{
		ProfileID: ProfileIDSubprocessDefault,
		Command:   "echo",
		Args:      []string{"ok"},
		Cwd:       cwd,
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
		Consumer: testManagedProviderConsumerView("sandbox_scope", ManagedProviderActionPromptExecution, []SecretScopeOutcome{
			testSecretScopeOutcome(ConsumerKindManagedProvider, "sandbox_scope", "SANDBOX_TOKEN", SecretEnvironmentScopeTest, SecretResolutionUnavailable),
		}),
	}
	denied, err := manager.Explain(context.Background(), request)
	if err != nil {
		t.Fatalf("Explain without tenant returned error: %v", err)
	}
	if denied.Resolution != DecisionResolutionDeny || denied.Consumer.SecretScope[0].Resolution != SecretResolutionDenied {
		t.Fatalf("expected missing tenant fail-closed, got %+v", denied)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_b", PrincipalID: "prn_b"})
	allowed, err := manager.Explain(ctx, request)
	if err != nil {
		t.Fatalf("Explain with tenant returned error: %v", err)
	}
	if allowed.Consumer.SecretScope[0].Resolution != SecretResolutionResolved {
		t.Fatalf("expected active tenant secret to resolve, got %+v", allowed.Consumer.SecretScope)
	}
}

func TestStartExecutionRedactsSecretValuesFromResults(t *testing.T) {
	t.Parallel()

	manager := newSandboxManagerForTest(t)
	cwd := t.TempDir()
	execution, err := manager.StartExecution(context.Background(), ExecutionRequest{
		ProfileID: ProfileIDSubprocessDefault,
		Command:   testShell(),
		Args:      testShellArgs("encoded=$(printf '%s' \"$EXEC_SKILL_TOKEN\" | base64 | tr -d '\\n'); printf '%s\\n%s' \"$EXEC_SKILL_TOKEN\" \"$encoded\""),
		Cwd:       cwd,
		Env: map[string]string{
			"EXEC_SKILL_TOKEN": "top-secret-token",
		},
		Access: AccessRequest{
			ReadRoots:  []string{cwd},
			WriteRoots: []string{cwd},
		},
		Consumer: &ConsumerContractView{
			Declaration: &ConsumerRequirementDeclaration{
				DeclarationID:               "skill:redaction-skill:tool_call.execute",
				ConsumerKind:                ConsumerKindSkill,
				ConsumerID:                  "redaction-skill",
				OperationKind:               "tool_call.execute",
				ProfileID:                   ProfileIDSubprocessDefault,
				ExecutionMode:               ExecutionModeSubprocess,
				AllowedBackendKinds:         []BackendKind{BackendKindSubprocess},
				ApprovalMode:                ApprovalModeAllow,
				RequiredEnforcementStrength: "declared_only",
				Active:                      true,
				Source:                      SourceBuiltin,
			},
			SecretScope: []SecretScopeOutcome{
				{
					ConsumerKind:     ConsumerKindSkill,
					ConsumerID:       "redaction-skill",
					SecretRef:        "EXEC_SKILL_TOKEN",
					EnvironmentScope: SecretEnvironmentScopeTest,
					DefaultSource:    SecretDefaultSourceInstanceOverride,
					DefaultRuleID:    "skill:redaction-skill",
					DeliveryKind:     "environment_variable",
					RedactionRule:    "value_redacted",
					Resolution:       SecretResolutionResolved,
				},
			},
			PolicyRecord: &ConsumerPolicyRecord{
				PolicyRecordID:   "policy_skill_redaction",
				ConsumerKind:     ConsumerKindSkill,
				ConsumerID:       "redaction-skill",
				OperationKind:    "tool_call.execute",
				Decision:         DecisionResolutionAllow,
				ApprovalStatus:   DecisionApprovalStatusNotApplicable,
				SecretResolution: SecretResolutionResolved,
				StartedAt:        time.Now().UTC(),
				Status:           PolicyRecordStatusPreflightAllowed,
			},
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	execution = waitForTerminalExecution(t, manager, execution.ExecutionID)
	if execution.Status != ExecutionStatusCompleted {
		t.Fatalf("expected completed execution, got %+v", execution)
	}
	if strings.Contains(execution.Result.Stdout, "top-secret-token") {
		t.Fatalf("expected raw secret to be redacted, got %+v", execution.Result)
	}
	encodedSecret := "dG9wLXNlY3JldC10b2tlbg=="
	if strings.Contains(execution.Result.Stdout, encodedSecret) {
		t.Fatalf("expected derived secret material to be redacted, got %+v", execution.Result)
	}
	if execution.Result.Stdout != "[REDACTED]\n[REDACTED]" {
		t.Fatalf("expected redacted stdout, got %+v", execution.Result)
	}
	if strings.Contains(execution.Result.Stderr, "top-secret-token") || strings.Contains(execution.Result.Error, "top-secret-token") {
		t.Fatalf("expected redacted error surfaces, got %+v", execution.Result)
	}

	records, err := manager.store.ListSandboxExecutions(context.Background())
	if err != nil {
		t.Fatalf("ListSandboxExecutions returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 persisted execution, got %+v", records)
	}
	if strings.Contains(string(records[0].Document), "top-secret-token") {
		t.Fatalf("persisted execution leaked secret: %s", string(records[0].Document))
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

func setBackendCapabilityForTest(t *testing.T, manager *Manager, kind BackendKind, capability BackendCapabilityProfile) {
	t.Helper()

	manager.mu.Lock()
	defer manager.mu.Unlock()

	manager.capabilities[kind] = cloneBackendCapability(capability)
	for profileID, profile := range manager.profiles {
		if profile.BackendKind != kind {
			continue
		}
		profile.BackendCapability = cloneBackendCapability(capability)
		manager.profiles[profileID] = profile
	}
}

func testAvailableDockerCapability() BackendCapabilityProfile {
	return BackendCapabilityProfile{
		BackendKind:           BackendKindDocker,
		DisplayName:           "Docker",
		FilesystemEnforcement: "container_mount_scoped",
		NetworkEnforcement:    "container_network_mode",
		EnvInjectionMode:      "container_env_injection",
		ApprovalBehavior:      "profile_and_command_policy",
		RestartBehavior:       "interrupted_execution_recovers_as_cancelled",
		HostPrerequisites:     []string{"docker CLI available on PATH"},
		AvailabilityStatus:    BackendAvailabilityStatusAvailable,
	}
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

func fakeDockerOnPath(t *testing.T, mode string) string {
	t.Helper()

	dir := t.TempDir()
	script := `#!/bin/sh
mode="${DOPE_FAKE_DOCKER_MODE:-available}"
cmd="$1"
shift
subcmd="$1"
if [ "$cmd" = "version" ]; then
  if [ "$mode" = "runtime_unavailable" ]; then
    echo "Cannot connect to the Docker daemon" >&2
    exit 1
  fi
  printf '26.1.0\n'
  exit 0
fi
if [ "$cmd" = "image" ] && [ "$subcmd" = "inspect" ]; then
  if [ "$mode" = "runtime_unavailable" ]; then
    echo "Cannot connect to the Docker daemon" >&2
    exit 1
  fi
  if [ "$mode" = "image_missing" ]; then
    echo "Error: No such image" >&2
    exit 1
  fi
  printf 'sha256:fake-image\n'
  exit 0
fi
if [ "$cmd" = "run" ]; then
  if [ "$mode" = "image_missing" ]; then
    echo "Error: No such image" >&2
    exit 1
  fi
  printf 'docker-ok'
  exit 0
fi
printf 'docker-ok'
exit 0
`
	if runtime.GOOS == "windows" {
		t.Skip("docker path fixture is not implemented for windows")
	}
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	current := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+current)
	t.Setenv("DOPE_FAKE_DOCKER_MODE", strings.TrimSpace(mode))
	return path
}

func testManagedProviderConsumerView(providerID string, action ManagedProviderActionKind, secretScope []SecretScopeOutcome) *ConsumerContractView {
	consumerID := strings.TrimSpace(providerID)
	operationKind := string(action)
	return &ConsumerContractView{
		Declaration: &ConsumerRequirementDeclaration{
			DeclarationID:               "managed_provider:" + consumerID + ":" + operationKind,
			ConsumerKind:                ConsumerKindManagedProvider,
			ConsumerID:                  consumerID,
			OperationKind:               operationKind,
			ProfileID:                   ProfileIDManagedProviderCodex,
			ExecutionMode:               ExecutionModeSubprocess,
			AllowedBackendKinds:         []BackendKind{BackendKindSubprocess},
			ReadRoots:                   []string{},
			WriteRoots:                  []string{},
			NetworkMode:                 NetworkModeDeny,
			SecretRefs:                  []string{"auth_file"},
			ApprovalMode:                ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
			Source:                      SourceBuiltin,
		},
		SecretScope: append([]SecretScopeOutcome(nil), secretScope...),
		PolicyRecord: &ConsumerPolicyRecord{
			PolicyRecordID:      "policy_" + consumerID + "_" + operationKind,
			ConsumerKind:        ConsumerKindManagedProvider,
			ConsumerID:          consumerID,
			OperationKind:       operationKind,
			DeclarationID:       "managed_provider:" + consumerID + ":" + operationKind,
			RequestedBy:         "test",
			Decision:            DecisionResolutionAllow,
			ApprovalStatus:      DecisionApprovalStatusNotApplicable,
			SecretResolution:    secretResolutionFromConsumer(&ConsumerContractView{SecretScope: secretScope}),
			EnforcementStrength: "declared_only",
			StartedAt:           time.Now().UTC(),
			Status:              PolicyRecordStatusPreflightAllowed,
		},
	}
}

func testSecretScopeOutcome(kind ConsumerKind, consumerID, secretRef string, environment SecretEnvironmentScope, resolution SecretResolution) SecretScopeOutcome {
	return SecretScopeOutcome{
		ConsumerKind:     kind,
		ConsumerID:       strings.TrimSpace(consumerID),
		SecretRef:        strings.TrimSpace(secretRef),
		EnvironmentScope: environment,
		DefaultSource:    SecretDefaultSourceInstanceOverride,
		DefaultRuleID:    string(kind) + ":" + strings.TrimSpace(consumerID),
		DeliveryKind:     "local_state_access",
		RedactionRule:    "class_summary_only",
		Resolution:       resolution,
	}
}
