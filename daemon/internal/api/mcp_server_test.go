package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
	"github.com/gorilla/websocket"
)

func TestAPIMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_API_MCP_HELPER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	toolsPayload := os.Getenv("API_MCP_HELPER_TOOLS")
	if strings.TrimSpace(toolsPayload) == "" {
		toolsPayload = `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`
	}
	for {
		payload, err := readAPIHelperFrame(reader)
		if err != nil {
			return
		}
		var req struct {
			ID     string `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}
		switch req.Method {
		case "initialize":
			writeAPIHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "api-test-mcp", "version": "1.0.0"}}})
		case "notifications/initialized":
		case "tools/list":
			var tools []map[string]any
			_ = json.Unmarshal([]byte(toolsPayload), &tools)
			writeAPIHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": tools}})
		case "tools/call":
			writeAPIHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "lookup ok"}}}})
		default:
			writeAPIHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}
}

func TestMCPServerRoutes(t *testing.T) {
	t.Setenv("GO_WANT_API_MCP_HELPER", "1")
	t.Setenv("API_MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_API_MCP_HELPER": "1",
		"API_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Policy:    policyEngine,
		Sandboxes: sandboxes,
		MCP:       mcpManager,
		Store:     sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "mcp-web")

	createReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers", strings.NewReader(`{"serverId":"api-mcp","displayName":"API MCP","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:api-mcp:lifecycle.start","transportKind":"stdio","command":"`+os.Args[0]+`","args":["-test.run=TestAPIMCPHelperProcess","--"],"workingDir":"`+t.TempDir()+`","secretRefs":["GO_WANT_API_MCP_HELPER","API_MCP_HELPER_TOOLS"],"autoRestart":true}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for mcp create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers", nil)
	listReq.Header.Set("Authorization", authHeader)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for mcp list, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/api-mcp/start", nil)
	startReq.Header.Set("Authorization", authHeader)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for mcp start, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var started mcp.LifecycleResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatalf("failed to decode lifecycle response: %v", err)
	}
	if started.Server.State.Status != mcp.LifecycleStatusHealthy || started.PreflightMs > 100 {
		t.Fatalf("unexpected lifecycle response: %+v", started)
	}

	toolsReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers/api-mcp/tools", nil)
	toolsReq.Header.Set("Authorization", authHeader)
	toolsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(toolsRec, toolsReq)
	if toolsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for mcp tools, got %d body=%s", toolsRec.Code, toolsRec.Body.String())
	}
	var tools ListResponse[mcp.ToolResource]
	if err := json.Unmarshal(toolsRec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("failed to decode tools response: %v", err)
	}
	if len(tools.Items) != 1 || tools.Items[0].ToolName != "lookup" {
		t.Fatalf("unexpected tools response: %+v", tools)
	}

	exposureReq := httptest.NewRequest(http.MethodPatch, "/v1/mcp/servers/api-mcp/tools/lookup", strings.NewReader(`{"runtimeSurface":"chat","exposureMode":"approval_required","active":true,"reason":"needs approval"}`))
	exposureReq.Header.Set("Authorization", authHeader)
	exposureRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(exposureRec, exposureReq)
	if exposureRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exposure update, got %d body=%s", exposureRec.Code, exposureRec.Body.String())
	}
	var tool mcp.ToolResource
	if err := json.Unmarshal(exposureRec.Body.Bytes(), &tool); err != nil {
		t.Fatalf("failed to decode tool response: %v", err)
	}
	if !tool.ApprovalRequired {
		t.Fatalf("expected approval-required tool response, got %+v", tool)
	}

	authorizeReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/api-mcp/tools/lookup/authorize", strings.NewReader(`{"runtimeSurface":"chat"}`))
	authorizeReq.Header.Set("Authorization", authHeader)
	authorizeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(authorizeRec, authorizeReq)
	if authorizeRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for tool authorization approval gate, got %d body=%s", authorizeRec.Code, authorizeRec.Body.String())
	}
	var authorizeResponse mcp.ToolAuthorizationResponse
	if err := json.Unmarshal(authorizeRec.Body.Bytes(), &authorizeResponse); err != nil {
		t.Fatalf("failed to decode authorization response: %v", err)
	}
	if authorizeResponse.Status != mcp.ToolAuthorizationStatusPending || authorizeResponse.Approval == nil {
		t.Fatalf("expected pending authorization approval response, got %+v", authorizeResponse)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+authorizeResponse.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"ok"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval resolve, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	authorizeApprovedReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/api-mcp/tools/lookup/authorize", strings.NewReader(`{"runtimeSurface":"chat","approvalId":"`+authorizeResponse.Approval.ApprovalID+`"}`))
	authorizeApprovedReq.Header.Set("Authorization", authHeader)
	authorizeApprovedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(authorizeApprovedRec, authorizeApprovedReq)
	if authorizeApprovedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approved tool authorization, got %d body=%s", authorizeApprovedRec.Code, authorizeApprovedRec.Body.String())
	}

	configReq := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	configReq.Header.Set("Authorization", authHeader)
	configRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK || !strings.Contains(configRec.Body.String(), `"mcp"`) || !strings.Contains(configRec.Body.String(), `"tools"`) {
		t.Fatalf("expected config response with mcp projection, got %d body=%s", configRec.Code, configRec.Body.String())
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/events?category=mcp", nil)
	eventsReq.Header.Set("Authorization", authHeader)
	eventsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK || !strings.Contains(eventsRec.Body.String(), `"mcp.server_started"`) {
		t.Fatalf("expected mcp event list response, got %d body=%s", eventsRec.Code, eventsRec.Body.String())
	}
}

