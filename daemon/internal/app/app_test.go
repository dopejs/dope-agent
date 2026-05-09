package app

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/reminders"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

type appTestDeliveryAdapter struct {
	results []error
	sends   int
}

func (a *appTestDeliveryAdapter) Supports(kind delivery.TargetKind) bool {
	return kind == delivery.TargetKindTestSink
}

func (a *appTestDeliveryAdapter) Send(_ context.Context, _ delivery.DeliveryTarget, _ delivery.DeliveryOutcome) (delivery.SendResult, error) {
	idx := a.sends
	a.sends++
	if idx < len(a.results) && a.results[idx] != nil {
		return delivery.SendResult{TransportKind: string(delivery.TargetKindTestSink)}, a.results[idx]
	}
	return delivery.SendResult{TransportKind: string(delivery.TargetKindTestSink), ReceiptSummary: "ok"}, nil
}

func TestRecoverPersistedStateRestoresRuntimeAndEventHistory(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	seedRuntime := runtime.NewManager()
	run, err := seedRuntime.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_recovery",
		Entrypoint: "chat",
		Goal:       "recover after daemon restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := seedRuntime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "plan recovery",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	step, runUpdate, err := seedRuntime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if runUpdate != nil {
		run = *runUpdate
	}

	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastActiveAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	checkpointManager := checkpoints.NewManager(sqliteStore, seedRuntime)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	persistedEvent := events.Event{
		EventID:    "evt_recovery",
		Category:   "run",
		Name:       "run.status_changed",
		OccurredAt: time.Now().UTC(),
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"status": run.Status,
		},
	}
	if _, err := sqliteStore.AppendEvent(ctx, persistedEvent); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotRun, ok := restoredRuntime.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := restoredRuntime.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	items := restoredEventBus.List(events.Filter{RunID: run.RunID})
	if len(items) != 1 {
		t.Fatalf("expected 1 restored event, got %d", len(items))
	}
	if items[0].EventID != persistedEvent.EventID {
		t.Fatalf("expected restored event ID %s, got %s", persistedEvent.EventID, items[0].EventID)
	}

	if _, ok := restoredRouter.GetSession(run.SessionID); !ok {
		t.Fatal("expected restored session")
	}
}

func TestRecoverPersistedStateBootstrapsIdentityForExistingLocalTokens(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	token := auth.AccessToken{
		TokenID:      "tok_existing_local",
		Label:        "existing local",
		Mode:         auth.PairingModeLocal,
		TokenHash:    "hash",
		TokenPreview: "dope_preview",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := sqliteStore.UpsertAccessToken(ctx, token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	identityManager := identity.NewManager(sqliteStore)

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, identityManager, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	principals, err := sqliteStore.ListPrincipals(ctx, identity.PrincipalFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListPrincipals returned error: %v", err)
	}
	if len(principals) != 1 || principals[0].Status != identity.StatusActive {
		t.Fatalf("expected one active bootstrap principal, got %+v", principals)
	}
	tenants, err := sqliteStore.ListTenants(ctx, identity.TenantFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if len(tenants) != 1 || tenants[0].TenantKind != identity.TenantKindPersonal || tenants[0].Status != identity.StatusActive {
		t.Fatalf("expected one active personal tenant, got %+v", tenants)
	}
	grants, err := sqliteStore.ListTokenTenantGrants(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("ListTokenTenantGrants returned error: %v", err)
	}
	if len(grants) != 1 || grants[0].TenantID != tenants[0].TenantID || !grants[0].IsDefault || grants[0].Status != identity.StatusActive {
		t.Fatalf("expected default token grant to bootstrap tenant, got %+v", grants)
	}
	restoredToken, ok := restoredAuth.GetToken(token.TokenID)
	if !ok {
		t.Fatal("expected restored auth token")
	}
	if restoredToken.PrincipalID != principals[0].PrincipalID || restoredToken.DefaultTenantID != tenants[0].TenantID {
		t.Fatalf("expected restored token identity fields, got %+v", restoredToken)
	}
	persistedTokens, err := sqliteStore.ListAccessTokens(ctx)
	if err != nil {
		t.Fatalf("ListAccessTokens returned error: %v", err)
	}
	if len(persistedTokens) != 1 || persistedTokens[0].PrincipalID != principals[0].PrincipalID || persistedTokens[0].DefaultTenantID != tenants[0].TenantID {
		t.Fatalf("expected persisted token identity fields, got %+v", persistedTokens)
	}
}

func TestRecoverPersistedStateBridgesLocalCredentialsIntoDefaultTenant(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := sqliteStore.UpsertAccessToken(ctx, auth.AccessToken{
		TokenID:      "tok_bridge",
		Label:        "bridge",
		Mode:         auth.PairingModeLocal,
		TokenHash:    "hash",
		TokenPreview: "dope_preview",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}
	writeAppJSONFile(t, filepath.Join(dataDir, "mcp-secrets.json"), map[string]string{
		"MCP_TOKEN":                  "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK",
		"R37_SHARED_BRIDGED_SECRET":  "shared-value",
		"R37_CONFLICT_BRIDGED_TOKEN": "one",
	})
	writeAppJSONFile(t, filepath.Join(dataDir, "skill-secrets.json"), map[string]string{
		"SKILL_TOKEN":                "skill-value",
		"R37_SHARED_BRIDGED_SECRET":  "shared-value",
		"R37_CONFLICT_BRIDGED_TOKEN": "two",
	})
	now = time.Now().UTC()
	if err := sqliteStore.UpsertProviderAuthState(ctx, providers.AuthState{
		ProviderID:    "legacy-provider",
		Family:        providers.FamilyOpenAICompatible,
		AuthMode:      providers.AuthModeAPIKey,
		Status:        providers.AuthStatusAuthenticated,
		CLIAvailable:  true,
		AccountLabel:  "legacy account",
		LastCheckedAt: now,
	}); err != nil {
		t.Fatalf("seed legacy provider auth: %v", err)
	}
	if err := sqliteStore.UpsertIntegration(ctx, integrations.Resource{
		IntegrationID:    "legacy-integration",
		DomainKind:       "calendar",
		DisplayName:      "Legacy Integration",
		EnvironmentScope: string(config.EnvironmentTest),
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateHealthy,
		AccountBinding:   integrations.AccountBinding{AccountKey: "legacy@example.com"},
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindManagedProvider, BackendRefID: "legacy-provider"},
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}); err != nil {
		t.Fatalf("seed legacy integration: %v", err)
	}
	if err := sqliteStore.UpsertConnector(ctx, connectors.Connector{
		ConnectorID:    "legacy-connector",
		Kind:           "discord",
		DisplayName:    "Legacy Connector",
		Status:         connectors.StatusHealthy,
		SecretRefs:     []string{"R37_CONFLICT_BRIDGED_TOKEN"},
		FailureCount:   0,
		RestartCount:   0,
		BackoffSeconds: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("seed legacy connector: %v", err)
	}
	mcpServerDocument := mustAppJSON(t, map[string]any{
		"serverId":      "legacy-mcp",
		"displayName":   "Legacy MCP",
		"enabled":       true,
		"transportKind": "stdio",
		"command":       "fake",
		"secretRefs":    []string{"R37_CONFLICT_BRIDGED_TOKEN"},
	})
	if err := sqliteStore.UpsertMCPServer(ctx, store.MCPServerRecord{ServerID: "legacy-mcp", Enabled: true, UpdatedAt: now, Document: mcpServerDocument}); err != nil {
		t.Fatalf("seed legacy mcp server: %v", err)
	}
	if err := sqliteStore.UpsertMCPServerState(ctx, store.MCPServerStateRecord{ServerID: "legacy-mcp", Status: "healthy", UpdatedAt: now, Document: mustAppJSON(t, map[string]any{"serverId": "legacy-mcp", "status": "healthy"})}); err != nil {
		t.Fatalf("seed legacy mcp state: %v", err)
	}
	if err := sqliteStore.UpsertMCPTool(ctx, store.MCPToolRecord{ServerID: "legacy-mcp", ToolName: "lookup", DiscoveryStatus: "discovered", UpdatedAt: now, Document: mustAppJSON(t, map[string]any{"name": "lookup"})}); err != nil {
		t.Fatalf("seed legacy mcp tool: %v", err)
	}
	if err := sqliteStore.UpsertMCPToolExposureRule(ctx, store.MCPToolExposureRuleRecord{ServerID: "legacy-mcp", ToolName: "lookup", RuntimeSurface: "chat", ExposureMode: "allow", Active: true, UpdatedAt: now, Document: mustAppJSON(t, map[string]any{"toolName": "lookup", "runtimeSurface": "chat"})}); err != nil {
		t.Fatalf("seed legacy mcp exposure: %v", err)
	}
	secretBackend, err := secrets.NewLocalBackend(filepath.Join(dataDir, "tenant-secret-values"))
	if err != nil {
		t.Fatalf("NewLocalBackend returned error: %v", err)
	}
	secretManager := secrets.NewManager(sqliteStore, secretBackend)
	identityManager := identity.NewManager(sqliteStore)

	if err := recoverPersistedStateWithSecrets(ctx, dataDir, config.EnvironmentTest, sqliteStore, router.NewSessionRouter(), checkpoints.NewManager(sqliteStore, runtime.NewManager()), events.NewBus(), connectors.NewSupervisor(), capabilities.NewSupervisor(), policy.NewEngine(), auth.NewManager(), identityManager, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, secretManager, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedStateWithSecrets returned error: %v", err)
	}
	tenants, err := sqliteStore.ListTenants(ctx, identity.TenantFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("expected one default tenant, got %+v", tenants)
	}
	defaultTenantID := tenants[0].TenantID
	for _, ref := range []string{"MCP_TOKEN", "SKILL_TOKEN", "R37_SHARED_BRIDGED_SECRET"} {
		resolved, err := secretManager.Resolve(ctx, secrets.ResolveInput{TenantID: defaultTenantID, SecretRef: ref})
		if err != nil {
			t.Fatalf("resolve bridged secret %s: %v", ref, err)
		}
		if resolved.Value == "" || resolved.Value == secrets.RedactedValue {
			t.Fatalf("unexpected bridged value for %s", ref)
		}
	}
	conflict, err := secretManager.Get(ctx, defaultTenantID, "R37_CONFLICT_BRIDGED_TOKEN")
	if err != nil {
		t.Fatalf("get conflict metadata: %v", err)
	}
	if conflict.Status != secrets.SecretStatusPendingRemediation || conflict.DisabledReason != "legacy_secret_ref_conflict" {
		t.Fatalf("expected conflict to be disabled for remediation, got %+v", conflict)
	}
	var versionCount int
	providerStates, err := sqliteStore.ListProviderAuthStates(ctx)
	if err != nil {
		t.Fatalf("list provider auth states: %v", err)
	}
	if len(providerStates) == 0 || providerStates[0].TenantID != defaultTenantID {
		t.Fatalf("expected bridged provider auth tenant %s, got %+v", defaultTenantID, providerStates)
	}
	integrationItems, err := sqliteStore.ListIntegrations(ctx, string(config.EnvironmentTest))
	if err != nil {
		t.Fatalf("list integrations: %v", err)
	}
	if len(integrationItems) == 0 || integrationItems[0].TenantID != defaultTenantID {
		t.Fatalf("expected bridged integration tenant %s, got %+v", defaultTenantID, integrationItems)
	}
	connectorItems, err := sqliteStore.ListConnectors(ctx)
	if err != nil {
		t.Fatalf("list connectors: %v", err)
	}
	if len(connectorItems) == 0 || connectorItems[0].TenantID != defaultTenantID || connectorItems[0].Status != connectors.StatusDisabled || connectorItems[0].DisabledReason != "legacy_secret_ref_conflict" {
		t.Fatalf("expected bridged disabled connector for tenant %s, got %+v", defaultTenantID, connectorItems)
	}
	var mcpTenantID string
	var mcpEnabled int
	var mcpDocumentRaw string
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT tenant_id, enabled, document_json FROM mcp_servers WHERE server_id = ?`, "legacy-mcp").Scan(&mcpTenantID, &mcpEnabled, &mcpDocumentRaw); err != nil {
		t.Fatalf("load bridged mcp server: %v", err)
	}
	var mcpDocument map[string]any
	if err := json.Unmarshal([]byte(mcpDocumentRaw), &mcpDocument); err != nil {
		t.Fatalf("decode bridged mcp server document: %v", err)
	}
	if mcpTenantID != defaultTenantID || mcpEnabled != 0 || mcpDocument["tenantId"] != defaultTenantID || mcpDocument["enabled"] != false {
		t.Fatalf("expected bridged disabled mcp server for tenant %s, tenant=%q enabled=%d document=%+v", defaultTenantID, mcpTenantID, mcpEnabled, mcpDocument)
	}
	var mcpStateTenantID, mcpStateStatus string
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT tenant_id, status FROM mcp_server_states WHERE server_id = ?`, "legacy-mcp").Scan(&mcpStateTenantID, &mcpStateStatus); err != nil {
		t.Fatalf("load bridged mcp state: %v", err)
	}
	if mcpStateTenantID != defaultTenantID || mcpStateStatus != "disabled" {
		t.Fatalf("expected bridged disabled mcp state, tenant=%q status=%q", mcpStateTenantID, mcpStateStatus)
	}
	var mcpToolTenantID, exposureTenantID string
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT tenant_id FROM mcp_tools WHERE server_id = ? AND tool_name = ?`, "legacy-mcp", "lookup").Scan(&mcpToolTenantID); err != nil {
		t.Fatalf("load bridged mcp tool: %v", err)
	}
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT tenant_id FROM mcp_tool_exposure_rules WHERE server_id = ? AND tool_name = ? AND runtime_surface = ?`, "legacy-mcp", "lookup", "chat").Scan(&exposureTenantID); err != nil {
		t.Fatalf("load bridged mcp exposure: %v", err)
	}
	if mcpToolTenantID != defaultTenantID || exposureTenantID != defaultTenantID {
		t.Fatalf("expected bridged mcp tool/exposure tenant %s, got tool=%q exposure=%q", defaultTenantID, mcpToolTenantID, exposureTenantID)
	}
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_secret_versions WHERE tenant_id = ? AND secret_ref = ?`, defaultTenantID, "MCP_TOKEN").Scan(&versionCount); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected one version before restart, got %d", versionCount)
	}
	if err := recoverPersistedStateWithSecrets(ctx, dataDir, config.EnvironmentTest, sqliteStore, router.NewSessionRouter(), checkpoints.NewManager(sqliteStore, runtime.NewManager()), events.NewBus(), connectors.NewSupervisor(), capabilities.NewSupervisor(), policy.NewEngine(), auth.NewManager(), identityManager, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, secretManager, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("second recoverPersistedStateWithSecrets returned error: %v", err)
	}
	if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_secret_versions WHERE tenant_id = ? AND secret_ref = ?`, defaultTenantID, "MCP_TOKEN").Scan(&versionCount); err != nil {
		t.Fatalf("count versions after restart: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("bridge duplicated versions after restart: %d", versionCount)
	}
}

func TestAppNewWiresEnabledSlackRuntime(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DOPE_ENV", "test")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")
	t.Setenv("DOPE_CONNECTORS_SLACK_ENABLED", "true")
	t.Setenv("DOPE_CONNECTORS_SLACK_CONNECTOR_ID", "slack-main")
	t.Setenv("DOPE_CONNECTORS_SLACK_DISPLAY_NAME", "Slack Main")
	t.Setenv("DOPE_CONNECTORS_SLACK_WORKSPACE_BINDING_ID", "workspace_binding_redacted")
	t.Setenv("DOPE_CONNECTORS_SLACK_WORKSPACE_ID", "workspace_redacted")
	t.Setenv("DOPE_CONNECTORS_SLACK_ALLOWED_DM_USER_IDS", "user_allowed")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() { _ = application.Close(context.Background()) })

	if application.slackRuntime == nil {
		t.Fatal("expected enabled Slack connector to create runtime")
	}
}

