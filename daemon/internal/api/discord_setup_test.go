package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestConnectorDiscordSetupProjectionIsTenantScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_discord_api",
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveDiscordHostedSetup(context.Background(), store.DiscordHostedSetupRecord{
		TenantID:           "ten_discord_api",
		ConnectorID:        "discord-main",
		ConnectorKind:      "discord",
		DisplayName:        "Discord Main",
		Status:             "degraded",
		ReadinessState:     "degraded_needs_repair",
		CredentialState:    "valid",
		RespondInDM:        true,
		RequireMention:     true,
		DeliveryMode:       "gateway",
		ReasonCode:         "missing_explicit_destination",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveDiscordHostedSetup returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/discord-main/discord-setup", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_discord_api",
		PrincipalID: "prn_discord_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body store.DiscordHostedSetupRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ReadinessState != "degraded_needs_repair" || body.HostedReady {
		t.Fatalf("unexpected setup projection: %+v", body)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/discord-main/discord-setup", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}

func TestConnectorDiscordSmokeEvidenceIsTenantScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_discord_api",
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveDiscordSmokeEvidence(context.Background(), store.DiscordSmokeEvidenceRecord{
		SmokeEvidenceID:    "discord_smoke_api",
		TenantID:           "ten_discord_api",
		ConnectorID:        "discord-main",
		Status:             "skipped",
		CredentialMode:     "unavailable",
		Owner:              "operator",
		Reason:             "safe_credentials_unavailable",
		RemainingRisk:      "No live Discord hosted smoke was run in this release validation.",
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"policy": "structured_skip"},
	}); err != nil {
		t.Fatalf("SaveDiscordSmokeEvidence returned error: %v", err)
	}
	if err := sqliteStore.SaveConnectorConformanceResult(context.Background(), connectors.ConformanceResult{
		ConformanceResultID: "conf_discord_api_tenant_ownership",
		TenantID:            "ten_discord_api",
		ConnectorKind:       "discord",
		ConnectorID:         "discord-main",
		ScenarioID:          "discord_hosted_setup",
		Area:                "tenant_ownership",
		Result:              connectors.ConformanceResultPass,
		RedactionStatus:     connectors.RedactionStatusRedacted,
		EvidenceTimestamp:   now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveConnectorConformanceResult returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/discord-main/discord-smoke", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_discord_api",
		PrincipalID: "prn_discord_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body store.DiscordSmokeEvidenceRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "skipped" || body.CredentialMode != "unavailable" || body.Reason != "safe_credentials_unavailable" {
		t.Fatalf("unexpected smoke evidence: %+v", body)
	}
	liveValidationReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/discord-smoke?connectorId=discord-main", nil)
	liveValidationReq = liveValidationReq.WithContext(withTenantContext(liveValidationReq.Context(), identity.TenantContext{
		TenantID:    "ten_discord_api",
		PrincipalID: "prn_discord_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	liveValidationRec := httptest.NewRecorder()
	handleLiveValidationRoutes(livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore}), nil, sqliteStore, liveValidationRec, liveValidationReq)
	if liveValidationRec.Code != http.StatusOK {
		t.Fatalf("live validation smoke status=%d body=%s", liveValidationRec.Code, liveValidationRec.Body.String())
	}
	conformanceReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/discord-conformance?connectorId=discord-main", nil)
	conformanceReq = conformanceReq.WithContext(withTenantContext(conformanceReq.Context(), identity.TenantContext{
		TenantID:    "ten_discord_api",
		PrincipalID: "prn_discord_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	conformanceRec := httptest.NewRecorder()
	handleLiveValidationRoutes(livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore}), nil, sqliteStore, conformanceRec, conformanceReq)
	if conformanceRec.Code != http.StatusOK || !bytes.Contains(conformanceRec.Body.Bytes(), []byte(`"tenant_ownership"`)) {
		t.Fatalf("live validation conformance status=%d body=%s", conformanceRec.Code, conformanceRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/discord-main/discord-smoke", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}

func TestConnectorTelegramSetupProjectionIsTenantScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_telegram_api",
		ConnectorID: "telegram-main",
		Kind:        "telegram",
		DisplayName: "Telegram Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveTelegramHostedSetup(context.Background(), store.TelegramHostedSetupRecord{
		TenantID:           "ten_telegram_api",
		ConnectorID:        "telegram-main",
		ConnectorKind:      "telegram",
		DisplayName:        "Telegram Main",
		Status:             "degraded",
		TerminalState:      "action-required",
		CredentialState:    "valid",
		AllowmentState:     "none",
		GroupBehavior:      "mention_or_command_required",
		ReasonCode:         "telegram_allowment_missing",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveTelegramHostedSetup returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/telegram-main/telegram-setup", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body store.TelegramHostedSetupRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TerminalState != "action-required" || body.HostedReady {
		t.Fatalf("unexpected setup projection: %+v", body)
	}
	diagnostic, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		TenantID:           "ten_telegram_api",
		ConnectorID:        "telegram-main",
		ConnectorAccountID: "telegram_bot_42",
		ReasonCode:         connectors.DiagnosticBlockedRoute,
		EvidenceTimestamp:  now,
		RedactionReliable:  true,
		SafeEvidence:       map[string]string{"stage": "setup"},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), diagnostic); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState returned error: %v", err)
	}
	diagReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/telegram-main/diagnostics", nil)
	diagReq = diagReq.WithContext(withTenantContext(diagReq.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	diagRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, diagRec, diagReq)
	if diagRec.Code != http.StatusOK || !bytes.Contains(diagRec.Body.Bytes(), []byte(`"reasonCode":"blocked_route"`)) {
		t.Fatalf("diagnostics status=%d body=%s", diagRec.Code, diagRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/telegram-main/telegram-setup", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}

func TestConnectorTelegramSmokeEvidenceIsTenantScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_telegram_api",
		ConnectorID: "telegram-main",
		Kind:        "telegram",
		DisplayName: "Telegram Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveTelegramSmokeEvidence(context.Background(), store.TelegramSmokeEvidenceRecord{
		SmokeEvidenceID:    "telegram_smoke_api",
		TenantID:           "ten_telegram_api",
		ConnectorID:        "telegram-main",
		Status:             "skipped",
		CredentialMode:     "unavailable",
		Owner:              "operator",
		Reason:             "safe_credentials_unavailable",
		RemainingRisk:      "No live Telegram hosted smoke was run in this release validation.",
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"policy": "structured_skip"},
	}); err != nil {
		t.Fatalf("SaveTelegramSmokeEvidence returned error: %v", err)
	}
	if err := sqliteStore.SaveConnectorConformanceResult(context.Background(), connectors.ConformanceResult{
		ConformanceResultID: "conf_telegram_api_tenant_ownership",
		TenantID:            "ten_telegram_api",
		ConnectorKind:       "telegram",
		ConnectorID:         "telegram-main",
		ScenarioID:          "telegram_hosted_setup",
		Area:                "tenant_ownership",
		Result:              connectors.ConformanceResultPass,
		RedactionStatus:     connectors.RedactionStatusRedacted,
		EvidenceTimestamp:   now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveConnectorConformanceResult returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/telegram-main/telegram-smoke", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body store.TelegramSmokeEvidenceRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "skipped" || body.Reason != "safe_credentials_unavailable" {
		t.Fatalf("unexpected smoke evidence: %+v", body)
	}

	liveValidationReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/telegram-smoke?connectorId=telegram-main", nil)
	liveValidationReq = liveValidationReq.WithContext(withTenantContext(liveValidationReq.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	liveValidationRec := httptest.NewRecorder()
	handleLiveValidationRoutes(livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore}), nil, sqliteStore, liveValidationRec, liveValidationReq)
	if liveValidationRec.Code != http.StatusOK {
		t.Fatalf("live validation smoke status=%d body=%s", liveValidationRec.Code, liveValidationRec.Body.String())
	}
	recordSmokeReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/telegram-smoke", bytes.NewBufferString(`{"connectorId":"telegram-main","status":"skipped","credentialMode":"unavailable","owner":"operator","reason":"safe_credentials_unavailable","remainingRisk":"No safe live Telegram credential was available.","safeEvidence":{"policy":"structured_skip"}}`))
	recordSmokeReq = recordSmokeReq.WithContext(withTenantContext(recordSmokeReq.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionLiveValidationExecute,
			identity.PermissionCredentialsInspect,
		},
	}))
	recordSmokeRec := httptest.NewRecorder()
	handleLiveValidationRoutes(livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore}), nil, sqliteStore, recordSmokeRec, recordSmokeReq)
	if recordSmokeRec.Code != http.StatusCreated || !bytes.Contains(recordSmokeRec.Body.Bytes(), []byte(`"policy":"structured_skip"`)) {
		t.Fatalf("record smoke status=%d body=%s", recordSmokeRec.Code, recordSmokeRec.Body.String())
	}
	conformanceReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/telegram-conformance?connectorId=telegram-main", nil)
	conformanceReq = conformanceReq.WithContext(withTenantContext(conformanceReq.Context(), identity.TenantContext{
		TenantID:    "ten_telegram_api",
		PrincipalID: "prn_telegram_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	conformanceRec := httptest.NewRecorder()
	handleLiveValidationRoutes(livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore}), nil, sqliteStore, conformanceRec, conformanceReq)
	if conformanceRec.Code != http.StatusOK || !bytes.Contains(conformanceRec.Body.Bytes(), []byte(`"tenant_ownership"`)) {
		t.Fatalf("live validation conformance status=%d body=%s", conformanceRec.Code, conformanceRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/telegram-main/telegram-smoke", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}