func TestMCPServerRoutesScopeTenantResources(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
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
	manager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, nil)

	ctxA := withTenantContext(context.Background(), r37TenantContext(t, "ten_r37_a", identity.RoleAdmin))
	ctxB := withTenantContext(context.Background(), r37TenantContext(t, "ten_r37_b", identity.RoleAdmin))
	if _, _, err := manager.CreateServer(ctxA, mcp.CreateServerInput{
		ServerID: "tenant-a-mcp", DisplayName: "Shared MCP", Enabled: true, SandboxProfileID: "subprocess_default",
		DeclarationID: "mcp_server:tenant-a-mcp:lifecycle.start", TransportKind: mcp.TransportKindStdio,
		Command: os.Args[0], Args: []string{"-test.run=TestAPIMCPHelperProcess", "--"}, WorkingDir: t.TempDir(), AutoRestart: true,
	}); err != nil {
		t.Fatalf("create tenant A server: %v", err)
	}
	if _, _, err := manager.CreateServer(ctxB, mcp.CreateServerInput{
		ServerID: "tenant-b-mcp", DisplayName: "Shared MCP", Enabled: true, SandboxProfileID: "subprocess_default",
		DeclarationID: "mcp_server:tenant-b-mcp:lifecycle.start", TransportKind: mcp.TransportKindStdio,
		Command: os.Args[0], Args: []string{"-test.run=TestAPIMCPHelperProcess", "--"}, WorkingDir: t.TempDir(), AutoRestart: true,
	}); err != nil {
		t.Fatalf("create tenant B server: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers", nil).WithContext(ctxA)
	listRec := httptest.NewRecorder()
	handleMCPServers(manager, listRec, listReq)
	if listRec.Code != http.StatusOK || strings.Contains(listRec.Body.String(), "tenant-b-mcp") || !strings.Contains(listRec.Body.String(), "tenant-a-mcp") {
		t.Fatalf("tenant A MCP list leaked or omitted resources: status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers/tenant-a-mcp", nil).WithContext(ctxB)
	getRec := httptest.NewRecorder()
	handleMCPServerByID(manager, getRec, getReq, "tenant-a-mcp")
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant MCP get must 404, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	exposureReq := httptest.NewRequest(http.MethodPatch, "/v1/mcp/servers/tenant-a-mcp/tools/lookup", strings.NewReader(`{"runtimeSurface":"chat","exposureMode":"allow","active":true}`)).WithContext(ctxB)
	exposureRec := httptest.NewRecorder()
	handleMCPServerToolExposure(manager, exposureRec, exposureReq, "tenant-a-mcp", []string{"tools", "lookup"})
	if exposureRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant MCP exposure update must 404, got %d body=%s", exposureRec.Code, exposureRec.Body.String())
	}
}

