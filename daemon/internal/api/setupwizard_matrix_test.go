package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestMatrixSetupWizardSubmittedSecretPersistsHostedSetup(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/_matrix/client/v3/account/whoami":
			_, _ = w.Write([]byte(`{"user_id":"@bot:example.org","device_id":"DEVICE1"}`))
		case "/_matrix/client/v3/rooms/%21room:example.org/state/m.room.member/@bot:example.org":
			_, _ = w.Write([]byte(`{"membership":"join"}`))
		default:
			t.Fatalf("unexpected Matrix setup validation path: %s", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)

	integration := newMatrixSetupWizardIntegration(sqliteStore, config.MatrixConnectorConfig{
		ConnectorID:          "matrix-main",
		DisplayName:          "Matrix Main",
		HomeserverURL:        server.URL,
		HomeserverID:         "matrix.example.org",
		BotUserID:            "@bot:example.org",
		SelectedRoomIDs:      []string{"!room:example.org"},
		AllowedDirectUserIDs: []string{"@alice:example.org"},
		ConfiguredCommands:   []string{"!dope"},
	})
	service := setupwizard.NewService(setupwizard.ServiceDependencies{
		Store:                   sqliteStore,
		Diagnostics:             setupWizardDiagnosticProbe{Default: setupwizard.DefaultDiagnosticProbe{}, Matrix: integration},
		SubmittedSecretRecorder: integration,
	})
	actor := identity.TenantContext{
		TenantID:    "ten_matrix_setup",
		PrincipalID: "prn_matrix",
		Permissions: []identity.Permission{
			identity.PermissionSecretsManage,
			identity.PermissionIntegrationsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	session, err := service.Start(context.Background(), setupwizard.StartInput{
		TenantContext: actor,
		TargetID:      setupwizard.TargetMatrixConnector,
		SetupStyle:    setupwizard.SetupStyleSubmittedSecret,
		Source:        "wizard",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if _, err := service.SubmitSecret(context.Background(), setupwizard.SubmitSecretInput{
		TenantContext: actor,
		SessionID:     session.SetupSessionID,
		SecretRef:     "matrix/matrix-main/bot_access_token",
		Value:         "matrix-token-do-not-leak",
		DisplayName:   "Matrix bot access token",
		ResourceRefs: []setupwizard.ResourceRef{
			{Kind: "matrix_route_policy_validation", ID: "room:!room:example.org"},
			{Kind: "matrix_route_policy_validation", ID: "direct:@alice:example.org"},
		},
	}); err != nil {
		t.Fatalf("SubmitSecret returned error: %v", err)
	}

	setup, ok, err := sqliteStore.GetMatrixHostedSetup(context.Background(), "ten_matrix_setup", "matrix-main")
	if err != nil || !ok {
		t.Fatalf("GetMatrixHostedSetup ok=%v err=%v", ok, err)
	}
	if setup.TerminalState != "ready" || !setup.DeliveryEligible {
		t.Fatalf("expected ready Matrix setup, got %+v", setup)
	}
	if setup.HomeserverBinding == nil || setup.HomeserverBinding.BotUserID != "@bot:example.org" {
		t.Fatalf("expected Matrix homeserver binding, got %+v", setup)
	}
	if setup.RoutePolicy == nil || len(setup.RoutePolicy.SelectedRooms) != 1 || len(setup.RoutePolicy.AllowedDirectUsers) != 1 {
		t.Fatalf("expected Matrix route policy from setup refs, got %+v", setup.RoutePolicy)
	}
}

func TestMatrixSetupWizardSubmittedSecretRejectsMismatchedBotIdentity(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"@other:example.org","device_id":"DEVICE1"}`))
	}))
	t.Cleanup(server.Close)

	integration := newMatrixSetupWizardIntegration(sqliteStore, config.MatrixConnectorConfig{
		ConnectorID:     "matrix-main",
		DisplayName:     "Matrix Main",
		HomeserverURL:   server.URL,
		HomeserverID:    "matrix.example.org",
		BotUserID:       "@bot:example.org",
		SelectedRoomIDs: []string{"!room:example.org"},
	})
	service := setupwizard.NewService(setupwizard.ServiceDependencies{
		Store:                   sqliteStore,
		Diagnostics:             setupWizardDiagnosticProbe{Default: setupwizard.DefaultDiagnosticProbe{}, Matrix: integration},
		SubmittedSecretRecorder: integration,
	})
	actor := identity.TenantContext{
		TenantID:    "ten_matrix_setup",
		PrincipalID: "prn_matrix",
		Permissions: []identity.Permission{
			identity.PermissionSecretsManage,
			identity.PermissionIntegrationsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	session, err := service.Start(context.Background(), setupwizard.StartInput{
		TenantContext: actor,
		TargetID:      setupwizard.TargetMatrixConnector,
		SetupStyle:    setupwizard.SetupStyleSubmittedSecret,
		Source:        "wizard",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	submitted, err := service.SubmitSecret(context.Background(), setupwizard.SubmitSecretInput{
		TenantContext: actor,
		SessionID:     session.SetupSessionID,
		SecretRef:     "matrix/matrix-main/bot_access_token",
		Value:         "matrix-token-do-not-leak",
		DisplayName:   "Matrix bot access token",
		ResourceRefs:  []setupwizard.ResourceRef{{Kind: "matrix_route_policy_validation", ID: "room:!room:example.org"}},
	})
	if err != nil {
		t.Fatalf("SubmitSecret returned error: %v", err)
	}
	if submitted.State == setupwizard.StateReady {
		t.Fatalf("mismatched Matrix bot identity should not be ready: %+v", submitted)
	}
	setup, ok, err := sqliteStore.GetMatrixHostedSetup(context.Background(), "ten_matrix_setup", "matrix-main")
	if err != nil || !ok {
		t.Fatalf("GetMatrixHostedSetup ok=%v err=%v", ok, err)
	}
	if setup.TerminalState == "ready" || setup.DeliveryEligible {
		t.Fatalf("mismatched Matrix bot identity should not be delivery eligible: %+v", setup)
	}
}
