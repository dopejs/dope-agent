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
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
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

	return NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, NewTransportMux(NewStdioTransport(), nil)), sandboxes
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