func TestRecoverPersistedStateRestoresTokenLifecycleAndTenantGrants(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	ctx := context.Background()
	now := time.Now().UTC()
	expiresAt := now.Add(-time.Minute)
	revokedAt := now.Add(-2 * time.Minute)
	principal := identity.Principal{PrincipalID: "prn_token_owner", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Token Owner", Status: identity.StatusActive, DefaultTenantID: "ten_token", CreatedAt: now, UpdatedAt: now}
	tenant := identity.Tenant{TenantID: principal.DefaultTenantID, TenantKind: identity.TenantKindOrganization, DisplayName: "Token Tenant", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now, CreatedByPrincipalID: principal.PrincipalID, DefaultOwnerPrincipalID: principal.PrincipalID}
	if err := sqliteStore.UpsertPrincipal(ctx, principal); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	if err := sqliteStore.UpsertTenant(ctx, tenant); err != nil {
		t.Fatalf("UpsertTenant returned error: %v", err)
	}
	if err := sqliteStore.UpsertMembership(ctx, identity.Membership{MembershipID: "mem_token_owner", TenantID: tenant.TenantID, PrincipalID: principal.PrincipalID, Role: identity.RoleOwner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now, AcceptedAt: &now}); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}
	oldToken := auth.AccessToken{TokenID: "tok_rotated_old", PrincipalID: principal.PrincipalID, Label: "old", Mode: auth.PairingModeToken, TokenHash: "old_hash", TokenPreview: "dope_old", Status: string(identity.StatusRotated), DefaultTenantID: tenant.TenantID, CreatedAt: now, UpdatedAt: now, ExpiresAt: &expiresAt, RotatedToTokenID: "tok_rotated_new"}
	newToken := auth.AccessToken{TokenID: "tok_rotated_new", PrincipalID: principal.PrincipalID, Label: "new", Mode: auth.PairingModeToken, TokenHash: "new_hash", TokenPreview: "dope_new", Status: string(identity.StatusRevoked), DefaultTenantID: tenant.TenantID, CreatedAt: now, UpdatedAt: now, RevokedAt: &revokedAt, RotatedFromTokenID: oldToken.TokenID}
	for _, token := range []auth.AccessToken{oldToken, newToken} {
		if err := sqliteStore.UpsertAccessToken(ctx, token); err != nil {
			t.Fatalf("UpsertAccessToken returned error: %v", err)
		}
	}
	if err := sqliteStore.UpsertTokenTenantGrant(ctx, identity.TokenTenantGrant{GrantID: "grant_token_new", TokenID: newToken.TokenID, TenantID: tenant.TenantID, IsDefault: true, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now, GrantedByPrincipalID: principal.PrincipalID}); err != nil {
		t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	identityManager := identity.NewManager(sqliteStore)

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, identityManager, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}
	restoredOld, ok := restoredAuth.GetToken(oldToken.TokenID)
	if !ok || restoredOld.Status != string(identity.StatusRotated) || restoredOld.RotatedToTokenID != newToken.TokenID || restoredOld.ExpiresAt == nil {
		t.Fatalf("expected restored rotated token, got ok=%v %+v", ok, restoredOld)
	}
	restoredNew, ok := restoredAuth.GetToken(newToken.TokenID)
	if !ok || restoredNew.Status != string(identity.StatusRevoked) || restoredNew.RotatedFromTokenID != oldToken.TokenID || restoredNew.RevokedAt == nil {
		t.Fatalf("expected restored revoked replacement token, got ok=%v %+v", ok, restoredNew)
	}
	grants, err := sqliteStore.ListTokenTenantGrants(ctx, newToken.TokenID)
	if err != nil {
		t.Fatalf("ListTokenTenantGrants returned error: %v", err)
	}
	if len(grants) != 1 || grants[0].TenantID != tenant.TenantID || !grants[0].IsDefault || grants[0].Status != identity.StatusActive {
		t.Fatalf("expected restored token tenant grant, got %+v", grants)
	}
}

