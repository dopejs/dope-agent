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
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
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
		default:
			writeAPIHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}
}

func TestMCPServerRoutes(t *testing.T) {
	t.Setenv("GO_WANT_API_MCP_HELPER", "1")
	t.Setenv("API_MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)

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

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewStdioTransport())
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

func writeAPIHelperFrame(value any) {
	payload, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}