func TestMCPCatalogInstallAndRuntimeToolInvocation(t *testing.T) {
	t.Setenv("GO_WANT_API_MCP_HELPER", "1")
	t.Setenv("API_MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_API_MCP_HELPER": "1",
		"API_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	runtimeManager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	sessionRouter := router.NewSessionRouter()
	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(nil, nil))
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Auth:        authManager,
		Policy:      policyEngine,
		Router:      sessionRouter,
		Runtime:     runtimeManager,
		Checkpoints: checkpointManager,
		Sandboxes:   sandboxes,
		MCP:         mcpManager,
		Store:       sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "mcp-runtime")

	catalogReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/catalog", nil)
	catalogReq.Header.Set("Authorization", authHeader)
	catalogRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(catalogRec, catalogReq)
	if catalogRec.Code != http.StatusOK || !strings.Contains(catalogRec.Body.String(), `"filesystem"`) {
		t.Fatalf("expected bundled catalog response, got %d body=%s", catalogRec.Code, catalogRec.Body.String())
	}

	workingDir := t.TempDir()
	installReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/catalog/filesystem/install", strings.NewReader(`{"serverId":"filesystem-test","command":"`+os.Args[0]+`","args":["-test.run=TestAPIMCPHelperProcess","--"],"workingDir":"`+workingDir+`","secretRefs":["GO_WANT_API_MCP_HELPER","API_MCP_HELPER_TOOLS"]}`))
	installReq.Header.Set("Authorization", authHeader)
	installRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(installRec, installReq)
	if installRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for catalog install, got %d body=%s", installRec.Code, installRec.Body.String())
	}
	var installResult mcp.CatalogInstallResult
	if err := json.Unmarshal(installRec.Body.Bytes(), &installResult); err != nil {
		t.Fatalf("decode install result: %v", err)
	}
	if installResult.Server == nil || installResult.Server.CatalogManagement == nil {
		t.Fatalf("expected catalog management projection on install, got %+v", installResult)
	}
	if installResult.Server.CatalogManagement.InstallInputSnapshot.Command != "" ||
		len(installResult.Server.CatalogManagement.InstallInputSnapshot.Args) != 0 ||
		installResult.Server.CatalogManagement.InstallInputSnapshot.WorkingDir != "" {
		t.Fatalf("expected install snapshot transport fields to be redacted from operator projection, got %+v", installResult.Server.CatalogManagement.InstallInputSnapshot)
	}
	if installResult.Server == nil || installResult.Server.OriginKind != mcp.OriginKindCatalog || installResult.Server.CatalogEntryID != "filesystem" {
		t.Fatalf("unexpected install result: %+v", installResult)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-test/start", nil)
	startReq.Header.Set("Authorization", authHeader)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for installed mcp start, got %d body=%s", startRec.Code, startRec.Body.String())
	}

	exposureReq := httptest.NewRequest(http.MethodPatch, "/v1/mcp/servers/filesystem-test/tools/lookup", strings.NewReader(`{"runtimeSurface":"chat","exposureMode":"allow","active":true}`))
	exposureReq.Header.Set("Authorization", authHeader)
	exposureRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(exposureRec, exposureReq)
	if exposureRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exposure update, got %d body=%s", exposureRec.Code, exposureRec.Body.String())
	}

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat", Goal: "invoke mcp"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "invoke", Kind: "tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(context.Background(), step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	toolCallReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"mcpServerId":"filesystem-test","toolName":"lookup","runtimeSurface":"chat","input":{"query":"hello"}}`))
	toolCallReq.Header.Set("Authorization", authHeader)
	toolCallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(toolCallRec, toolCallReq)
	if toolCallRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for mcp tool call create, got %d body=%s", toolCallRec.Code, toolCallRec.Body.String())
	}
	var toolCall runtime.ToolCall
	if err := json.Unmarshal(toolCallRec.Body.Bytes(), &toolCall); err != nil {
		t.Fatalf("decode tool call: %v", err)
	}
	if toolCall.InvocationKind != runtime.ToolCallInvocationKindMCPTool || toolCall.MCPServerID != "filesystem-test" || toolCall.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("unexpected mcp tool call response: %+v", toolCall)
	}
	if !strings.Contains(toolCallRec.Body.String(), `"mcpTransportKind":"stdio"`) {
		t.Fatalf("expected mcp provenance in tool call response, got %s", toolCallRec.Body.String())
	}
}

func TestMCPCatalogMaintenanceRoutes(t *testing.T) {
	t.Setenv("GO_WANT_API_MCP_HELPER", "1")
	t.Setenv("API_MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_API_MCP_HELPER": "1",
		"API_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
		"POSTGRES_DSN":           "postgres://user:pass@localhost/db",
	})
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Policy:    policyEngine,
		Sandboxes: sandboxes,
		MCP:       mcpManager,
		Store:     sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "mcp-maintenance")

	installReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/catalog/filesystem/install", strings.NewReader(`{"serverId":"filesystem-phase22","command":"`+os.Args[0]+`","args":["-test.run=TestAPIMCPHelperProcess","--"],"workingDir":"`+t.TempDir()+`","secretRefs":["GO_WANT_API_MCP_HELPER","API_MCP_HELPER_TOOLS"]}`))
	installReq.Header.Set("Authorization", authHeader)
	installRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(installRec, installReq)
	if installRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for catalog install, got %d body=%s", installRec.Code, installRec.Body.String())
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-phase22/refresh", nil)
	refreshReq.Header.Set("Authorization", authHeader)
	refreshRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for refresh, got %d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	var refreshResult mcp.CatalogLifecycleResult
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshResult); err != nil {
		t.Fatalf("decode refresh result: %v", err)
	}
	if refreshResult.Status != mcp.CatalogActionStatusCompleted || refreshResult.Server == nil || refreshResult.Server.CatalogManagement == nil {
		t.Fatalf("unexpected refresh result: %+v", refreshResult)
	}
	if refreshResult.PreflightMs > 100 {
		t.Fatalf("expected refresh preflight <=100ms, got %d", refreshResult.PreflightMs)
	}

	modifiedReq := httptest.NewRequest(http.MethodPatch, "/v1/mcp/servers/filesystem-phase22", strings.NewReader(`{"displayName":"Filesystem Modified"}`))
	modifiedReq.Header.Set("Authorization", authHeader)
	modifiedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(modifiedRec, modifiedReq)
	if modifiedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for modification patch, got %d body=%s", modifiedRec.Code, modifiedRec.Body.String())
	}

	conflictReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-phase22/refresh", nil)
	conflictReq.Header.Set("Authorization", authHeader)
	conflictRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(conflictRec, conflictReq)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for modified refresh, got %d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
	var conflictResult mcp.CatalogLifecycleResult
	if err := json.Unmarshal(conflictRec.Body.Bytes(), &conflictResult); err != nil {
		t.Fatalf("decode conflict refresh result: %v", err)
	}
	if conflictResult.FailureClass != "conflict" {
		t.Fatalf("expected conflict failure class, got %+v", conflictResult)
	}

	postgresInstallReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/catalog/postgres/install", strings.NewReader(`{"serverId":"postgres-phase22","command":"`+os.Args[0]+`","args":["-test.run=TestAPIMCPHelperProcess","--"],"workingDir":"`+t.TempDir()+`","secretRefs":["POSTGRES_DSN"]}`))
	postgresInstallReq.Header.Set("Authorization", authHeader)
	postgresInstallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(postgresInstallRec, postgresInstallReq)
	if postgresInstallRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for postgres install, got %d body=%s", postgresInstallRec.Code, postgresInstallRec.Body.String())
	}
	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{})

	revalidateReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/postgres-phase22/revalidate", nil)
	revalidateReq.Header.Set("Authorization", authHeader)
	revalidateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(revalidateRec, revalidateReq)
	if revalidateRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for blocked revalidation, got %d body=%s", revalidateRec.Code, revalidateRec.Body.String())
	}
	var revalidateResult mcp.CatalogRevalidationResult
	if err := json.Unmarshal(revalidateRec.Body.Bytes(), &revalidateResult); err != nil {
		t.Fatalf("decode revalidation result: %v", err)
	}
	if revalidateResult.Classification != mcp.RevalidationClassificationPrerequisiteLost || len(revalidateResult.Issues) == 0 {
		t.Fatalf("unexpected revalidation result: %+v", revalidateResult)
	}

	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_API_MCP_HELPER": "1",
		"API_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	runningInstallReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/catalog/filesystem/install", strings.NewReader(`{"serverId":"filesystem-revalidate","command":"`+os.Args[0]+`","args":["-test.run=TestAPIMCPHelperProcess","--"],"workingDir":"`+t.TempDir()+`","secretRefs":["GO_WANT_API_MCP_HELPER","API_MCP_HELPER_TOOLS"]}`))
	runningInstallReq.Header.Set("Authorization", authHeader)
	runningInstallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(runningInstallRec, runningInstallReq)
	if runningInstallRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for running verification install, got %d body=%s", runningInstallRec.Code, runningInstallRec.Body.String())
	}

	runningReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-revalidate/start", nil)
	runningReq.Header.Set("Authorization", authHeader)
	runningRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(runningRec, runningReq)
	if runningRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for start before running revalidation, got %d body=%s", runningRec.Code, runningRec.Body.String())
	}

	revalidateHealthyReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-revalidate/revalidate", nil)
	revalidateHealthyReq.Header.Set("Authorization", authHeader)
	revalidateHealthyRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(revalidateHealthyRec, revalidateHealthyReq)
	if revalidateHealthyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for healthy running revalidation, got %d body=%s", revalidateHealthyRec.Code, revalidateHealthyRec.Body.String())
	}
	var revalidateHealthyResult mcp.CatalogRevalidationResult
	if err := json.Unmarshal(revalidateHealthyRec.Body.Bytes(), &revalidateHealthyResult); err != nil {
		t.Fatalf("decode healthy revalidation result: %v", err)
	}
	if revalidateHealthyResult.Status != mcp.AvailabilityStatusReady || revalidateHealthyResult.Classification != mcp.RevalidationClassificationHealthy {
		t.Fatalf("unexpected healthy revalidation result: %+v", revalidateHealthyResult)
	}

	stopReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-revalidate/stop", nil)
	stopReq.Header.Set("Authorization", authHeader)
	stopRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(stopRec, stopReq)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stop before uninstall, got %d body=%s", stopRec.Code, stopRec.Body.String())
	}

	uninstallReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/filesystem-phase22/uninstall", nil)
	uninstallReq.Header.Set("Authorization", authHeader)
	uninstallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(uninstallRec, uninstallReq)
	if uninstallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for uninstall, got %d body=%s", uninstallRec.Code, uninstallRec.Body.String())
	}
	var uninstallResult mcp.CatalogLifecycleResult
	if err := json.Unmarshal(uninstallRec.Body.Bytes(), &uninstallResult); err != nil {
		t.Fatalf("decode uninstall result: %v", err)
	}
	if uninstallResult.Status != mcp.CatalogActionStatusCompleted || !uninstallResult.Removed {
		t.Fatalf("unexpected uninstall result: %+v", uninstallResult)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/servers/filesystem-phase22", nil)
	getReq.Header.Set("Authorization", authHeader)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after uninstall, got %d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestMCPTransportInspectionRoutes(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Policy:    policyEngine,
		Sandboxes: sandboxes,
		MCP:       mcpManager,
		Store:     sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "mcp-transports")

	transportReq := httptest.NewRequest(http.MethodGet, "/v1/mcp/transports", nil)
	transportReq.Header.Set("Authorization", authHeader)
	transportRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(transportRec, transportReq)
	if transportRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for transport inspection, got %d body=%s", transportRec.Code, transportRec.Body.String())
	}
	var transports ListResponse[mcp.TransportCapability]
	if err := json.Unmarshal(transportRec.Body.Bytes(), &transports); err != nil {
		t.Fatalf("decode transport capability response: %v", err)
	}
	if len(transports.Items) < 3 {
		t.Fatalf("expected additive transport capability records, got %+v", transports)
	}

	configReq := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	configReq.Header.Set("Authorization", authHeader)
	configRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for config, got %d body=%s", configRec.Code, configRec.Body.String())
	}
	var configResponse ConfigResponse
	if err := json.Unmarshal(configRec.Body.Bytes(), &configResponse); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if len(configResponse.MCP.Transports) < 3 {
		t.Fatalf("expected config to mirror transport capability records, got %+v", configResponse.MCP)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers", strings.NewReader(`{"serverId":"websocket-mcp","displayName":"Websocket MCP","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:websocket-mcp:lifecycle.start","transportKind":"websocket","endpoint":"ws://127.0.0.1:19234/mcp","websocketConfig":{"auth":{"mode":"bearer_header","secretRef":"MCP_WS_TOKEN"}},"secretRefs":["MCP_WS_TOKEN"],"autoRestart":true}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for websocket server create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var websocketServer mcp.ServerResource
	if err := json.Unmarshal(createRec.Body.Bytes(), &websocketServer); err != nil {
		t.Fatalf("decode websocket server resource: %v", err)
	}
	if websocketServer.TransportKind != mcp.TransportKindWebsocket {
		t.Fatalf("expected websocket transport kind, got %+v", websocketServer)
	}
	if websocketServer.WebsocketAuthSummary == nil || websocketServer.WebsocketAuthSummary.SecretRef != "MCP_WS_TOKEN" {
		t.Fatalf("expected redacted websocket auth summary, got %+v", websocketServer)
	}
	if websocketServer.AvailabilityStatus != mcp.AvailabilityStatusBlocked {
		t.Fatalf("expected missing websocket auth secret to block the server, got %+v", websocketServer)
	}
}