func TestAppRunRestoresPendingDeliveryLifecycle(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}

	seedManager := delivery.NewManager("test", events.NewBus(), sqliteStore, &appTestDeliveryAdapter{
		results: []error{context.DeadlineExceeded},
	})
	seedManager.ConfigureForTesting(3, time.Hour, time.Hour)
	target, err := seedManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "app-restore-target",
		DisplayName:      "App Restore Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := seedManager.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "app-restore-pref",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: target.TargetID,
			delivery.ResultClassUrgent:         target.TargetID,
			delivery.ResultClassFailure:        target.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}
	outcome, err := seedManager.EmitOutcome(context.Background(), delivery.OutcomeInput{
		SourceKind:     "run",
		SourceID:       "app_restore_run",
		RunID:          "app_restore_run",
		ResultClass:    delivery.ResultClassFailure,
		PayloadPreview: "restore in app run",
	})
	if err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}
	outcome, ok, err := seedManager.GetOutcome(context.Background(), outcome.DeliveryID)
	if err != nil || !ok {
		t.Fatalf("GetOutcome returned ok=%v err=%v", ok, err)
	}
	attempt := outcome.Attempts[0]
	retrySoon := time.Now().UTC().Add(20 * time.Millisecond)
	attempt.NextRetryAt = &retrySoon
	if err := sqliteStore.UpsertDeliveryAttempt(context.Background(), store.DeliveryAttemptRecord{
		AttemptID:     attempt.AttemptID,
		DeliveryID:    attempt.DeliveryID,
		AttemptNumber: attempt.AttemptNumber,
		TargetID:      attempt.TargetID,
		Status:        string(attempt.Status),
		NextRetryAt:   attempt.NextRetryAt,
		Document:      mustMarshalDeliveryAttempt(t, attempt),
	}); err != nil {
		t.Fatalf("UpsertDeliveryAttempt returned error: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	liveStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore(live) returned error: %v", err)
	}
	successAdapter := &appTestDeliveryAdapter{}
	sharedEventBus := events.NewBus()
	deliveryManager := delivery.NewManager("test", sharedEventBus, liveStore, successAdapter)
	deliveryManager.ConfigureForTesting(3, 10*time.Millisecond, 20*time.Millisecond)
	server := api.NewServer(api.Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
		},
		Logger:   telemetry.New("error").Slog(),
		EventBus: sharedEventBus,
		Delivery: deliveryManager,
		Store:    liveStore,
	})
	app := &App{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
		},
		Logger:   telemetry.New("error"),
		Store:    liveStore,
		EventBus: sharedEventBus,
		Delivery: deliveryManager,
		Server:   server,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- app.Run(ctx)
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	checkStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore(check) returned error: %v", err)
	}
	defer func() { _ = checkStore.Close() }()
	checkManager := delivery.NewManager("test", events.NewBus(), checkStore, successAdapter)
	final, ok, err := checkManager.GetOutcome(context.Background(), outcome.DeliveryID)
	if err != nil || !ok {
		t.Fatalf("GetOutcome(final) returned ok=%v err=%v", ok, err)
	}
	if final.Status != delivery.OutcomeStatusDelivered || len(final.Attempts) != 2 {
		t.Fatalf("expected restored delivery to complete on second attempt, got %+v", final)
	}
}

func mustMarshalDeliveryAttempt(t *testing.T, attempt delivery.DeliveryAttempt) []byte {
	t.Helper()
	data, err := json.Marshal(attempt)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return data
}

func TestRecoverPersistedStateRestoresIntegrations(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	item := integrations.Resource{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusDegraded,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateDegraded,
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "acct_calendar",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
		Provenance: integrations.Provenance{
			SecretResolution:      "resolved",
			SecretMaterialPresent: true,
			EnvironmentScope:      "test",
			BackedBy:              string(integrations.BackendKindFakeLocal),
		},
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now,
		LastTransitionAt: now,
	}
	if err := sqliteStore.UpsertIntegration(ctx, item); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredIntegrations := integrations.NewManager("test")

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, restoredIntegrations, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	got, ok := restoredIntegrations.Get(item.IntegrationID)
	if !ok {
		t.Fatal("expected restored integration")
	}
	if got.ReadinessStatus != integrations.ReadinessStatusDegraded || got.AccountBinding.AccountKey != item.AccountBinding.AccountKey || !got.CanonicalDefault {
		t.Fatalf("expected restored integration state %+v, got %+v", item, got)
	}
}

