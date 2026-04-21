package mcp

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/gorilla/websocket"
)

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER") != "1" {
		return
	}

	type request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}

	toolsPayload := os.Getenv("MCP_HELPER_TOOLS")
	if strings.TrimSpace(toolsPayload) == "" {
		toolsPayload = `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		payload, err := readFramedMessage(reader)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "file already closed") {
				return
			}
			fmt.Fprint(os.Stderr, err.Error())
			return
		}
		var req request
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}
		switch req.Method {
		case "initialize":
			writeHelperResponse(req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"serverInfo": map[string]any{
					"name":    "dope-test-mcp",
					"version": "1.0.0",
				},
			})
		case "notifications/initialized":
		case "tools/list":
			var tools []map[string]any
			_ = json.Unmarshal([]byte(toolsPayload), &tools)
			writeHelperResponse(req.ID, map[string]any{"tools": tools})
		case "tools/call":
			writeHelperResponse(req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "lookup ok"}},
			})
		default:
			writeHelperError(req.ID, -32601, "method not found")
		}
	}
}

func TestManagerRegistersUpdatesAndListsServers(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	server, created, err := manager.CreateServer(ctx, testServerInput(t, false))
	if err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if !created {
		t.Fatal("expected server to be created")
	}
	if server.ServerID != "test-mcp" || server.State.Status != LifecycleStatusDisabled {
		t.Fatalf("unexpected created server resource: %+v", server)
	}

	displayName := "Updated MCP"
	enabled := true
	updated, err := manager.UpdateServer(ctx, "test-mcp", UpdateServerInput{
		DisplayName: &displayName,
		Enabled:     &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateServer returned error: %v", err)
	}
	if updated.DisplayName != displayName || !updated.Enabled {
		t.Fatalf("expected updated server values, got %+v", updated)
	}

	items := manager.ListServers()
	if len(items) != 1 {
		t.Fatalf("expected 1 server, got %d", len(items))
	}
}

func TestManagerStartDiscoversToolsAndSupportsStopCancel(t *testing.T) {
	manager, sandboxes := newTestManager(t)
	ctx := context.Background()

	input := testServerInput(t, true)
	if _, _, err := manager.CreateServer(ctx, input); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	started, err := manager.Start(ctx, input.ServerID, "manager-test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.Server.State.Status != LifecycleStatusHealthy {
		t.Fatalf("expected healthy server after start, got %+v", started)
	}
	if started.PreflightMs > 100 {
		t.Fatalf("expected start preflight <=100ms, got %d", started.PreflightMs)
	}
	healthEvents := manager.eventBus.List(events.Filter{Category: "mcp", ResourceKind: resourceKindServer})
	foundHealthChanged := false
	for _, event := range healthEvents {
		if event.Name == "mcp.server_health_changed" && event.Resource.ID == input.ServerID {
			foundHealthChanged = true
			break
		}
	}
	if !foundHealthChanged {
		t.Fatal("expected health-changed event after successful start")
	}

	tools, err := manager.ListTools(input.ServerID)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools) != 1 || tools[0].ToolName != "lookup" || tools[0].DiscoveryStatus != DiscoveryStatusDiscovered {
		t.Fatalf("unexpected tool discovery result: %+v", tools)
	}

	execution, ok := sandboxes.GetExecution(started.ExecutionID)
	if !ok || execution.ResourceKind != resourceKindServer || execution.ResourceID != input.ServerID {
		t.Fatalf("expected sandbox execution provenance for started server, got %+v", execution)
	}

	stopped, err := manager.Stop(ctx, input.ServerID)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if stopped.Action != LifecycleActionStop {
		t.Fatalf("expected stop lifecycle action, got %+v", stopped)
	}

	cancelledInput := testServerInput(t, true)
	cancelledInput.ServerID = "cancel-mcp"
	cancelledInput.DisplayName = "Cancel MCP"
	if _, _, err := manager.CreateServer(ctx, cancelledInput); err != nil {
		t.Fatalf("CreateServer(cancelled) returned error: %v", err)
	}
	if _, err := manager.Start(ctx, cancelledInput.ServerID, "manager-test"); err != nil {
		t.Fatalf("Start(cancelled) returned error: %v", err)
	}
	cancelled, err := manager.Cancel(ctx, cancelledInput.ServerID)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if cancelled.FailureClass != "cancelled" {
		t.Fatalf("expected cancelled failure class, got %+v", cancelled)
	}
}

func TestFilesystemCatalogEntryIsUnavailableWithoutLocalOverride(t *testing.T) {
	manager, _ := newTestManager(t)

	entry, ok := manager.GetCatalogEntry("filesystem")
	if !ok {
		t.Fatal("expected bundled filesystem catalog entry")
	}
	if entry.ImmediateUse {
		t.Fatalf("expected filesystem starter to stop advertising immediate use, got %+v", entry)
	}
	if entry.AvailabilityStatus != AvailabilityStatusUnavailable {
		t.Fatalf("expected unavailable filesystem starter by default, got %+v", entry)
	}
	if entry.AvailabilityReason == "" {
		t.Fatalf("expected local override availability reason, got %+v", entry)
	}
}

func TestCatalogLifecycleActionsPersistProvenanceAndRemoval(t *testing.T) {
	manager, sqliteStore, _ := newTestManagerWithStore(t)
	ctx := context.Background()

	workingDir := t.TempDir()
	install, err := manager.InstallCatalogEntry(ctx, "filesystem", CatalogInstallInput{
		ServerID:   "filesystem-phase22",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir: workingDir,
		SecretRefs: []string{"GO_WANT_MCP_HELPER", "MCP_HELPER_TOOLS"},
	}, InstallMethodAPI)
	if err != nil {
		t.Fatalf("InstallCatalogEntry returned error: %v", err)
	}
	if install.Server == nil || install.Server.CatalogManagement == nil {
		t.Fatalf("expected catalog management projection on install, got %+v", install)
	}
	if install.Server.CatalogManagement.InstalledRevision == "" || install.Server.CatalogManagement.DriftStatus != CatalogDriftStatusInSync {
		t.Fatalf("expected installed revision + in_sync drift, got %+v", install.Server.CatalogManagement)
	}

	refreshed, err := manager.RefreshCatalogServer(ctx, "filesystem-phase22")
	if err != nil {
		t.Fatalf("RefreshCatalogServer returned error: %v", err)
	}
	if refreshed.Status != CatalogActionStatusCompleted || refreshed.Server == nil {
		t.Fatalf("expected completed refresh with server projection, got %+v", refreshed)
	}
	if refreshed.PreflightMs > 100 {
		t.Fatalf("expected refresh preflight <=100ms, got %d", refreshed.PreflightMs)
	}
	if refreshed.Server.CatalogManagement == nil || refreshed.Server.CatalogManagement.LastMaintainedAt == nil {
		t.Fatalf("expected maintained timestamp after refresh, got %+v", refreshed.Server)
	}

	uninstalled, err := manager.UninstallCatalogServer(ctx, "filesystem-phase22")
	if err != nil {
		t.Fatalf("UninstallCatalogServer returned error: %v", err)
	}
	if uninstalled.Status != CatalogActionStatusCompleted || !uninstalled.Removed {
		t.Fatalf("expected completed uninstall removal, got %+v", uninstalled)
	}
	if _, ok := manager.GetServerResource("filesystem-phase22"); ok {
		t.Fatal("expected uninstalled server to be removed from active registry")
	}
	persistedServers, err := sqliteStore.ListMCPServers(ctx)
	if err != nil {
		t.Fatalf("ListMCPServers returned error: %v", err)
	}
	if len(persistedServers) != 0 {
		t.Fatalf("expected uninstall to remove persisted server record, got %+v", persistedServers)
	}
	events := manager.eventBus.List(events.Filter{Category: "mcp", ResourceKind: resourceKindServer})
	foundCompleted := false
	for _, event := range events {
		if event.Name == "mcp.catalog_lifecycle_completed" && event.Resource.ID == "filesystem-phase22" {
			foundCompleted = true
			break
		}
	}
	if !foundCompleted {
		t.Fatal("expected lifecycle completed audit event for uninstall")
	}
}

func TestCatalogLifecycleFailsClosedForModifiedBusyMissingEntryAndEnvironmentMismatch(t *testing.T) {
	manager, sqliteStore, _ := newTestManagerWithStore(t)
	ctx := context.Background()

	workingDir := t.TempDir()
	install, err := manager.InstallCatalogEntry(ctx, "filesystem", CatalogInstallInput{
		ServerID:   "filesystem-conflict",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir: workingDir,
		SecretRefs: []string{"GO_WANT_MCP_HELPER", "MCP_HELPER_TOOLS"},
	}, InstallMethodAPI)
	if err != nil {
		t.Fatalf("InstallCatalogEntry returned error: %v", err)
	}
	if install.Server == nil {
		t.Fatalf("expected installed server, got %+v", install)
	}

	updatedName := "Filesystem Modified"
	if _, err := manager.UpdateServer(ctx, "filesystem-conflict", UpdateServerInput{DisplayName: &updatedName}); err != nil {
		t.Fatalf("UpdateServer returned error: %v", err)
	}
	conflict, err := manager.RefreshCatalogServer(ctx, "filesystem-conflict")
	if err != nil {
		t.Fatalf("RefreshCatalogServer(conflict) returned error: %v", err)
	}
	if conflict.Status != CatalogActionStatusBlocked || conflict.FailureClass != "conflict" {
		t.Fatalf("expected conflict refresh block, got %+v", conflict)
	}

	busyInput := testServerInput(t, true)
	busyInput.ServerID = "filesystem-busy"
	busyInput.DisplayName = "Filesystem Busy"
	busyInput.OriginKind = OriginKindCatalog
	busyInput.CatalogEntryID = "filesystem"
	busyInput.InstallMethod = InstallMethodAPI
	busyInput.EnvironmentScope = string(manager.cfg.Environment)
	busyInput.CatalogManagement = catalogManagementForCreate(CatalogEntry{ID: "filesystem", SourceKind: "bundled"}, busyInput, nil, CatalogActionInstall, time.Now().UTC())
	if _, _, err := manager.CreateServer(ctx, busyInput); err != nil {
		t.Fatalf("CreateServer(busy) returned error: %v", err)
	}
	if _, err := manager.Start(ctx, "filesystem-busy", "manager-test"); err != nil {
		t.Fatalf("Start(busy) returned error: %v", err)
	}
	busy, err := manager.UninstallCatalogServer(ctx, "filesystem-busy")
	if err != nil {
		t.Fatalf("UninstallCatalogServer(busy) returned error: %v", err)
	}
	if busy.Status != CatalogActionStatusBlocked || busy.FailureClass != "busy" {
		t.Fatalf("expected busy uninstall block, got %+v", busy)
	}

	missingInput := testServerInput(t, false)
	missingInput.ServerID = "filesystem-missing"
	missingInput.DisplayName = "Filesystem Missing"
	missingInput.OriginKind = OriginKindCatalog
	missingInput.CatalogEntryID = "missing-entry"
	missingInput.InstallMethod = InstallMethodAPI
	missingInput.EnvironmentScope = string(manager.cfg.Environment)
	missingInput.CatalogManagement = &CatalogManagement{
		SourceKind: "bundled",
		InstallInputSnapshot: CatalogInstallSnapshot{
			ServerID:         "filesystem-missing",
			DisplayName:      "Filesystem Missing",
			Enabled:          boolPtr(false),
			SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
			Command:          os.Args[0],
			Args:             []string{"-test.run=TestMCPHelperProcess", "--"},
			WorkingDir:       t.TempDir(),
			InstallMethod:    InstallMethodAPI,
		},
	}
	if _, _, err := manager.CreateServer(ctx, missingInput); err != nil {
		t.Fatalf("CreateServer(missing) returned error: %v", err)
	}
	missing, err := manager.RefreshCatalogServer(ctx, "filesystem-missing")
	if err != nil {
		t.Fatalf("RefreshCatalogServer(missing) returned error: %v", err)
	}
	if missing.Status != CatalogActionStatusBlocked || missing.FailureClass != "missing_entry" {
		t.Fatalf("expected missing-entry refresh block, got %+v", missing)
	}

	envInput := testServerInput(t, false)
	envInput.ServerID = "filesystem-prod"
	envInput.DisplayName = "Filesystem Prod"
	envInput.OriginKind = OriginKindCatalog
	envInput.CatalogEntryID = "filesystem"
	envInput.InstallMethod = InstallMethodAPI
	envInput.EnvironmentScope = "prod"
	envInput.CatalogManagement = &CatalogManagement{
		SourceKind: "bundled",
		InstallInputSnapshot: CatalogInstallSnapshot{
			ServerID:         "filesystem-prod",
			DisplayName:      "Filesystem Prod",
			Enabled:          boolPtr(false),
			SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
			Command:          os.Args[0],
			Args:             []string{"-test.run=TestMCPHelperProcess", "--"},
			WorkingDir:       t.TempDir(),
			InstallMethod:    InstallMethodAPI,
		},
	}
	if _, _, err := manager.CreateServer(ctx, envInput); err != nil {
		t.Fatalf("CreateServer(env mismatch) returned error: %v", err)
	}
	envMismatch, err := manager.RefreshCatalogServer(ctx, "filesystem-prod")
	if err != nil {
		t.Fatalf("RefreshCatalogServer(env mismatch) returned error: %v", err)
	}
	if envMismatch.Status != CatalogActionStatusBlocked || envMismatch.FailureClass != "environment_mismatch" {
		t.Fatalf("expected environment mismatch block, got %+v", envMismatch)
	}

	run := runtime.Run{
		RunID:      "run_busy",
		Entrypoint: "chat",
		Status:     runtime.RunStatusRunning,
		Goal:       "busy tool call",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	step := runtime.Step{
		StepID:    "step_busy",
		RunID:     run.RunID,
		Title:     "busy tool step",
		Kind:      "tool",
		Status:    runtime.StepStatusExecutingTool,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertToolCall(ctx, runtime.ToolCall{
		ToolCallID:     "tool_call_busy_1",
		RunID:          "run_busy",
		StepID:         "step_busy",
		InvocationKind: runtime.ToolCallInvocationKindMCPTool,
		MCPServerID:    "filesystem-conflict",
		MCPToolName:    "lookup",
		ToolName:       "lookup",
		Status:         runtime.ToolCallStatusRunning,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}
	busyToolCall, err := manager.UninstallCatalogServer(ctx, "filesystem-conflict")
	if err != nil {
		t.Fatalf("UninstallCatalogServer(tool busy) returned error: %v", err)
	}
	if busyToolCall.Status != CatalogActionStatusBlocked || busyToolCall.FailureClass != "busy" {
		t.Fatalf("expected busy tool-call uninstall block, got %+v", busyToolCall)
	}
}

func TestCatalogRevalidationClassifiesPrerequisiteLossAndDrift(t *testing.T) {
	manager, _, _ := newTestManagerWithStore(t)
	ctx := context.Background()

	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"POSTGRES_DSN": "postgres://user:pass@localhost/db",
	})
	install, err := manager.InstallCatalogEntry(ctx, "postgres", CatalogInstallInput{
		ServerID:   "postgres-phase22",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir: t.TempDir(),
		SecretRefs: []string{"POSTGRES_DSN"},
	}, InstallMethodAPI)
	if err != nil {
		t.Fatalf("InstallCatalogEntry(postgres) returned error: %v", err)
	}
	if install.Server == nil {
		t.Fatalf("expected installed postgres server, got %+v", install)
	}

	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{})
	revalidated, err := manager.RevalidateCatalogServer(ctx, "postgres-phase22")
	if err != nil {
		t.Fatalf("RevalidateCatalogServer returned error: %v", err)
	}
	if revalidated.Status != AvailabilityStatusBlocked || revalidated.Classification != RevalidationClassificationPrerequisiteLost {
		t.Fatalf("expected prerequisite_lost blocked revalidation, got %+v", revalidated)
	}
	if len(revalidated.Issues) == 0 || revalidated.Issues[0].Kind != "secret" {
		t.Fatalf("expected secret issue, got %+v", revalidated.Issues)
	}
	if revalidated.PreflightMs > 100 {
		t.Fatalf("expected revalidation preflight <=100ms, got %d", revalidated.PreflightMs)
	}

	displayName := "Postgres Modified"
	if _, err := manager.UpdateServer(ctx, "postgres-phase22", UpdateServerInput{DisplayName: &displayName}); err != nil {
		t.Fatalf("UpdateServer returned error: %v", err)
	}
	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"POSTGRES_DSN": "postgres://user:pass@localhost/db",
	})
	localDrift, err := manager.RevalidateCatalogServer(ctx, "postgres-phase22")
	if err != nil {
		t.Fatalf("RevalidateCatalogServer(local drift) returned error: %v", err)
	}
	if localDrift.Classification != RevalidationClassificationLocallyModified {
		t.Fatalf("expected locally_modified classification, got %+v", localDrift)
	}
}

func TestCatalogRevalidationSucceedsForHealthyRunningServer(t *testing.T) {
	manager, _, _ := newTestManagerWithStore(t)
	ctx := context.Background()

	install, err := manager.InstallCatalogEntry(ctx, "filesystem", CatalogInstallInput{
		ServerID:   "filesystem-running",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir: t.TempDir(),
		SecretRefs: []string{"GO_WANT_MCP_HELPER", "MCP_HELPER_TOOLS"},
	}, InstallMethodAPI)
	if err != nil {
		t.Fatalf("InstallCatalogEntry(filesystem) returned error: %v", err)
	}
	if install.Server == nil {
		t.Fatalf("expected installed filesystem server, got %+v", install)
	}
	if _, err := manager.Start(ctx, "filesystem-running", "manager-test"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForServerStatus(t, manager, "filesystem-running", LifecycleStatusHealthy)

	revalidated, err := manager.RevalidateCatalogServer(ctx, "filesystem-running")
	if err != nil {
		t.Fatalf("RevalidateCatalogServer returned error: %v", err)
	}
	if revalidated.Status != AvailabilityStatusReady || revalidated.Classification != RevalidationClassificationHealthy {
		t.Fatalf("expected healthy ready revalidation on running server, got %+v", revalidated)
	}
	if len(revalidated.Issues) != 0 {
		t.Fatalf("expected no issues for healthy running server, got %+v", revalidated.Issues)
	}
	if revalidated.Server == nil || revalidated.Server.State.Status != LifecycleStatusHealthy {
		t.Fatalf("expected healthy server projection after revalidation, got %+v", revalidated.Server)
	}
}

func TestManagerRestoreAutoStartsEnabledServers(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	writeMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_MCP_HELPER": "1",
		"MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	manager := NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, NewTransportMux(NewStdioTransport(), nil))
	input := testServerInput(t, true)
	input.AutoRestart = false
	if _, _, err := manager.CreateServer(context.Background(), input); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	manager2 := NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, NewTransportMux(NewStdioTransport(), nil))
	if err := manager2.Restore(context.Background()); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	resource, ok := manager2.GetServerResource(input.ServerID)
	if !ok {
		t.Fatal("expected restored server resource")
	}
	if resource.State.Status != LifecycleStatusHealthy {
		t.Fatalf("expected restored server to auto-start healthy, got %+v", resource)
	}
}

func TestManagerStartTimesOutOnUnresponsiveStdioServer(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	previousTimeout := mcpSessionStartTimeout
	mcpSessionStartTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		mcpSessionStartTimeout = previousTimeout
	})

	if _, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "hung-mcp",
		DisplayName:      "Hung MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:hung-mcp:lifecycle.start",
		TransportKind:    TransportKindStdio,
		Command:          "/bin/sh",
		Args:             []string{"-c", "sleep 60"},
		WorkingDir:       t.TempDir(),
		AutoRestart:      false,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	started, err := manager.Start(ctx, "hung-mcp", "manager-test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.FailureClass != "transport_runtime_failure" {
		t.Fatalf("expected transport runtime failure for timed-out MCP start, got %+v", started)
	}
	if started.Server.State.Status != LifecycleStatusFailed {
		t.Fatalf("expected failed lifecycle status after timeout, got %+v", started.Server.State)
	}
	if !strings.Contains(started.Server.State.HealthReason, "context deadline exceeded") {
		t.Fatalf("expected timeout health reason, got %+v", started.Server.State)
	}
}

func TestManagerRejectsInactiveDeclarationWhenEnablingServer(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	input := testServerInput(t, false)
	input.Declaration = &Declaration{
		ExecutionMode:               sandbox.ExecutionModeSubprocess,
		AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
		NetworkMode:                 sandbox.NetworkModeDeny,
		ApprovalMode:                sandbox.ApprovalModeAllow,
		RequiredEnforcementStrength: "declared_only",
		Active:                      false,
	}
	if _, _, err := manager.CreateServer(ctx, input); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	enabled := true
	if _, err := manager.UpdateServer(ctx, input.ServerID, UpdateServerInput{Enabled: &enabled}); err == nil {
		t.Fatal("expected inactive declaration to block enabling the server")
	}
}

func TestManagerMarksUnsupportedDeclarationAtStart(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	input := testServerInput(t, true)
	input.Declaration = &Declaration{
		ExecutionMode:               sandbox.ExecutionModeSubprocess,
		AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
		NetworkMode:                 sandbox.NetworkModeDeny,
		ApprovalMode:                sandbox.ApprovalModeAllow,
		RequiredEnforcementStrength: "container",
		Active:                      true,
	}
	if _, _, err := manager.CreateServer(ctx, input); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	started, err := manager.Start(ctx, input.ServerID, "manager-test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !started.Blocked || started.FailureClass != "backend_unavailable" {
		t.Fatalf("expected unsupported declaration start to block with backend_unavailable, got %+v", started)
	}
	if started.Server.State.Status != LifecycleStatusUnsupported {
		t.Fatalf("expected unsupported lifecycle status, got %+v", started.Server.State)
	}
}

func TestManagerPersistsStaleToolsAcrossRediscovery(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	input := testServerInput(t, true)
	if _, _, err := manager.CreateServer(ctx, input); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if _, err := manager.Start(ctx, input.ServerID, "manager-test"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if _, err := manager.Stop(ctx, input.ServerID); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	waitForServerStatus(t, manager, input.ServerID, LifecycleStatusStopped)
	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"GO_WANT_MCP_HELPER": "1",
		"MCP_HELPER_TOOLS":   `[{"name":"search","title":"Search","description":"Search tool","inputSchema":{"type":"object"}}]`,
	})
	if _, err := manager.Start(ctx, input.ServerID, "manager-test"); err != nil {
		t.Fatalf("Start(rediscovery) returned error: %v", err)
	}

	records, err := manager.store.ListMCPTools(ctx, input.ServerID)
	if err != nil {
		t.Fatalf("ListMCPTools returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 persisted tool records after rediscovery, got %+v", records)
	}
	statusByTool := map[string]string{}
	for _, record := range records {
		statusByTool[record.ToolName] = record.DiscoveryStatus
	}
	if statusByTool["lookup"] != string(DiscoveryStatusStale) || statusByTool["search"] != string(DiscoveryStatusDiscovered) {
		t.Fatalf("expected stale+discovered persisted tools, got %+v", statusByTool)
	}
}

func TestManagerSupportsStreamableHTTPLifecycleAndInvocation(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		acceptHeader := r.Header.Get("Accept")
		if !strings.Contains(acceptHeader, "application/json") || !strings.Contains(acceptHeader, "text/event-stream") {
			t.Fatalf("expected streamable-http accept header to include application/json and text/event-stream, got %q", acceptHeader)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode remote mcp request: %v", err)
		}
		method, _ := req["method"].(string)
		id := req["id"]
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}}})
		case "notifications/initialized":
			if _, hasID := req["id"]; hasID {
				t.Fatalf("expected notifications/initialized to be sent without an id, got %+v", req)
			}
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"tools": []map[string]any{{"name": "lookup", "title": "Lookup", "description": "Lookup tool", "inputSchema": map[string]any{"type": "object"}}}}})
		case "tools/call":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "remote ok"}}}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}))
	defer remote.Close()

	server, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "remote-mcp",
		DisplayName:      "Remote MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:remote-mcp:lifecycle.start",
		TransportKind:    TransportKindStreamableHTTP,
		Endpoint:         remote.URL,
		AutoRestart:      true,
	})
	if err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if server.TransportKind != TransportKindStreamableHTTP {
		t.Fatalf("expected streamable-http server, got %+v", server)
	}

	started, err := manager.Start(ctx, "remote-mcp", "manager-test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.Server.State.Status != LifecycleStatusHealthy || started.ExecutionID != "" {
		t.Fatalf("expected healthy remote server without sandbox execution, got %+v", started)
	}

	if _, err := manager.UpdateToolExposure(ctx, "remote-mcp", "lookup", UpdateExposureInput{RuntimeSurface: "chat", ExposureMode: ExposureModeAllow, Active: true}); err != nil {
		t.Fatalf("UpdateToolExposure returned error: %v", err)
	}
	authz, err := manager.AuthorizeTool(ctx, "remote-mcp", "lookup", AuthorizeToolInput{RuntimeSurface: "chat", RequestedBy: "manager-test"})
	if err != nil {
		t.Fatalf("AuthorizeTool returned error: %v", err)
	}
	if authz.Status != ToolAuthorizationStatusAllowed {
		t.Fatalf("expected allowed authorization, got %+v", authz)
	}
	result, err := manager.CallTool(ctx, "remote-mcp", "lookup", map[string]any{"query": "hello"}, authz)
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.FailureClass != "" || result.SessionID == "" {
		t.Fatalf("unexpected remote call result: %+v", result)
	}
}

func TestManagerSupportsWebsocketLifecycleAndInvocation(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"MCP_WS_TOKEN": "secret-token",
	})
	remote := newTestWebsocketMCPServer(t, "secret-token")
	defer remote.Close()

	server, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "websocket-remote-mcp",
		DisplayName:      "Websocket Remote MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:websocket-remote-mcp:lifecycle.start",
		TransportKind:    TransportKindWebsocket,
		Endpoint:         remote.wsURL(),
		WebsocketConfig: &WebsocketConfig{
			Auth: &WebsocketAuthConfig{
				Mode:      WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	})
	if err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if server.TransportKind != TransportKindWebsocket {
		t.Fatalf("expected websocket server, got %+v", server)
	}

	started, err := manager.Start(ctx, "websocket-remote-mcp", "manager-test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.Server.State.Status != LifecycleStatusHealthy || started.ExecutionID != "" {
		t.Fatalf("expected healthy websocket server without sandbox execution, got %+v", started)
	}
	if started.Server.State.LastSessionID == "" {
		t.Fatalf("expected websocket start to persist last session id, got %+v", started.Server.State)
	}

	if _, err := manager.UpdateToolExposure(ctx, "websocket-remote-mcp", "lookup", UpdateExposureInput{RuntimeSurface: "chat", ExposureMode: ExposureModeAllow, Active: true}); err != nil {
		t.Fatalf("UpdateToolExposure returned error: %v", err)
	}
	authz, err := manager.AuthorizeTool(ctx, "websocket-remote-mcp", "lookup", AuthorizeToolInput{RuntimeSurface: "chat", RequestedBy: "manager-test"})
	if err != nil {
		t.Fatalf("AuthorizeTool returned error: %v", err)
	}
	if authz.Status != ToolAuthorizationStatusAllowed {
		t.Fatalf("expected allowed authorization, got %+v", authz)
	}
	result, err := manager.CallTool(ctx, "websocket-remote-mcp", "lookup", map[string]any{"query": "hello"}, authz)
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.FailureClass != "" || result.SessionID == "" {
		t.Fatalf("unexpected websocket call result: %+v", result)
	}
}

func TestManagerReconnectsWebsocketAfterDisconnect(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	restoreTimeout := SetSessionStartTimeoutForTest(250 * time.Millisecond)
	defer restoreTimeout()
	restoreBackoff := SetReconnectBackoffDelayForTest(25 * time.Millisecond)
	defer restoreBackoff()
	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"MCP_WS_TOKEN": "secret-token",
	})
	remote := newTestWebsocketMCPServer(t, "secret-token")
	defer remote.Close()

	if _, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "websocket-reconnect",
		DisplayName:      "Websocket Reconnect",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:websocket-reconnect:lifecycle.start",
		TransportKind:    TransportKindWebsocket,
		Endpoint:         remote.wsURL(),
		WebsocketConfig: &WebsocketConfig{
			Auth: &WebsocketAuthConfig{
				Mode:      WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if _, err := manager.Start(ctx, "websocket-reconnect", "manager-test"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForServerStatus(t, manager, "websocket-reconnect", LifecycleStatusHealthy)

	remote.closeActive()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resource, ok := manager.GetServerResource("websocket-reconnect")
		if ok && resource.State.LastRecoveryClass == "reconnect_succeeded" && resource.State.Status == LifecycleStatusHealthy {
			if resource.State.ReconnectAttemptCount != 0 {
				t.Fatalf("expected reconnect attempt count to reset after successful recovery, got %+v", resource.State)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	resource, _ := manager.GetServerResource("websocket-reconnect")
	t.Fatalf("expected websocket reconnect success, got %+v", resource.State)
}

func TestManagerReconnectBudgetResetsPerDisconnectEpisode(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	restoreTimeout := SetSessionStartTimeoutForTest(250 * time.Millisecond)
	defer restoreTimeout()
	restoreBackoff := SetReconnectBackoffDelayForTest(25 * time.Millisecond)
	defer restoreBackoff()
	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{
		"MCP_WS_TOKEN": "secret-token",
	})
	remote := newTestWebsocketMCPServer(t, "secret-token")
	defer remote.Close()

	if _, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "websocket-reconnect-reset",
		DisplayName:      "Websocket Reconnect Reset",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:websocket-reconnect-reset:lifecycle.start",
		TransportKind:    TransportKindWebsocket,
		Endpoint:         remote.wsURL(),
		WebsocketConfig: &WebsocketConfig{
			Auth: &WebsocketAuthConfig{
				Mode:      WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	if _, err := manager.Start(ctx, "websocket-reconnect-reset", "manager-test"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForServerStatus(t, manager, "websocket-reconnect-reset", LifecycleStatusHealthy)

	for episode := 0; episode < 2; episode++ {
		remote.closeActive()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			resource, ok := manager.GetServerResource("websocket-reconnect-reset")
			if ok && resource.State.LastRecoveryClass == "reconnect_succeeded" && resource.State.Status == LifecycleStatusHealthy {
				if resource.State.ReconnectAttemptCount != 0 {
					t.Fatalf("expected reconnect attempt count to reset after episode %d, got %+v", episode+1, resource.State)
				}
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
}

func TestManagerListsTransportCapabilitiesAndWebsocketServerTruth(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	capabilities := manager.ListTransportCapabilities()
	if len(capabilities) < 3 {
		t.Fatalf("expected stdio, streamable-http, and websocket capability records, got %+v", capabilities)
	}
	byKind := map[TransportKind]TransportCapability{}
	for _, capability := range capabilities {
		byKind[capability.TransportKind] = capability
		if capability.EnvironmentScope != string(manager.cfg.Environment) {
			t.Fatalf("expected environment-scoped capability, got %+v", capability)
		}
	}
	if byKind[TransportKindStdio].AvailabilityStatus != AvailabilityStatusReady {
		t.Fatalf("expected stdio capability to be ready, got %+v", byKind[TransportKindStdio])
	}
	if byKind[TransportKindStreamableHTTP].AvailabilityStatus != AvailabilityStatusReady {
		t.Fatalf("expected streamable-http capability to be ready, got %+v", byKind[TransportKindStreamableHTTP])
	}
	websocketCapability, ok := byKind[TransportKindWebsocket]
	if !ok {
		t.Fatalf("expected websocket capability record, got %+v", capabilities)
	}
	if websocketCapability.AvailabilityStatus != AvailabilityStatusReady {
		t.Fatalf("expected websocket capability to be ready on the current host, got %+v", websocketCapability)
	}
	if websocketCapability.HealthStatus != TransportHealthStatusHealthy {
		t.Fatalf("expected healthy websocket capability, got %+v", websocketCapability)
	}
	if !websocketCapability.DaemonManagedReconnect {
		t.Fatalf("expected websocket capability to advertise daemon-managed reconnect, got %+v", websocketCapability)
	}
	if len(websocketCapability.SupportedAuthKinds) == 0 {
		t.Fatalf("expected websocket capability to advertise supported auth kinds, got %+v", websocketCapability)
	}

	writeMCPSecretsFileForTest(t, manager.cfg.DataDir, map[string]string{})
	resource, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "websocket-mcp",
		DisplayName:      "Websocket MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:websocket-mcp:lifecycle.start",
		TransportKind:    TransportKindWebsocket,
		Endpoint:         "ws://127.0.0.1:19234/mcp",
		WebsocketConfig: &WebsocketConfig{
			Auth: &WebsocketAuthConfig{
				Mode:      WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	})
	if err != nil {
		t.Fatalf("CreateServer(websocket) returned error: %v", err)
	}
	if resource.TransportKind != TransportKindWebsocket {
		t.Fatalf("expected websocket server resource, got %+v", resource)
	}
	if resource.WebsocketAuthSummary == nil {
		t.Fatalf("expected websocket auth summary, got %+v", resource)
	}
	if resource.WebsocketAuthSummary.SecretRef != "MCP_WS_TOKEN" {
		t.Fatalf("expected redacted auth summary to retain secret ref only, got %+v", resource.WebsocketAuthSummary)
	}
	if resource.WebsocketAuthSummary.Resolved {
		t.Fatalf("expected unresolved secret to stay unresolved, got %+v", resource.WebsocketAuthSummary)
	}
	if resource.AvailabilityStatus != AvailabilityStatusBlocked {
		t.Fatalf("expected websocket server with missing secret to be blocked, got %+v", resource)
	}
	if !strings.Contains(resource.TransportConfigSummary, "ws://127.0.0.1:19234/mcp") {
		t.Fatalf("expected websocket endpoint in transport summary, got %+v", resource)
	}
}

func TestManagerRejectsWebsocketEndpointWithInlineSecretMaterial(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	_, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "websocket-inline-secret",
		DisplayName:      "Websocket Inline Secret",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:websocket-inline-secret:lifecycle.start",
		TransportKind:    TransportKindWebsocket,
		Endpoint:         "wss://user:secret-token@example.com/mcp?token=secret-token",
		WebsocketConfig: &WebsocketConfig{
			Auth: &WebsocketAuthConfig{
				Mode:      WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	})
	if err == nil {
		t.Fatal("expected websocket endpoint with inline secret material to be rejected")
	}
	if !strings.Contains(err.Error(), "websocket endpoint") {
		t.Fatalf("expected websocket endpoint validation error, got %v", err)
	}
}

func TestManagerSanitizesLegacyWebsocketEndpointProjection(t *testing.T) {
	manager, _ := newTestManager(t)

	server := Server{
		ServerID:          "legacy-websocket-mcp",
		DisplayName:       "Legacy Websocket MCP",
		Enabled:           true,
		SandboxProfileID:  sandbox.ProfileIDSubprocessDefault,
		DeclarationID:     "mcp_server:legacy-websocket-mcp:lifecycle.start",
		Declaration:       Declaration{Active: true},
		TransportKind:     TransportKindWebsocket,
		Endpoint:          "wss://user:secret-token@example.com/mcp?token=secret-token",
		WebsocketConfig:   &WebsocketConfig{Auth: &WebsocketAuthConfig{Mode: WebsocketAuthModeBearerHeader, SecretRef: "MCP_WS_TOKEN"}},
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		EnvironmentScope:  string(manager.cfg.Environment),
		SecretRefs:        []string{"MCP_WS_TOKEN"},
		AutoRestart:       true,
	}

	manager.mu.Lock()
	manager.servers[server.ServerID] = server
	manager.serverIDs = append(manager.serverIDs, server.ServerID)
	manager.states[server.ServerID] = defaultStateForServer(server)
	manager.mu.Unlock()

	resource, ok := manager.GetServerResource(server.ServerID)
	if !ok {
		t.Fatal("expected legacy websocket resource")
	}
	if strings.Contains(resource.Endpoint, "secret-token") || strings.Contains(resource.Endpoint, "user:") {
		t.Fatalf("expected websocket endpoint projection to redact inline secret material, got %+v", resource)
	}
	if strings.Contains(resource.TransportConfigSummary, "secret-token") || strings.Contains(resource.TransportConfigSummary, "user:") {
		t.Fatalf("expected websocket transport summary to redact inline secret material, got %+v", resource)
	}
}

func TestInstallCatalogEntryBlocksManualServerIDCollision(t *testing.T) {
	manager, _ := newTestManager(t)
	ctx := context.Background()

	if _, _, err := manager.CreateServer(ctx, CreateServerInput{
		ServerID:         "filesystem",
		DisplayName:      "Manual Filesystem",
		Enabled:          false,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:filesystem:lifecycle.start",
		TransportKind:    TransportKindStdio,
		Command:          os.Args[0],
		Args:             []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir:       t.TempDir(),
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	result, err := manager.InstallCatalogEntry(ctx, "filesystem", CatalogInstallInput{
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir: t.TempDir(),
	}, InstallMethodAPI)
	if err != nil {
		t.Fatalf("InstallCatalogEntry returned error: %v", err)
	}
	if result.Status != "blocked" || result.AvailabilityStatus != AvailabilityStatusBlocked {
		t.Fatalf("expected blocked install on manual id collision, got %+v", result)
	}
	if !strings.Contains(result.AvailabilityReason, "manual MCP server") {
		t.Fatalf("expected manual collision reason, got %+v", result)
	}
}

func TestResolveMCPSecretsIgnoresProcessEnvironment(t *testing.T) {
	t.Setenv("MCP_ENV_ONLY", "prod-secret")

	resolved, err := ResolveMCPSecrets(t.TempDir(), []string{"MCP_ENV_ONLY"})
	if err != nil {
		t.Fatalf("ResolveMCPSecrets returned error: %v", err)
	}
	if _, ok := resolved["MCP_ENV_ONLY"]; ok {
		t.Fatalf("expected process environment to be ignored, got %+v", resolved)
	}
}

func TestRedactStringRedactsCommonDerivedSecretForms(t *testing.T) {
	secret := "top/secret+value"
	cases := []string{
		"literal=" + secret,
		"url=" + url.QueryEscape(secret),
		"b64=" + base64.StdEncoding.EncodeToString([]byte(secret)),
		"rawb64=" + base64.RawStdEncoding.EncodeToString([]byte(secret)),
		"urlb64=" + base64.URLEncoding.EncodeToString([]byte(secret)),
		"rawurlb64=" + base64.RawURLEncoding.EncodeToString([]byte(secret)),
		"hex=" + hex.EncodeToString([]byte(secret)),
		"hexupper=" + strings.ToUpper(hex.EncodeToString([]byte(secret))),
	}
	for _, input := range cases {
		redacted := redactString(input, map[string]string{"SECRET": secret})
		if strings.Contains(redacted, secret) || strings.Contains(redacted, url.QueryEscape(secret)) {
			t.Fatalf("expected secret-derived value to be redacted, input=%q output=%q", input, redacted)
		}
		if !strings.Contains(redacted, "[REDACTED]") {
			t.Fatalf("expected redaction marker, input=%q output=%q", input, redacted)
		}
	}
}

func newTestManager(t *testing.T) (*Manager, *sandbox.Manager) {
	manager, _, sandboxes := newTestManagerWithStore(t)
	return manager, sandboxes
}

func newTestManagerWithStore(t *testing.T) (*Manager, *store.SQLiteStore, *sandbox.Manager) {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "dope")
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	writeMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_MCP_HELPER": "1",
		"MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqliteStore.Close()
	})

	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)

	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	t.Cleanup(func() {
		_ = sandboxes.Close(context.Background())
	})

	return NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, NewTransportMux(NewStdioTransport(), nil)), sqliteStore, sandboxes
}

func waitForServerStatus(t *testing.T, manager *Manager, serverID string, status LifecycleStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resource, ok := manager.GetServerResource(serverID)
		if ok && resource.State.Status == status {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	resource, _ := manager.GetServerResource(serverID)
	t.Fatalf("expected server %s to reach status %s, got %+v", serverID, status, resource.State)
}

func testServerInput(t *testing.T, enabled bool) CreateServerInput {
	t.Helper()
	t.Setenv("GO_WANT_MCP_HELPER", "1")
	t.Setenv("MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)
	return CreateServerInput{
		ServerID:         "test-mcp",
		DisplayName:      "Test MCP",
		Enabled:          enabled,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:test-mcp:lifecycle.start",
		TransportKind:    TransportKindStdio,
		Command:          os.Args[0],
		Args:             []string{"-test.run=TestMCPHelperProcess", "--"},
		WorkingDir:       t.TempDir(),
		SecretRefs:       []string{"GO_WANT_MCP_HELPER", "MCP_HELPER_TOOLS"},
		AutoRestart:      enabled,
	}
}

func writeMCPSecretsFileForTest(t *testing.T, dataDir string, values map[string]string) {
	t.Helper()
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal mcp secrets: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, mcpSecretsFileName), payload, 0o600); err != nil {
		t.Fatalf("write mcp secrets: %v", err)
	}
}

func writeHelperResponse(id string, result any) {
	writeHelperFrame(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeHelperError(id string, code int, message string) {
	writeHelperFrame(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writeHelperFrame(value any) {
	payload, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

type testWebsocketMCPServer struct {
	server *httptest.Server
	mu     sync.Mutex
	conn   *websocket.Conn
}

func newTestWebsocketMCPServer(t *testing.T, token string) *testWebsocketMCPServer {
	t.Helper()

	ws := &testWebsocketMCPServer{}
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.Header.Get("Authorization")); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade websocket: %v", err)
		}
		defer conn.Close()
		ws.mu.Lock()
		ws.conn = conn
		ws.mu.Unlock()

		for {
			var req rpcRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			switch req.Method {
			case "initialize":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "ws-test-mcp", "version": "1.0.0"}}})
			case "notifications/initialized":
			case "tools/list":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": []map[string]any{{"name": "lookup", "title": "Lookup", "description": "Lookup tool", "inputSchema": map[string]any{"type": "object"}}}}})
			case "tools/call":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "websocket ok"}}}})
			default:
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
			}
		}
	}))
	ws.server = server
	return ws
}

func (s *testWebsocketMCPServer) Close() {
	s.server.Close()
}

func (s *testWebsocketMCPServer) wsURL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/mcp"
}

func (s *testWebsocketMCPServer) closeActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "test disconnect"), time.Now().Add(time.Second))
		_ = s.conn.Close()
		s.conn = nil
	}
}