func TestMCPWebsocketRuntimeToolInvocation(t *testing.T) {
	remote := newAPIWebsocketMCPServer(t, "secret-token")
	defer remote.Close()

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAPIMCPSecretsFileForTest(t, dataDir, map[string]string{
		"MCP_WS_TOKEN": "secret-token",
	})
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	eventBus := events.NewBus()
	defer eventBus.Close()

	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	runtimeManager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	sessionRouter := router.NewSessionRouter()
	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(nil, nil))
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Auth:        authManager,
		Policy:      policyEngine,
		Router:      sessionRouter,
		Runtime:     runtimeManager,
		Checkpoints: checkpointManager,
		Sandboxes:   sandboxes,
		MCP:         mcpManager,
		Store:       sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "mcp-websocket-runtime")

	createReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers", strings.NewReader(`{"serverId":"websocket-runtime","displayName":"Websocket Runtime","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:websocket-runtime:lifecycle.start","transportKind":"websocket","endpoint":"`+remote.wsURL()+`","websocketConfig":{"auth":{"mode":"bearer_header","secretRef":"MCP_WS_TOKEN"}},"secretRefs":["MCP_WS_TOKEN"],"autoRestart":true}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for websocket create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/mcp/servers/websocket-runtime/start", nil)
	startReq.Header.Set("Authorization", authHeader)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for websocket start, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var started mcp.LifecycleResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode websocket lifecycle response: %v", err)
	}
	if started.Server.State.Status != mcp.LifecycleStatusHealthy {
		t.Fatalf("expected healthy websocket lifecycle response, got %+v", started)
	}

	exposureReq := httptest.NewRequest(http.MethodPatch, "/v1/mcp/servers/websocket-runtime/tools/lookup", strings.NewReader(`{"runtimeSurface":"chat","exposureMode":"allow","active":true}`))
	exposureReq.Header.Set("Authorization", authHeader)
	exposureRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(exposureRec, exposureReq)
	if exposureRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for websocket exposure update, got %d body=%s", exposureRec.Code, exposureRec.Body.String())
	}

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat", Goal: "invoke websocket mcp"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "invoke websocket", Kind: "tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(context.Background(), step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	toolCallReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"mcpServerId":"websocket-runtime","toolName":"lookup","runtimeSurface":"chat","input":{"query":"hello"}}`))
	toolCallReq.Header.Set("Authorization", authHeader)
	toolCallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(toolCallRec, toolCallReq)
	if toolCallRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for websocket tool call create, got %d body=%s", toolCallRec.Code, toolCallRec.Body.String())
	}
	if !strings.Contains(toolCallRec.Body.String(), `"mcpTransportKind":"websocket"`) {
		t.Fatalf("expected websocket provenance in tool call response, got %s", toolCallRec.Body.String())
	}
}

func readAPIHelperFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Content-Length:"), "content-length:"))
		if _, err := fmt.Sscanf(value, "%d", &length); err != nil {
			return nil, err
		}
	}
	if length < 0 {
		return nil, io.EOF
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type apiWebsocketMCPServer struct {
	server *httptest.Server
}

func newAPIWebsocketMCPServer(t *testing.T, token string) *apiWebsocketMCPServer {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("Authorization")) != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade websocket: %v", err)
		}
		defer conn.Close()
		for {
			var req map[string]any
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			method, _ := req["method"].(string)
			id := req["id"]
			switch method {
			case "initialize":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "api-ws-mcp", "version": "1.0.0"}}})
			case "notifications/initialized":
			case "tools/list":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"tools": []map[string]any{{"name": "lookup", "title": "Lookup", "description": "Lookup tool", "inputSchema": map[string]any{"type": "object"}}}}})
			case "tools/call":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "lookup over websocket ok"}}}})
			default:
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "method not found"}})
			}
		}
	}))
	return &apiWebsocketMCPServer{server: server}
}

func (s *apiWebsocketMCPServer) Close() {
	s.server.Close()
}

func (s *apiWebsocketMCPServer) wsURL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/mcp"
}

func writeAPIHelperFrame(value any) {
	payload, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

func writeAPIMCPSecretsFileForTest(t *testing.T, dataDir string, values map[string]string) {
	t.Helper()
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal mcp secrets: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "mcp-secrets.json"), payload, 0o600); err != nil {
		t.Fatalf("write mcp secrets: %v", err)
	}
}