func TestRecoverPersistedStateRestoresCalendarDomainState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	integrationManager := integrations.NewManager("test")
	resource, err := integrationManager.Create(integrations.CreateInput{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "acct_calendar",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	resource, err = integrationManager.UpdateReadiness(resource.IntegrationID, integrations.UpdateReadinessInput{
		ReadinessStatus: integrations.ReadinessStatusHealthy,
		AuthState:       integrations.AuthStateAuthorized,
		HealthState:     integrations.HealthStateHealthy,
	})
	if err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if err := sqliteStore.UpsertIntegration(ctx, resource); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	seedCalendar := calendar.NewManager("test")
	account, created, operation, artifacts, err := seedCalendar.CreateEvent([]integrations.Resource{resource}, calendar.CreateEventInput{
		Selection: calendar.Selection{IntegrationID: resource.IntegrationID},
		Title:     "Recovered event",
		StartsAt:  time.Date(2026, 4, 23, 20, 0, 0, 0, time.UTC),
		EndsAt:    time.Date(2026, 4, 23, 21, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}
	if err := sqliteStore.UpsertCalendarAccount(ctx, account); err != nil {
		t.Fatalf("UpsertCalendarAccount returned error: %v", err)
	}
	if err := sqliteStore.UpsertCalendarOperation(ctx, operation); err != nil {
		t.Fatalf("UpsertCalendarOperation returned error: %v", err)
	}
	for _, artifact := range artifacts {
		if err := sqliteStore.UpsertCalendarArtifact(ctx, artifact); err != nil {
			t.Fatalf("UpsertCalendarArtifact returned error: %v", err)
		}
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredIntegrations := integrations.NewManager("test")
	restoredCalendar := calendar.NewManager("test")

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, restoredIntegrations, restoredCalendar, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	restoredAccount, ok := restoredCalendar.GetAccount(resource.IntegrationID)
	if !ok || restoredAccount.IntegrationID != resource.IntegrationID {
		t.Fatalf("expected restored calendar account, got %+v ok=%v", restoredAccount, ok)
	}
	restoredOperation, ok := restoredCalendar.GetOperation(operation.OperationID)
	if !ok || restoredOperation.ExternalEventID != created.ExternalEventID {
		t.Fatalf("expected restored calendar operation, got %+v ok=%v", restoredOperation, ok)
	}
	restoredArtifacts := restoredCalendar.ListArtifacts(operation.OperationID)
	if len(restoredArtifacts) == 0 {
		t.Fatal("expected restored calendar artifacts")
	}
	_, restoredEvent, _, _, err := restoredCalendar.GetEvent(restoredIntegrations.List(), calendar.GetEventInput{
		Selection:       calendar.Selection{IntegrationID: resource.IntegrationID},
		ExternalEventID: created.ExternalEventID,
	})
	if err != nil {
		t.Fatalf("GetEvent after restore returned error: %v", err)
	}
	if restoredEvent.Title != created.Title {
		t.Fatalf("expected restored event snapshot %q, got %+v", created.Title, restoredEvent)
	}
}

func TestRecoverPersistedStateRestoresMailDomainState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	resource, err := integrations.NewManager("test").Create(integrations.CreateInput{
		IntegrationID:    "mail-a",
		DomainKind:       "mail",
		DisplayName:      "Mail A",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "alice@example.com",
			AccountLabel: "Alice Mailbox",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	resource.ReadinessStatus = integrations.ReadinessStatusHealthy
	resource.AuthState = integrations.AuthStateAuthorized
	resource.HealthState = integrations.HealthStateHealthy
	if err := sqliteStore.UpsertIntegration(ctx, resource); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	seedMail := mail.NewManager("test")
	account, draft, operation, artifacts, err := seedMail.CreateDraft([]integrations.Resource{resource}, mail.CreateDraftInput{
		Selection:   mail.Selection{IntegrationID: resource.IntegrationID},
		ComposeMode: mail.ComposeModeNewMessage,
		To:          []string{"carol@example.com"},
		Subject:     "Recovered draft",
		Body:        "Recovered mail body",
	})
	if err != nil {
		t.Fatalf("CreateDraft returned error: %v", err)
	}
	if err := sqliteStore.UpsertMailAccount(ctx, account); err != nil {
		t.Fatalf("UpsertMailAccount returned error: %v", err)
	}
	if err := sqliteStore.UpsertMailOperation(ctx, operation); err != nil {
		t.Fatalf("UpsertMailOperation returned error: %v", err)
	}
	for _, artifact := range artifacts {
		if err := sqliteStore.UpsertMailArtifact(ctx, artifact); err != nil {
			t.Fatalf("UpsertMailArtifact returned error: %v", err)
		}
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredIntegrations := integrations.NewManager("test")
	restoredMail := mail.NewManager("test")

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, restoredIntegrations, nil, restoredMail, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	restoredAccount, ok := restoredMail.GetAccount(resource.IntegrationID)
	if !ok || restoredAccount.IntegrationID != resource.IntegrationID {
		t.Fatalf("expected restored mail account, got %+v ok=%v", restoredAccount, ok)
	}
	restoredOperation, ok := restoredMail.GetOperation(operation.OperationID)
	if !ok || restoredOperation.DraftID != draft.DraftID {
		t.Fatalf("expected restored mail operation, got %+v ok=%v", restoredOperation, ok)
	}
	restoredArtifacts := restoredMail.ListArtifacts(operation.OperationID)
	if len(restoredArtifacts) == 0 {
		t.Fatal("expected restored mail artifacts")
	}
	_, restoredDraft, _, _, err := restoredMail.GetDraft(restoredIntegrations.List(), mail.GetDraftInput{
		Selection: mail.Selection{IntegrationID: resource.IntegrationID},
		DraftID:   draft.DraftID,
	})
	if err != nil {
		t.Fatalf("GetDraft after restore returned error: %v", err)
	}
	if restoredDraft.Subject != draft.Subject {
		t.Fatalf("expected restored draft subject %q, got %+v", draft.Subject, restoredDraft)
	}
}

func TestRecoverPersistedStateRestoresIntegrationBindingsOnApprovalsAndToolCalls(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	seedRuntime := runtime.NewManager()
	run, err := seedRuntime.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_integration_restore",
		Entrypoint: "operator",
		Goal:       "restore integration probe state",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := seedRuntime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "probe integration",
		Kind:  "integration_probe",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	binding := integrations.BindingSummary{
		IntegrationID:         "calendar-a",
		DomainKind:            "calendar",
		DisplayName:           "Calendar A",
		AccountKey:            "acct_calendar",
		CanonicalDefault:      true,
		ReadinessAtInvocation: integrations.ReadinessStatusDegraded,
		BackendKind:           integrations.BackendKindFakeLocal,
		SecretResolution:      "resolved",
		EnvironmentScope:      "test",
		CapturedAt:            now,
	}
	toolCall, err := seedRuntime.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		CapabilityID:        "integration_probe",
		ToolName:            "inspect",
		IntegrationBindings: []integrations.BindingSummary{binding},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	toolCall, err = seedRuntime.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{
		Output: map[string]any{"message": "ok"},
	})
	if err != nil {
		t.Fatalf("CompleteToolCall returned error: %v", err)
	}

	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    now.Add(-time.Minute),
		UpdatedAt:    now.Add(-time.Minute),
		LastActiveAt: now.Add(-time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertToolCall(ctx, toolCall); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}

	approval := policy.Approval{
		ApprovalID:          "approval_integration_restore",
		Action:              "integration.probe.mutate",
		ResourceKind:        "integration",
		ResourceID:          "calendar-a",
		Reason:              "mutation probe",
		RequestedBy:         "run:" + run.RunID,
		Status:              policy.ApprovalStatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		IntegrationBindings: []integrations.BindingSummary{binding},
	}
	if err := sqliteStore.UpsertApproval(ctx, approval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	integration := integrations.Resource{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusDegraded,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateDegraded,
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey: "acct_calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now,
		LastTransitionAt: now,
	}
	if err := sqliteStore.UpsertIntegration(ctx, integration); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	checkpointManager := checkpoints.NewManager(sqliteStore, seedRuntime)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredIntegrations := integrations.NewManager("test")

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, restoredIntegrations, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	restoredToolCall, ok := restoredRuntime.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected restored tool call")
	}
	if len(restoredToolCall.IntegrationBindings) != 1 || restoredToolCall.IntegrationBindings[0].IntegrationID != binding.IntegrationID {
		t.Fatalf("expected restored tool call integration bindings, got %+v", restoredToolCall)
	}

	restoredApproval, ok := restoredPolicy.GetApproval(approval.ApprovalID)
	if !ok {
		t.Fatal("expected restored approval")
	}
	if len(restoredApproval.IntegrationBindings) != 1 || restoredApproval.IntegrationBindings[0].IntegrationID != binding.IntegrationID {
		t.Fatalf("expected restored approval integration bindings, got %+v", restoredApproval)
	}

	restoredIntegration, ok := restoredIntegrations.Get(integration.IntegrationID)
	if !ok {
		t.Fatal("expected restored integration")
	}
	if restoredIntegration.ReadinessStatus != integrations.ReadinessStatusDegraded {
		t.Fatalf("expected degraded integration after restore, got %+v", restoredIntegration)
	}
}

func TestRecoverPersistedStateCancelsInFlightSandboxToolCalls(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	seedRuntime := runtime.NewManager()
	run, err := seedRuntime.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_recovery_cancelled",
		Entrypoint: "chat",
		Goal:       "cancel in-flight tool call on restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := seedRuntime.CreateStep(run.RunID, runtime.CreateStepInput{Title: "execute tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	toolCall, err := seedRuntime.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		CapabilityID:       "shell",
		ToolName:           "shell",
		Input:              map[string]any{"cmd": "pwd"},
		SandboxExecutionID: "sandbox_exec_restore_1",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	toolCall, err = seedRuntime.MarkToolCallRunning(run.RunID, step.StepID, toolCall.ToolCallID, "sandbox_exec_restore_1", map[string]any{
		"policyRecord": map[string]any{"policyRecordId": "policy_restore_1"},
	})
	if err != nil {
		t.Fatalf("MarkToolCallRunning returned error: %v", err)
	}

	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastActiveAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertToolCall(ctx, toolCall); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}
	checkpointManager := checkpoints.NewManager(sqliteStore, seedRuntime)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	startedAt := time.Now().UTC().Add(-time.Minute)
	execution := sandbox.Execution{
		ExecutionID:  "sandbox_exec_restore_1",
		ProfileID:    sandbox.ProfileIDSubprocessDefault,
		BackendKind:  sandbox.BackendKindSubprocess,
		Command:      "/bin/sh",
		Cwd:          t.TempDir(),
		RequestedBy:  "test",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Scope:        "tool_call",
		Status:       sandbox.ExecutionStatusRunning,
		RequestedAt:  startedAt,
		UpdatedAt:    startedAt,
		Result: sandbox.Result{
			Status: sandbox.ExecutionStatusRunning,
		},
		Consumer: &sandbox.ConsumerContractView{
			PolicyRecord: &sandbox.ConsumerPolicyRecord{
				PolicyRecordID:     "policy_restore_1",
				ConsumerKind:       sandbox.ConsumerKindLocalTool,
				ConsumerID:         "shell",
				OperationKind:      "tool_call.execute",
				ToolCallID:         toolCall.ToolCallID,
				SandboxExecutionID: "sandbox_exec_restore_1",
				StartedAt:          startedAt,
				Status:             sandbox.PolicyRecordStatusRunning,
			},
		},
	}
	document, err := json.Marshal(execution)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := sqliteStore.UpsertSandboxExecution(ctx, store.SandboxExecutionRecord{
		ExecutionID: execution.ExecutionID,
		Status:      string(execution.Status),
		ApprovalID:  "",
		StartedAt:   &startedAt,
		CompletedAt: nil,
		Document:    document,
	}); err != nil {
		t.Fatalf("UpsertSandboxExecution returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	restoredProviders := providers.NewManager(config.Config{}, llm.NewDispatcher())
	restoredSandboxes := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     t.TempDir(),
	}, sqliteStore, restoredEventBus, restoredPolicy)
	defer func() {
		if err := restoredSandboxes.Close(context.Background()); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, nil, nil, restoredPolicy, restoredAuth, nil, restoredProviders, restoredSandboxes, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotToolCall, ok := restoredRuntime.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected restored tool call")
	}
	if gotToolCall.Status != runtime.ToolCallStatusCancelled {
		t.Fatalf("expected cancelled restored tool call, got %+v", gotToolCall)
	}
	if gotToolCall.FailureClass != string(sandbox.ErrorClassCancelled) {
		t.Fatalf("expected cancelled failure class, got %+v", gotToolCall)
	}
	restoredExecution, ok := restoredSandboxes.GetExecution(execution.ExecutionID)
	if !ok || restoredExecution.Status != sandbox.ExecutionStatusCancelled {
		t.Fatalf("expected cancelled restored sandbox execution, got %+v", restoredExecution)
	}
	persistedToolCalls, err := sqliteStore.ListToolCalls(ctx, run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(persistedToolCalls) != 1 || persistedToolCalls[0].Status != runtime.ToolCallStatusCancelled {
		t.Fatalf("expected persisted cancelled tool call, got %+v", persistedToolCalls)
	}
}

func TestRecoverPersistedStatePreservesSchedulesAndCatchUpDispatchesOnlyLatestOverdue(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	seedRuntime := runtime.NewManager()
	seedCheckpoints := checkpoints.NewManager(sqliteStore, seedRuntime)
	seedClock := &appTestClock{now: time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC)}
	seedScheduler := scheduler.New(scheduler.Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			DataDir:     sqliteStore.DataDir,
		},
		Runtime:     seedRuntime,
		EventBus:    events.NewBus(),
		Store:       sqliteStore,
		Checkpoints: seedCheckpoints,
		Clock:       seedClock,
	})

	created, err := seedScheduler.Create(context.Background(), scheduler.CreateInput{
		Trigger: scheduler.Trigger{
			Kind:     scheduler.TriggerKindCron,
			CronExpr: "*/1 * * * *",
			Timezone: "UTC",
		},
		Target: scheduler.Target{
			Kind: scheduler.TargetKindRun,
			Run: &scheduler.RunTarget{
				Entrypoint: "operator",
				Goal:       "restart catch-up",
			},
		},
		RetryPolicy: scheduler.RetryPolicy{MaxRetries: 0, BackoffKind: scheduler.RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	if err := recoverPersistedState(context.Background(), config.EnvironmentTest, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	catchUpClock := &appTestClock{now: time.Date(2026, 4, 22, 14, 4, 30, 0, time.UTC)}
	catchUpScheduler := scheduler.New(scheduler.Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			DataDir:     sqliteStore.DataDir,
		},
		Runtime:     restoredRuntime,
		EventBus:    restoredEventBus,
		Store:       sqliteStore,
		Checkpoints: restoreCheckpoints,
		Clock:       catchUpClock,
	})
	if err := catchUpScheduler.CatchUp(context.Background()); err != nil {
		t.Fatalf("CatchUp returned error: %v", err)
	}

	got, ok, err := catchUpScheduler.Get(context.Background(), created.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("expected one missed and one dispatched attempt, got %+v", got.Attempts)
	}
	if got.Attempts[1].DispatchStatus != scheduler.DispatchStatusMissed || got.Attempts[1].MissedCount != 4 {
		t.Fatalf("expected visible missed intervals, got %+v", got.Attempts)
	}
	if got.Attempts[0].DispatchStatus != scheduler.DispatchStatusDispatched {
		t.Fatalf("expected latest overdue dispatch, got %+v", got.Attempts[0])
	}
	if got.Attempts[0].DueAt.After(catchUpClock.now) {
		t.Fatalf("expected catch-up dispatch to use latest overdue dueAt, got %+v", got.Attempts[0])
	}
	runs := restoredRuntime.ListRuns()
	if len(runs) != 1 || runs[0].ScheduleID != created.ScheduleID {
		t.Fatalf("expected exactly one catch-up run with schedule linkage, got %+v", runs)
	}
}

func TestSchedulerCatchUpStaysUnder2SecondsFor100PersistedSchedules(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	seedRuntime := runtime.NewManager()
	seedCheckpoints := checkpoints.NewManager(sqliteStore, seedRuntime)
	seedClock := &appTestClock{now: time.Date(2026, 4, 22, 16, 0, 0, 0, time.UTC)}
	seedScheduler := scheduler.New(scheduler.Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			DataDir:     sqliteStore.DataDir,
		},
		Runtime:     seedRuntime,
		EventBus:    events.NewBus(),
		Store:       sqliteStore,
		Checkpoints: seedCheckpoints,
		Clock:       seedClock,
	})
	for idx := 0; idx < 100; idx++ {
		if _, err := seedScheduler.Create(context.Background(), scheduler.CreateInput{
			Trigger: scheduler.Trigger{
				Kind:     scheduler.TriggerKindCron,
				CronExpr: "*/1 * * * *",
				Timezone: "UTC",
			},
			Target: scheduler.Target{
				Kind: scheduler.TargetKindRun,
				Run: &scheduler.RunTarget{
					Entrypoint: "operator",
					Goal:       "bulk catch-up",
				},
			},
			RetryPolicy: scheduler.RetryPolicy{MaxRetries: 0, BackoffKind: scheduler.RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
		}); err != nil {
			t.Fatalf("Create(%d) returned error: %v", idx, err)
		}
	}

	catchUpClock := &appTestClock{now: time.Date(2026, 4, 22, 16, 4, 30, 0, time.UTC)}
	catchUpRuntime := runtime.NewManager()
	catchUpScheduler := scheduler.New(scheduler.Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			DataDir:     sqliteStore.DataDir,
		},
		Runtime:     catchUpRuntime,
		EventBus:    events.NewBus(),
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, catchUpRuntime),
		Clock:       catchUpClock,
	})
	started := time.Now()
	if err := catchUpScheduler.CatchUp(context.Background()); err != nil {
		t.Fatalf("CatchUp returned error: %v", err)
	}
	elapsed := time.Since(started)
	if elapsed > 2*time.Second {
		t.Fatalf("expected catch-up under 2s, got %s", elapsed)
	}
}

