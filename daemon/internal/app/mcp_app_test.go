package app

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
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/gorilla/websocket"
)

func TestAppMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_APP_MCP_HELPER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	toolsPayload := os.Getenv("APP_MCP_HELPER_TOOLS")
	if strings.TrimSpace(toolsPayload) == "" {
		toolsPayload = `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`
	}
	for {
		payload, err := readAppHelperFrame(reader)
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
			writeAppHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "app-test-mcp", "version": "1.0.0"}}})
		case "notifications/initialized":
		case "tools/list":
			var tools []map[string]any
			_ = json.Unmarshal([]byte(toolsPayload), &tools)
			writeAppHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": tools}})
		case "tools/call":
			writeAppHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "lookup ok"}}}})
		default:
			writeAppHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}
}

func TestRecoverPersistedStateRestoresMCPServers(t *testing.T) {
	t.Setenv("GO_WANT_APP_MCP_HELPER", "1")
	t.Setenv("APP_MCP_HELPER_TOOLS", `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`)

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAppMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_APP_MCP_HELPER": "1",
		"APP_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
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

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	if _, _, err := mcpManager.CreateServer(context.Background(), mcp.CreateServerInput{
		ServerID:         "restored-mcp",
		DisplayName:      "Restored MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:restored-mcp:lifecycle.start",
		TransportKind:    mcp.TransportKindStdio,
		Command:          os.Args[0],
		Args:             []string{"-test.run=TestAppMCPHelperProcess", "--"},
		WorkingDir:       t.TempDir(),
		SecretRefs:       []string{"GO_WANT_APP_MCP_HELPER", "APP_MCP_HELPER_TOOLS"},
		AutoRestart:      true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredEventBus := events.NewBus()
	defer restoredEventBus.Close()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredProviders := providers.NewManager(config.Config{}, llm.NewDispatcher())
	restoredSandboxes := sandbox.NewManager(cfg, sqliteStore, restoredEventBus, restoredPolicy)
	defer func() { _ = restoredSandboxes.Close(context.Background()) }()
	restoredMCP := mcp.NewManager(cfg, sqliteStore, restoredEventBus, restoredSandboxes, restoredPolicy, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))

	if err := recoverPersistedState(context.Background(), config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, restoredProviders, restoredSandboxes, restoredMCP, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}
	resource, ok := restoredMCP.GetServerResource("restored-mcp")
	if !ok {
		t.Fatal("expected restored mcp server resource")
	}
	if resource.State.Status != mcp.LifecycleStatusHealthy {
		t.Fatalf("expected restored mcp server to be healthy, got %+v", resource)
	}
}

func TestRecoverPersistedStateDoesNotHangOnUnresponsiveMCPServer(t *testing.T) {
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

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	if _, _, err := mcpManager.CreateServer(context.Background(), mcp.CreateServerInput{
		ServerID:         "hung-mcp",
		DisplayName:      "Hung MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:hung-mcp:lifecycle.start",
		TransportKind:    mcp.TransportKindStdio,
		Command:          "/bin/sh",
		Args:             []string{"-c", "sleep 60"},
		WorkingDir:       t.TempDir(),
		AutoRestart:      true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	t.Cleanup(mcp.SetSessionStartTimeoutForTest(50 * time.Millisecond))

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredEventBus := events.NewBus()
	defer restoredEventBus.Close()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredProviders := providers.NewManager(config.Config{}, llm.NewDispatcher())
	restoredSandboxes := sandbox.NewManager(cfg, sqliteStore, restoredEventBus, restoredPolicy)
	defer func() { _ = restoredSandboxes.Close(context.Background()) }()
	restoredMCP := mcp.NewManager(cfg, sqliteStore, restoredEventBus, restoredSandboxes, restoredPolicy, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))

	startedAt := time.Now()
	if err := recoverPersistedState(context.Background(), config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, restoredProviders, restoredSandboxes, restoredMCP, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}
	if time.Since(startedAt) > time.Second {
		t.Fatalf("expected restore to finish promptly after MCP timeout, took %s", time.Since(startedAt))
	}
	resource, ok := restoredMCP.GetServerResource("hung-mcp")
	if !ok {
		t.Fatal("expected restored hung mcp server resource")
	}
	if resource.State.Status != mcp.LifecycleStatusFailed {
		t.Fatalf("expected failed state for hung restored mcp server, got %+v", resource.State)
	}
}

func TestRecoverPersistedStateRestoresWebsocketMCPServers(t *testing.T) {
	remote := newAppWebsocketMCPServer(t, "secret-token")
	defer remote.Close()

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAppMCPSecretsFileForTest(t, dataDir, map[string]string{
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
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(nil, nil))
	if _, _, err := mcpManager.CreateServer(context.Background(), mcp.CreateServerInput{
		ServerID:         "restored-websocket-mcp",
		DisplayName:      "Restored Websocket MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:restored-websocket-mcp:lifecycle.start",
		TransportKind:    mcp.TransportKindWebsocket,
		Endpoint:         remote.wsURL(),
		WebsocketConfig: &mcp.WebsocketConfig{
			Auth: &mcp.WebsocketAuthConfig{
				Mode:      mcp.WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredEventBus := events.NewBus()
	defer restoredEventBus.Close()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredProviders := providers.NewManager(config.Config{}, llm.NewDispatcher())
	restoredSandboxes := sandbox.NewManager(cfg, sqliteStore, restoredEventBus, restoredPolicy)
	defer func() { _ = restoredSandboxes.Close(context.Background()) }()
	restoredMCP := mcp.NewManager(cfg, sqliteStore, restoredEventBus, restoredSandboxes, restoredPolicy, mcp.NewTransportMux(nil, nil))

	if err := recoverPersistedState(context.Background(), config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, restoredProviders, restoredSandboxes, restoredMCP, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}
	resource, ok := restoredMCP.GetServerResource("restored-websocket-mcp")
	if !ok {
		t.Fatal("expected restored websocket mcp server resource")
	}
	if resource.State.Status != mcp.LifecycleStatusHealthy {
		t.Fatalf("expected restored websocket mcp server to be healthy, got %+v", resource)
	}
	if resource.State.LastRecoveryClass != "restore_succeeded" {
		t.Fatalf("expected restored websocket mcp server to record restore_succeeded truth, got %+v", resource.State)
	}
	items, err := sqliteStore.ListEvents(context.Background(), events.Filter{Category: "mcp"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "mcp.server_restore_completed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected restore-completed event in history, got %+v", items)
	}
}

func TestRecoverPersistedStateRecordsWebsocketRestoreFailureTruth(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	writeAppMCPSecretsFileForTest(t, dataDir, map[string]string{
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
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxes.Close(context.Background()) }()

	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(nil, nil))
	if _, _, err := mcpManager.CreateServer(context.Background(), mcp.CreateServerInput{
		ServerID:         "restore-failed-websocket-mcp",
		DisplayName:      "Restore Failed Websocket MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:restore-failed-websocket-mcp:lifecycle.start",
		TransportKind:    mcp.TransportKindWebsocket,
		Endpoint:         "ws://127.0.0.1:1/mcp",
		WebsocketConfig: &mcp.WebsocketConfig{
			Auth: &mcp.WebsocketAuthConfig{
				Mode:      mcp.WebsocketAuthModeBearerHeader,
				SecretRef: "MCP_WS_TOKEN",
			},
		},
		SecretRefs:  []string{"MCP_WS_TOKEN"},
		AutoRestart: true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredEventBus := events.NewBus()
	defer restoredEventBus.Close()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredProviders := providers.NewManager(config.Config{}, llm.NewDispatcher())
	restoredSandboxes := sandbox.NewManager(cfg, sqliteStore, restoredEventBus, restoredPolicy)
	defer func() { _ = restoredSandboxes.Close(context.Background()) }()
	restoredMCP := mcp.NewManager(cfg, sqliteStore, restoredEventBus, restoredSandboxes, restoredPolicy, mcp.NewTransportMux(nil, nil))

	if err := recoverPersistedState(context.Background(), config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, restoredProviders, restoredSandboxes, restoredMCP, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}
	resource, ok := restoredMCP.GetServerResource("restore-failed-websocket-mcp")
	if !ok {
		t.Fatal("expected restore-failed websocket mcp server resource")
	}
	if resource.State.Status != mcp.LifecycleStatusFailed {
		t.Fatalf("expected failed restore state, got %+v", resource.State)
	}
	if resource.State.LastRecoveryClass != "restore_failed" {
		t.Fatalf("expected restore_failed truth, got %+v", resource.State)
	}
	items, err := sqliteStore.ListEvents(context.Background(), events.Filter{Category: "mcp"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "mcp.server_restore_failed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected restore-failed event in history, got %+v", items)
	}
}

func readAppHelperFrame(reader *bufio.Reader) ([]byte, error) {
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

func writeAppHelperFrame(value any) {
	payload, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

func writeAppMCPSecretsFileForTest(t *testing.T, dataDir string, values map[string]string) {
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

type appWebsocketMCPServer struct {
	server *httptest.Server
}

func newAppWebsocketMCPServer(t *testing.T, token string) *appWebsocketMCPServer {
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
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "app-ws-mcp", "version": "1.0.0"}}})
			case "notifications/initialized":
			case "tools/list":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"tools": []map[string]any{{"name": "lookup", "title": "Lookup", "description": "Lookup tool", "inputSchema": map[string]any{"type": "object"}}}}})
			case "tools/call":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "app websocket ok"}}}})
			default:
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "method not found"}})
			}
		}
	}))
	return &appWebsocketMCPServer{server: server}
}

func (s *appWebsocketMCPServer) Close() {
	s.server.Close()
}

func (s *appWebsocketMCPServer) wsURL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/mcp"
}
