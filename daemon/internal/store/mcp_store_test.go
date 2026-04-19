package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStorePersistsMCPRecords(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	ctx := context.Background()
	now := time.Now().UTC()

	if err := sqliteStore.UpsertMCPServer(ctx, MCPServerRecord{
		ServerID:  "mcp_test",
		Enabled:   true,
		UpdatedAt: now,
		Document:  []byte(`{"serverId":"mcp_test","enabled":true}`),
	}); err != nil {
		t.Fatalf("UpsertMCPServer returned error: %v", err)
	}
	if err := sqliteStore.UpsertMCPServerState(ctx, MCPServerStateRecord{
		ServerID:  "mcp_test",
		Status:    "healthy",
		UpdatedAt: now,
		Document:  []byte(`{"serverId":"mcp_test","status":"healthy"}`),
	}); err != nil {
		t.Fatalf("UpsertMCPServerState returned error: %v", err)
	}
	if err := sqliteStore.ReplaceMCPTools(ctx, "mcp_test", []MCPToolRecord{{
		ServerID:         "mcp_test",
		ToolName:         "lookup",
		DiscoveryStatus:  "discovered",
		UpdatedAt:        now,
		LastDiscoveredAt: &now,
		Document:         []byte(`{"serverId":"mcp_test","toolName":"lookup","discoveryStatus":"discovered"}`),
	}}); err != nil {
		t.Fatalf("ReplaceMCPTools returned error: %v", err)
	}
	if err := sqliteStore.UpsertMCPToolExposureRule(ctx, MCPToolExposureRuleRecord{
		ServerID:       "mcp_test",
		ToolName:       "lookup",
		RuntimeSurface: "chat",
		ExposureMode:   "allow",
		Active:         true,
		UpdatedAt:      now,
		Document:       []byte(`{"serverId":"mcp_test","toolName":"lookup","runtimeSurface":"chat","exposureMode":"allow","active":true}`),
	}); err != nil {
		t.Fatalf("UpsertMCPToolExposureRule returned error: %v", err)
	}

	servers, err := sqliteStore.ListMCPServers(ctx)
	if err != nil {
		t.Fatalf("ListMCPServers returned error: %v", err)
	}
	if len(servers) != 1 || servers[0].ServerID != "mcp_test" || !servers[0].Enabled {
		t.Fatalf("unexpected mcp servers result: %+v", servers)
	}

	states, err := sqliteStore.ListMCPServerStates(ctx)
	if err != nil {
		t.Fatalf("ListMCPServerStates returned error: %v", err)
	}
	if len(states) != 1 || states[0].Status != "healthy" {
		t.Fatalf("unexpected mcp states result: %+v", states)
	}

	tools, err := sqliteStore.ListMCPTools(ctx, "mcp_test")
	if err != nil {
		t.Fatalf("ListMCPTools returned error: %v", err)
	}
	if len(tools) != 1 || tools[0].ToolName != "lookup" {
		t.Fatalf("unexpected mcp tools result: %+v", tools)
	}

	rules, err := sqliteStore.ListMCPToolExposureRules(ctx, "mcp_test")
	if err != nil {
		t.Fatalf("ListMCPToolExposureRules returned error: %v", err)
	}
	if len(rules) != 1 || rules[0].RuntimeSurface != "chat" || !rules[0].Active {
		t.Fatalf("unexpected mcp tool exposure result: %+v", rules)
	}
}