type appTestClock struct {
	now time.Time
}

func (c *appTestClock) Now() time.Time {
	return c.now
}

func TestRecoverPersistedStateInterruptsInFlightWorkflows(t *testing.T) {
	t.Parallel()

	dataDir := filepath.Join(t.TempDir(), "dope-data")
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_workflow_restore",
		Entrypoint: "operator",
		Goal:       "interrupt workflow on restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	now := time.Now().UTC()
	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "workflow",
		RoutingKey:   "direct:local:local:workflow",
		Generation:   1,
		CreatedAt:    now.Add(-2 * time.Minute),
		UpdatedAt:    now.Add(-2 * time.Minute),
		LastActiveAt: now.Add(-2 * time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_restore_interrupt",
		RunID:            run.RunID,
		EnvironmentScope: "test",
		Goal:             run.Goal,
		Status:           orchestration.WorkflowStatusRunning,
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now.Add(-time.Minute),
		StartedAt:        ptrAppTime(now.Add(-50 * time.Second)),
		Steps: []orchestration.WorkflowStep{{
			WorkflowStepID: "wfstep_restore_interrupt",
			WorkflowID:     "wf_restore_interrupt",
			Title:          "running step",
			Position:       1,
			ConsumerKind:   "skill",
			ConsumerID:     "exec-skill",
			ToolName:       "exec-skill",
			Status:         orchestration.StepStatusRunning,
			AttemptCount:   1,
			MaxAttempts:    2,
			CreatedAt:      now.Add(-time.Minute),
			UpdatedAt:      now.Add(-30 * time.Second),
		}},
	}
	if err := sqliteStore.UpsertWorkflow(ctx, workflow); err != nil {
		t.Fatalf("UpsertWorkflow returned error: %v", err)
	}
	if err := sqliteStore.ReplaceWorkflowSteps(ctx, workflow.WorkflowID, workflow.Steps); err != nil {
		t.Fatalf("ReplaceWorkflowSteps returned error: %v", err)
	}

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, router.NewSessionRouter(), checkpoints.NewManager(sqliteStore, runtimeManager), events.NewBus(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	restored, ok, err := sqliteStore.GetWorkflow(ctx, "test", run.RunID, workflow.WorkflowID)
	if err != nil {
		t.Fatalf("GetWorkflow returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected restored workflow")
	}
	if restored.Status != orchestration.WorkflowStatusInterrupted {
		t.Fatalf("expected interrupted workflow, got %+v", restored)
	}
	if restored.InterruptedAt == nil || restored.Steps[0].Status != orchestration.StepStatusInterrupted {
		t.Fatalf("expected interrupted workflow-step truth, got %+v", restored)
	}
}

func TestRecoverPersistedStateInterruptsInFlightComputerUseTruth(t *testing.T) {
	t.Parallel()

	dataDir := filepath.Join(t.TempDir(), "dope-data")
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_computer_use_restore",
		Entrypoint: "operator",
		Goal:       "interrupt computer-use on restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	now := time.Now().UTC()
	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "computer-use",
		RoutingKey:   "direct:local:local:computer-use",
		Generation:   1,
		CreatedAt:    now.Add(-2 * time.Minute),
		UpdatedAt:    now.Add(-2 * time.Minute),
		LastActiveAt: now.Add(-2 * time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "computer-use click",
		Kind:  "computer_use",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	step, _, err = runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(planning) returned error: %v", err)
	}
	step, _, err = runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusExecutingTool,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(executing_tool) returned error: %v", err)
	}
	step, _, err = runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusBlocked,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(blocked) returned error: %v", err)
	}
	toolCall, err := runtimeManager.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		ToolName:             "click",
		InvocationKind:       runtime.ToolCallInvocationKindLocalTool,
		CapabilityID:         "browser",
		ComputerUseSessionID: "cusess_restore_interrupt",
		ComputerUseActionID:  "cuact_restore_interrupt",
		Input:                map[string]any{"actionKind": "click"},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}
	if err := sqliteStore.UpsertToolCall(ctx, toolCall); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}

	cuSession := computeruse.Session{
		ComputerUseSessionID: "cusess_restore_interrupt",
		EnvironmentScope:     "test",
		RunID:                run.RunID,
		Status:               computeruse.SessionStatusBlocked,
		DriverKind:           "browser",
		StartedAt:            now.Add(-time.Minute),
		UpdatedAt:            now.Add(-30 * time.Second),
	}
	if err := sqliteStore.UpsertComputerUseSession(ctx, cuSession); err != nil {
		t.Fatalf("UpsertComputerUseSession returned error: %v", err)
	}
	cuAction := computeruse.Action{
		ComputerUseActionID:  "cuact_restore_interrupt",
		EnvironmentScope:     "test",
		ComputerUseSessionID: cuSession.ComputerUseSessionID,
		RunID:                run.RunID,
		StepID:               step.StepID,
		ToolCallID:           toolCall.ToolCallID,
		ActionKind:           computeruse.ActionKindClick,
		Status:               computeruse.ActionStatusWaitingApproval,
		RiskLevel:            computeruse.RiskLevelHigh,
		ApprovalID:           "approval_restore_interrupt",
		RequestedAt:          now.Add(-40 * time.Second),
		UpdatedAt:            now.Add(-30 * time.Second),
	}
	if err := sqliteStore.UpsertComputerUseAction(ctx, cuAction); err != nil {
		t.Fatalf("UpsertComputerUseAction returned error: %v", err)
	}

	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, router.NewSessionRouter(), checkpoints.NewManager(sqliteStore, runtimeManager), events.NewBus(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	restoredSession, ok, err := sqliteStore.GetComputerUseSession(ctx, "test", run.RunID, cuSession.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("GetComputerUseSession returned error: %v", err)
	}
	if !ok || restoredSession.Status != computeruse.SessionStatusInterrupted || restoredSession.InterruptedAt == nil {
		t.Fatalf("expected interrupted computer-use session, got ok=%v session=%+v", ok, restoredSession)
	}

	restoredAction, ok, err := sqliteStore.GetComputerUseAction(ctx, "test", run.RunID, cuSession.ComputerUseSessionID, cuAction.ComputerUseActionID)
	if err != nil {
		t.Fatalf("GetComputerUseAction returned error: %v", err)
	}
	if !ok || restoredAction.Status != computeruse.ActionStatusInterrupted || restoredAction.FailureClass != string(computeruse.FailureClassInterrupted) {
		t.Fatalf("expected interrupted computer-use action, got ok=%v action=%+v", ok, restoredAction)
	}

	restoredToolCall, ok := runtimeManager.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected restored tool call")
	}
	if restoredToolCall.Status != runtime.ToolCallStatusFailed || restoredToolCall.FailureClass != string(computeruse.FailureClassInterrupted) {
		t.Fatalf("expected interrupted runtime tool call truth, got %+v", restoredToolCall)
	}
}

func ptrAppTime(value time.Time) *time.Time {
	return &value
}

func TestRecoverPersistedStateRestoresSupervisionState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	connectorHeartbeat := time.Now().UTC().Add(-30 * time.Second)
	connector := connectors.Connector{
		ConnectorID:     "slack-main",
		Kind:            "slack",
		DisplayName:     "Slack Main",
		Status:          connectors.StatusHealthy,
		FailureCount:    0,
		RestartCount:    2,
		LastHeartbeatAt: &connectorHeartbeat,
		CreatedAt:       time.Now().UTC().Add(-time.Hour),
		UpdatedAt:       time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertConnector(ctx, connector); err != nil {
		t.Fatalf("UpsertConnector returned error: %v", err)
	}

	capabilityRestart := time.Now().UTC().Add(-20 * time.Second)
	capability := capabilities.Capability{
		CapabilityID:   "shell",
		Kind:           "exec",
		DisplayName:    "Shell",
		Status:         capabilities.StatusBackingOff,
		FailureCount:   3,
		RestartCount:   1,
		BackoffSeconds: 20,
		LastRestartAt:  &capabilityRestart,
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
		UpdatedAt:      time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertCapability(ctx, capability); err != nil {
		t.Fatalf("UpsertCapability returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotConnector, ok := restoredConnectors.Get(connector.ConnectorID)
	if !ok {
		t.Fatal("expected restored connector")
	}
	if gotConnector.Status != connectors.StatusHealthy {
		t.Fatalf("expected restored connector status healthy, got %s", gotConnector.Status)
	}
	if gotConnector.RestartCount != 2 {
		t.Fatalf("expected restored connector restart count 2, got %d", gotConnector.RestartCount)
	}

	gotCapability, ok := restoredCapabilities.Get(capability.CapabilityID)
	if !ok {
		t.Fatal("expected restored capability")
	}
	if gotCapability.Status != capabilities.StatusBackingOff {
		t.Fatalf("expected restored capability status backing_off, got %s", gotCapability.Status)
	}
	if gotCapability.BackoffSeconds != 20 {
		t.Fatalf("expected restored capability backoff 20, got %d", gotCapability.BackoffSeconds)
	}
}

func TestRecoverPersistedStateRestoresAuthAndPolicyState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	completedAt := time.Now().UTC().Add(-2 * time.Minute)
	pairing := auth.Pairing{
		PairingID:   "pair_restore",
		Mode:        auth.PairingModeLocal,
		Label:       "web-ui",
		Status:      auth.PairingStatusCompleted,
		CodeHash:    "hash",
		CreatedAt:   time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-5 * time.Minute),
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
		CompletedAt: &completedAt,
	}
	if err := sqliteStore.UpsertPairing(ctx, pairing); err != nil {
		t.Fatalf("UpsertPairing returned error: %v", err)
	}

	token := auth.AccessToken{
		TokenID:      "tok_restore",
		Label:        "web-ui",
		Mode:         auth.PairingModeLocal,
		TokenHash:    "token-hash",
		TokenPreview: "dope_preview",
		CreatedAt:    time.Now().UTC().Add(-9 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertAccessToken(ctx, token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}

	approval := policy.Approval{
		ApprovalID:   "approval_restore",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "needs approval",
		RequestedBy:  "web-ui",
		Status:       policy.ApprovalStatusApproved,
		CreatedAt:    time.Now().UTC().Add(-8 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-7 * time.Minute),
		ResolvedAt:   &completedAt,
		Resolution:   string(policy.ApprovalStatusApproved),
	}
	if err := sqliteStore.UpsertApproval(ctx, approval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	decision := policy.Decision{
		DecisionID:   "decision_restore",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Outcome:      policy.DecisionOutcomeApproved,
		Reason:       "needs approval",
		ApprovalID:   approval.ApprovalID,
		CreatedAt:    time.Now().UTC().Add(-7 * time.Minute),
	}
	if err := sqliteStore.UpsertDecision(ctx, decision); err != nil {
		t.Fatalf("UpsertDecision returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, nil, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	if got, ok := restoredAuth.GetPairing(pairing.PairingID); !ok || got.Status != auth.PairingStatusCompleted {
		t.Fatalf("expected restored completed pairing, got %+v ok=%v", got, ok)
	}
	if got, ok := restoredAuth.GetToken(token.TokenID); !ok || got.TokenPreview != token.TokenPreview {
		t.Fatalf("expected restored token, got %+v ok=%v", got, ok)
	}
	if got, ok := restoredPolicy.GetApproval(approval.ApprovalID); !ok || got.Status != policy.ApprovalStatusApproved {
		t.Fatalf("expected restored approved approval, got %+v ok=%v", got, ok)
	}
	if len(restoredPolicy.ListDecisions()) != 1 {
		t.Fatalf("expected 1 restored decision, got %d", len(restoredPolicy.ListDecisions()))
	}
}

func TestAppRunPublishesLifecycleEvents(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	reopenedStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := reopenedStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	items, err := reopenedStore.ListEvents(context.Background(), events.Filter{Category: "system"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 system events, got %d", len(items))
	}
	if items[0].Name != "system.started" {
		t.Fatalf("expected system.started, got %s", items[0].Name)
	}
	if items[1].Name != "system.stopped" {
		t.Fatalf("expected system.stopped, got %s", items[1].Name)
	}
	if items[1].Sequence <= items[0].Sequence {
		t.Fatalf("expected monotonic system event sequence, got %d then %d", items[0].Sequence, items[1].Sequence)
	}
}

func TestNewLoadsEvaluationFixturesAndSupportsReplayComparison(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DOPE_ENV", "test")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		if err := application.Close(context.Background()); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	fixtures, err := application.Evaluation.ListFixtures(ctx, evaluation.FixtureFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListFixtures returned error: %v", err)
	}
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 evaluation fixtures, got %+v", fixtures)
	}

	seen := map[evaluation.FixtureDomainClass]bool{}
	for _, fixture := range fixtures {
		seen[fixture.DomainClass] = true
	}
	for _, domain := range []evaluation.FixtureDomainClass{evaluation.FixtureDomainSchedule, evaluation.FixtureDomainIntegration, evaluation.FixtureDomainComputerUse} {
		if !seen[domain] {
			t.Fatalf("expected loaded fixture for %s, got %+v", domain, fixtures)
		}
	}

	candidates, err := application.Evaluation.ListReplayCandidates(ctx, evaluation.CandidateFilter{EnvironmentScope: "test", CandidateKind: evaluation.CandidateKindFixture})
	if err != nil {
		t.Fatalf("ListReplayCandidates returned error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 fixture replay candidates, got %+v", candidates)
	}
	for _, candidate := range candidates {
		attempt, err := application.Evaluation.CreateReplayAttempt(ctx, candidate.CandidateID, evaluation.CreateReplayAttemptInput{})
		if err != nil {
			t.Fatalf("CreateReplayAttempt(%s) returned error: %v", candidate.CandidateID, err)
		}
		if attempt.Status != evaluation.ReplayAttemptStatusCompleted {
			t.Fatalf("expected completed fixture replay for %s, got %+v", candidate.CandidateID, attempt)
		}
		comparison, err := application.Evaluation.CreateComparison(ctx, attempt.AttemptID, evaluation.CreateComparisonInput{})
		if err != nil {
			t.Fatalf("CreateComparison(%s) returned error: %v", attempt.AttemptID, err)
		}
		if comparison.TerminalStatus != evaluation.ComparisonMatched {
			t.Fatalf("expected matched fixture comparison for %s, got %+v", attempt.AttemptID, comparison)
		}
	}
}

func TestAppRunCatchesUpDueReminderAfterRestart(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	ctx := context.Background()
	seedStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore(seed) returned error: %v", err)
	}

	seedEventBus := events.NewBus()
	seedDelivery := delivery.NewManager("test", seedEventBus, seedStore, delivery.NewTestSinkAdapter())
	target, err := seedDelivery.CreateTarget(ctx, delivery.DeliveryTarget{
		TargetID:         "restart-reminder-target",
		DisplayName:      "Restart Reminder Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := seedDelivery.UpsertPreference(ctx, delivery.DeliveryPreference{
		PreferenceID:     "restart-reminder-pref",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: target.TargetID,
			delivery.ResultClassUrgent:         target.TargetID,
			delivery.ResultClassFailure:        target.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}

	seedReminders := reminders.NewManager(reminders.Dependencies{
		EnvironmentScope: "test",
		Store:            seedStore,
		EventBus:         seedEventBus,
		Delivery:         seedDelivery,
	})
	fireAt := time.Now().UTC().Add(75 * time.Millisecond)
	created, err := seedReminders.Create(ctx, reminders.CreateInput{
		Title:        "Restart reminder catch-up",
		BehaviorMode: reminders.BehaviorModeNotifyOnly,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: &fireAt,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := seedStore.Close(); err != nil {
		t.Fatalf("Close(seed) returned error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	t.Setenv("DOPE_ENV", "test")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(runCtx)
	}()
	defer func() {
		cancel()
		if err := <-errCh; err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	}()

	var current reminders.Reminder
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		item, ok, err := application.Reminders.Get(ctx, created.ReminderID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if ok && item.ActiveOccurrenceID != "" && item.CurrentState == reminders.StateDue {
			current = item
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if current.ActiveOccurrenceID == "" || current.CurrentState != reminders.StateDue {
		t.Fatalf("expected due reminder after restart catch-up, got %+v", current)
	}

	occurrence, ok, err := application.Reminders.GetOccurrence(ctx, current.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence returned ok=%v err=%v", ok, err)
	}
	if occurrence.State != reminders.StateDue || occurrence.LatestDeliveryID == "" || occurrence.LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) {
		t.Fatalf("expected delivered due occurrence after restart catch-up, got %+v", occurrence)
	}
}

func TestAppRestartRestoresRuntimeBoundary(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	authHeader := testAuthHeader(t, first.Auth)

	createRunRec := httptest.NewRecorder()
	createRunReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"restart-safe"}`))
	createRunReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(createRunRec, createRunReq)
	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for run create, got %d body=%s", createRunRec.Code, createRunRec.Body.String())
	}
	var createdRun runtime.Run
	if err := json.Unmarshal(createRunRec.Body.Bytes(), &createdRun); err != nil {
		t.Fatalf("failed to decode created run: %v", err)
	}

	createStepRec := httptest.NewRecorder()
	createStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps", strings.NewReader(`{"title":"recover me","kind":"task"}`))
	createStepReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(createStepRec, createStepReq)
	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for step create, got %d", createStepRec.Code)
	}
	var createdStep runtime.Step
	if err := json.Unmarshal(createStepRec.Body.Bytes(), &createdStep); err != nil {
		t.Fatalf("failed to decode created step: %v", err)
	}

	updateStepRec := httptest.NewRecorder()
	updateStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps/"+createdStep.StepID+"/status", strings.NewReader(`{"status":"planning"}`))
	updateStepReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(updateStepRec, updateStepReq)
	if updateStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step status update, got %d", updateStepRec.Code)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	gotRun, ok := second.Runtime.GetRun(createdRun.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := second.Runtime.GetStep(createdRun.RunID, createdStep.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	eventsRec := httptest.NewRecorder()
	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+createdRun.RunID+"/events", nil)
	eventsReq.Header.Set("Authorization", testAuthHeader(t, second.Auth))
	second.Server.Handler().ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events after restart, got %d", eventsRec.Code)
	}
	var eventList struct {
		Items      []events.Event `json:"items"`
		NextCursor int64          `json:"nextCursor"`
	}
	if err := json.Unmarshal(eventsRec.Body.Bytes(), &eventList); err != nil {
		t.Fatalf("failed to decode run events response: %v", err)
	}
	if len(eventList.Items) < 3 {
		t.Fatalf("expected at least 3 restored events, got %d", len(eventList.Items))
	}
	if eventList.NextCursor == 0 {
		t.Fatal("expected next cursor after restart")
	}
}

func TestAppRestartRestoresConnectorIngressBoundRuns(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	authHeader := testAuthHeader(t, first.Auth)

	registerConnectorRec := httptest.NewRecorder()
	registerConnectorReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`))
	registerConnectorReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(registerConnectorRec, registerConnectorReq)
	if registerConnectorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector create, got %d body=%s", registerConnectorRec.Code, registerConnectorRec.Body.String())
	}

	ingressRec := httptest.NewRecorder()
	ingressReq := httptest.NewRequest(http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", strings.NewReader(`{
		"route":{"kind":"group","accountId":"bot-main","peerId":"chat-1","threadId":"thread-1"},
		"message":{"messageId":"msg_1","text":"hello"},
		"run":{"entrypoint":"connector.message","goal":"restart ingress"}
	}`))
	ingressReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(ingressRec, ingressReq)
	if ingressRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for ingress, got %d body=%s", ingressRec.Code, ingressRec.Body.String())
	}

	var ingressResponse api.ConnectorIngressMessageResponse
	if err := json.Unmarshal(ingressRec.Body.Bytes(), &ingressResponse); err != nil {
		t.Fatalf("failed to decode ingress response: %v", err)
	}
	if ingressResponse.Run == nil || ingressResponse.Session == nil {
		t.Fatal("expected ingress-created run")
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}

	restoredRun, ok := second.Runtime.GetRun(ingressResponse.Run.RunID)
	if !ok {
		t.Fatal("expected ingress-created run to be restored")
	}
	if restoredRun.SessionID != ingressResponse.Session.SessionID {
		t.Fatalf("expected restored run session %s, got %s", ingressResponse.Session.SessionID, restoredRun.SessionID)
	}
	restoredSession, ok := second.Router.GetSession(ingressResponse.Session.SessionID)
	if !ok {
		t.Fatal("expected ingress-created session to be restored")
	}
	if restoredSession.Channel != "telegram" {
		t.Fatalf("expected restored session channel telegram, got %s", restoredSession.Channel)
	}
}

func TestAppRestartRestoresAuthAndApprovalAPIState(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	startRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/start", strings.NewReader(`{"mode":"local","label":"web-ui"}`)))
	if startRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pairing start, got %d", startRec.Code)
	}
	var pairingStart struct {
		Pairing     auth.Pairing `json:"pairing"`
		PairingCode string       `json:"pairingCode"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &pairingStart); err != nil {
		t.Fatalf("failed to decode pairing start response: %v", err)
	}

	completeRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(completeRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/"+pairingStart.Pairing.PairingID+"/complete", strings.NewReader(`{"code":"`+pairingStart.PairingCode+`"}`)))
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pairing complete, got %d", completeRec.Code)
	}
	var pairingComplete struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &pairingComplete); err != nil {
		t.Fatalf("failed to decode pairing complete response: %v", err)
	}
	authHeader := "Bearer " + pairingComplete.AccessToken

	createApprovalReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals", strings.NewReader(`{"action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","reason":"restart persistence"}`))
	createApprovalReq.Header.Set("Authorization", authHeader)
	createApprovalRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createApprovalRec, createApprovalReq)
	if createApprovalRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval create, got %d body=%s", createApprovalRec.Code, createApprovalRec.Body.String())
	}
	var created struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(createApprovalRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode approval create response: %v", err)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", authHeader)
	meRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me after restart, got %d body=%s", meRec.Code, meRec.Body.String())
	}

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+created.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restored policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restored); err != nil {
		t.Fatalf("failed to decode approval get response: %v", err)
	}
	if restored.ApprovalID != created.Approval.ApprovalID {
		t.Fatalf("expected approval ID %s, got %s", created.Approval.ApprovalID, restored.ApprovalID)
	}
}

func TestAppRestartRestoresHighRiskApprovalSandboxProvenance(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	startRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/start", strings.NewReader(`{"mode":"local","label":"web-ui"}`)))
	if startRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pairing start, got %d", startRec.Code)
	}
	var pairingStart struct {
		Pairing     auth.Pairing `json:"pairing"`
		PairingCode string       `json:"pairingCode"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &pairingStart); err != nil {
		t.Fatalf("failed to decode pairing start response: %v", err)
	}

	completeRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(completeRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/"+pairingStart.Pairing.PairingID+"/complete", strings.NewReader(`{"code":"`+pairingStart.PairingCode+`"}`)))
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pairing complete, got %d", completeRec.Code)
	}
	var pairingComplete struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &pairingComplete); err != nil {
		t.Fatalf("failed to decode pairing complete response: %v", err)
	}
	authHeader := "Bearer " + pairingComplete.AccessToken

	if _, _, err := first.CapabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register shell capability returned error: %v", err)
	}
	run, err := first.Runtime.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := first.Runtime.CreateStep(run.RunID, runtime.CreateStepInput{Title: "approval restart"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for pending approval tool call, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval response: %v", err)
	}
	if pending.Approval.Sandbox == nil || pending.Decision.Sandbox == nil {
		t.Fatalf("expected pending approval response sandbox provenance, got approval=%+v decision=%+v", pending.Approval.Sandbox, pending.Decision.Sandbox)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+pending.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restoredApproval policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restoredApproval); err != nil {
		t.Fatalf("failed to decode restored approval: %v", err)
	}
	if restoredApproval.Sandbox == nil {
		t.Fatalf("expected restored approval sandbox provenance, got %+v", restoredApproval)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"rejected","comment":"still denied"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval resolve after restart, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}
	var resolved struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("failed to decode resolved approval response: %v", err)
	}
	if resolved.Approval.Sandbox == nil || resolved.Decision.Sandbox == nil {
		t.Fatalf("expected resolved approval response sandbox provenance after restart, got approval=%+v decision=%+v", resolved.Approval.Sandbox, resolved.Decision.Sandbox)
	}
}

func TestAppRestartPreservesOperatorVisibleSandboxLinkageForCancelledToolCall(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	authHeader := testAuthHeader(t, first.Auth)
	if _, _, err := first.CapabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "shell",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register shell capability returned error: %v", err)
	}
	run, err := first.Runtime.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := first.Runtime.CreateStep(run.RunID, runtime.CreateStepInput{Title: "restart sandbox linkage"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"sleep 5"}}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for pending shell approval, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending shell approval: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"restart coverage"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for shell approval resolve, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	launchReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+pending.Approval.ApprovalID+`","input":{"cmd":"sleep 5"}}`))
	launchReq.Header.Set("Authorization", authHeader)
	launchRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(launchRec, launchReq)
	if launchRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved shell launch, got %d body=%s", launchRec.Code, launchRec.Body.String())
	}
	var created runtime.ToolCall
	if err := json.Unmarshal(launchRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created tool call: %v", err)
	}
	if created.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox execution linkage before restart, got %+v", created)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	toolCallReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+created.ToolCallID, nil)
	toolCallReq.Header.Set("Authorization", authHeader)
	toolCallRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(toolCallRec, toolCallReq)
	if toolCallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for restored tool call get, got %d body=%s", toolCallRec.Code, toolCallRec.Body.String())
	}
	var restoredToolCall runtime.ToolCall
	if err := json.Unmarshal(toolCallRec.Body.Bytes(), &restoredToolCall); err != nil {
		t.Fatalf("failed to decode restored tool call: %v", err)
	}
	if restoredToolCall.Status != runtime.ToolCallStatusCancelled || restoredToolCall.SandboxExecutionID != created.SandboxExecutionID {
		t.Fatalf("expected cancelled restored tool call with sandbox linkage, got %+v", restoredToolCall)
	}
	if restoredToolCall.Sandbox == nil {
		t.Fatalf("expected sandbox provenance on restored tool call, got %+v", restoredToolCall)
	}

	executionReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/executions/"+created.SandboxExecutionID, nil)
	executionReq.Header.Set("Authorization", authHeader)
	executionRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(executionRec, executionReq)
	if executionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for restored sandbox execution get, got %d body=%s", executionRec.Code, executionRec.Body.String())
	}
	var execution sandbox.Execution
	if err := json.Unmarshal(executionRec.Body.Bytes(), &execution); err != nil {
		t.Fatalf("failed to decode restored sandbox execution: %v", err)
	}
	if execution.Status != sandbox.ExecutionStatusCancelled || execution.Consumer == nil || execution.Consumer.PolicyRecord == nil {
		t.Fatalf("expected cancelled restored sandbox execution with consumer view, got %+v", execution)
	}
	if execution.Consumer.PolicyRecord.ToolCallID != created.ToolCallID || execution.Consumer.PolicyRecord.SandboxExecutionID != created.SandboxExecutionID {
		t.Fatalf("expected restored sandbox execution to preserve tool-call linkage, got %+v", execution.Consumer.PolicyRecord)
	}
}

func TestAppRestartRestoresOperatorStateAcrossSubsystems(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	authHeader := testAuthHeader(t, first.Auth)

	createConnectorReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`))
	createConnectorReq.Header.Set("Authorization", authHeader)
	createConnectorRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createConnectorRec, createConnectorReq)
	if createConnectorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector create, got %d body=%s", createConnectorRec.Code, createConnectorRec.Body.String())
	}

	createCapabilityReq := httptest.NewRequest(http.MethodPost, "/v1/capabilities", strings.NewReader(`{"capabilityId":"docs","kind":"docs","displayName":"Docs"}`))
	createCapabilityReq.Header.Set("Authorization", authHeader)
	createCapabilityRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createCapabilityRec, createCapabilityReq)
	if createCapabilityRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for capability create, got %d body=%s", createCapabilityRec.Code, createCapabilityRec.Body.String())
	}

	createDispatchReq := httptest.NewRequest(http.MethodPost, "/v1/llm/dispatches", strings.NewReader(`{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"hello restart"}]}`))
	createDispatchReq.Header.Set("Authorization", authHeader)
	createDispatchRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createDispatchRec, createDispatchReq)
	if createDispatchRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for llm dispatch create, got %d body=%s", createDispatchRec.Code, createDispatchRec.Body.String())
	}
	var createdDispatch llm.Dispatch
	if err := json.Unmarshal(createDispatchRec.Body.Bytes(), &createdDispatch); err != nil {
		t.Fatalf("failed to decode llm dispatch create response: %v", err)
	}

	createApprovalReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals", strings.NewReader(`{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"restart coverage"}`))
	createApprovalReq.Header.Set("Authorization", authHeader)
	createApprovalRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createApprovalRec, createApprovalReq)
	if createApprovalRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval create, got %d body=%s", createApprovalRec.Code, createApprovalRec.Body.String())
	}
	var created struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(createApprovalRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode approval create response: %v", err)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	connectorsReq := httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	connectorsReq.Header.Set("Authorization", authHeader)
	connectorsRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(connectorsRec, connectorsReq)
	if connectorsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for connector list after restart, got %d body=%s", connectorsRec.Code, connectorsRec.Body.String())
	}
	var connectorList struct {
		Items []connectors.Connector `json:"items"`
	}
	if err := json.Unmarshal(connectorsRec.Body.Bytes(), &connectorList); err != nil {
		t.Fatalf("failed to decode connector list response: %v", err)
	}
	if len(connectorList.Items) != 1 || connectorList.Items[0].ConnectorID != "telegram-main" {
		t.Fatalf("expected restored connector telegram-main, got %+v", connectorList.Items)
	}

	capabilitiesReq := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	capabilitiesReq.Header.Set("Authorization", authHeader)
	capabilitiesRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(capabilitiesRec, capabilitiesReq)
	if capabilitiesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for capability list after restart, got %d body=%s", capabilitiesRec.Code, capabilitiesRec.Body.String())
	}
	var capabilityList struct {
		Items []capabilities.Capability `json:"items"`
	}
	if err := json.Unmarshal(capabilitiesRec.Body.Bytes(), &capabilityList); err != nil {
		t.Fatalf("failed to decode capability list response: %v", err)
	}
	if len(capabilityList.Items) != 1 || capabilityList.Items[0].CapabilityID != "docs" {
		t.Fatalf("expected restored capability docs, got %+v", capabilityList.Items)
	}

	dispatchesReq := httptest.NewRequest(http.MethodGet, "/v1/llm/dispatches", nil)
	dispatchesReq.Header.Set("Authorization", authHeader)
	dispatchesRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(dispatchesRec, dispatchesReq)
	if dispatchesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for llm dispatch list after restart, got %d body=%s", dispatchesRec.Code, dispatchesRec.Body.String())
	}
	var dispatchList struct {
		Items []llm.Dispatch `json:"items"`
	}
	if err := json.Unmarshal(dispatchesRec.Body.Bytes(), &dispatchList); err != nil {
		t.Fatalf("failed to decode llm dispatch list response: %v", err)
	}
	if len(dispatchList.Items) != 1 || dispatchList.Items[0].DispatchID != createdDispatch.DispatchID {
		t.Fatalf("expected restored llm dispatch %s, got %+v", createdDispatch.DispatchID, dispatchList.Items)
	}

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+created.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restoredApproval policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restoredApproval); err != nil {
		t.Fatalf("failed to decode approval get response: %v", err)
	}
	if restoredApproval.ApprovalID != created.Approval.ApprovalID {
		t.Fatalf("expected restored approval ID %s, got %s", created.Approval.ApprovalID, restoredApproval.ApprovalID)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", authHeader)
	meRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me after restart, got %d body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestNewConfiguresOpenAICompatibleProviderAndServesChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Fatalf("expected bearer auth header, got %q", r.Header.Get("Authorization"))
			}
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"stream":true`) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":2,\"total_tokens\":4}}\n\n"))
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	dataDir := filepath.Join(t.TempDir(), "dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"llm": {
			"defaultProvider": "openai_compatible",
			"defaultModel": "gpt-test",
			"defaultTimeoutMs": 30000,
			"openaiCompatible": {
				"baseURL": "`+upstream.URL+`/v1",
				"apiKeyEnv": "OPENAI_TEST_KEY",
				"model": "gpt-test"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")
	t.Setenv("OPENAI_TEST_KEY", "secret")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		if err := application.Close(context.Background()); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	authHeader := testAuthHeader(t, application.Auth)

	queryReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"query":"hello"}`))
	queryReq.Header.Set("Authorization", authHeader)
	queryRec := httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for chat query, got %d body=%s", queryRec.Code, queryRec.Body.String())
	}
	var response struct {
		Reply    string `json:"reply"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(queryRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	if response.Reply != "hello world" {
		t.Fatalf("expected hello world reply, got %q", response.Reply)
	}
	if response.Provider != llm.OpenAICompatibleProviderName {
		t.Fatalf("expected provider %s, got %s", llm.OpenAICompatibleProviderName, response.Provider)
	}
	if response.Model != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %s", response.Model)
	}

	streamServer := httptest.NewServer(application.Server.Handler())
	defer streamServer.Close()

	streamReq, err := http.NewRequest(http.MethodPost, streamServer.URL+"/v1/chat/query/stream", strings.NewReader(`{"query":"hello"}`))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	streamReq.Header.Set("Authorization", authHeader)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer streamResp.Body.Close()

	reader := bufio.NewReader(streamResp.Body)
	var chunks []string
	for range 16 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		chunks = append(chunks, line)
		if strings.Contains(strings.Join(chunks, ""), "event: chat.query.completed") {
			break
		}
	}
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "event: chat.query.started") || !strings.Contains(joined, "event: chat.query.delta") || !strings.Contains(joined, "event: chat.query.completed") {
		t.Fatalf("unexpected stream payload %q", joined)
	}
}

func TestRecoverPersistedStateRestoresManagedProviderState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	authState := providers.AuthState{
		ProviderID:    "codex_managed",
		Family:        providers.FamilyCodexCLI,
		AuthMode:      providers.AuthModeLocalCLIBridge,
		Status:        providers.AuthStatusAuthenticated,
		CLIPath:       "/usr/bin/codex",
		CLIAvailable:  true,
		AccountLabel:  "user@example.com",
		Plan:          "pro",
		AuthMethod:    "chatgpt",
		LoginCommand:  []string{"codex", "login"},
		LogoutCommand: []string{"codex", "logout"},
		LastCheckedAt: time.Now().UTC(),
		Metadata: map[string]string{
			"managedProviderId":     "codex_managed",
			"managedProviderAction": "auth_status",
			"sandboxProfileId":      "managed_provider_codex",
			"sandboxDecision":       "allow",
			"enforcementStrength":   "declared_only",
		},
	}
	if err := sqliteStore.UpsertProviderAuthState(ctx, authState); err != nil {
		t.Fatalf("UpsertProviderAuthState returned error: %v", err)
	}
	models := []providers.Model{
		{ProviderID: "codex_managed", ModelID: "gpt-5.4", DisplayName: "GPT-5.4", Default: true, Available: true, Source: "cache", Chat: true, Stream: true, Coding: true},
	}
	if err := sqliteStore.ReplaceProviderModels(ctx, "codex_managed", models); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}
	preference := providers.Preference{
		ProviderID:   "codex_managed",
		DefaultModel: "gpt-5.4",
		UpdatedAt:    time.Now().UTC(),
	}
	if err := sqliteStore.UpsertProviderPreference(ctx, preference); err != nil {
		t.Fatalf("UpsertProviderPreference returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	providerManager := providers.NewManager(config.Config{}, llm.NewDispatcher())

	if err := recoverPersistedState(ctx, config.EnvironmentTest, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, nil, providerManager, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	state, ok := providerManager.GetAuthState("codex_managed")
	if !ok {
		t.Fatal("expected restored provider auth state")
	}
	if state.Status != providers.AuthStatusAuthenticated {
		t.Fatalf("expected restored authenticated state, got %s", state.Status)
	}
	if state.Metadata["managedProviderAction"] != "auth_status" {
		t.Fatalf("expected restored managed-provider metadata, got %+v", state.Metadata)
	}
	persistedModels, ok := providerManager.ListModels("codex_managed")
	if !ok || len(persistedModels) != 1 {
		t.Fatalf("expected restored provider models, got %+v", persistedModels)
	}
	if persistedModels[0].ModelID != "gpt-5.4" {
		t.Fatalf("expected restored model gpt-5.4, got %s", persistedModels[0].ModelID)
	}
	if pref, ok := providerManager.GetPreference("codex_managed"); !ok || pref.DefaultModel != "gpt-5.4" {
		t.Fatalf("expected restored provider preference, got %+v ok=%v", pref, ok)
	}
}

func TestNewRejectsInvalidProviderConfig(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"llm": {
			"defaultProvider": "openai_compatible",
			"openaiCompatible": {
				"baseURL": "not-a-url",
				"apiKey": "secret",
				"model": "gpt-test"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	if _, err := New(); err == nil {
		t.Fatal("expected invalid provider config to fail startup")
	}
}

func testAuthHeader(t *testing.T, manager *auth.Manager) string {
	t.Helper()

	pairing, code, err := manager.StartPairing(auth.StartPairingInput{
		Mode:  auth.PairingModeLocal,
		Label: "test-client",
	})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, _, tokenSecret, err := manager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	return "Bearer " + tokenSecret
}

func writeAppJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func mustAppJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return payload
}
