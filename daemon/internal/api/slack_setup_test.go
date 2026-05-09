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
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestConnectorSlackSetupProjectionIsTenantScopedAndRedacted(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_slack_api",
		ConnectorID: "slack-main",
		Kind:        "slack",
		DisplayName: "Slack Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveSlackHostedSetup(context.Background(), store.SlackHostedSetupRecord{
		TenantID:           "ten_slack_api",
		ConnectorID:        "slack-main",
		ConnectorKind:      "slack",
		DisplayName:        "Slack Main",
		Status:             "degraded",
		TerminalState:      "action-required",
		OAuthState:         "grant_valid",
		RoutePolicyState:   "valid",
		WorkspaceBindingID: "slack_workspace_binding_1",
		ReasonCode:         "slack_route_policy_missing",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		WorkspaceBinding: &store.SlackWorkspaceBinding{
			TenantID:           "ten_slack_api",
			ConnectorID:        "slack-main",
			WorkspaceBindingID: "slack_workspace_binding_1",
			WorkspaceID:        "workspace_redacted",
			WorkspaceLabel:     "Workspace Redacted",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
			ValidatedAt:        now,
			RedactionStatus:    "redacted",
		},
	}); err != nil {
		t.Fatalf("SaveSlackHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.SaveSlackRoutePolicy(context.Background(), store.SlackRoutePolicyRecord{
		TenantID:           "ten_slack_api",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "slack_workspace_binding_1",
		SelectedChannels: []store.SlackConversationRouteRecord{{
			ConversationID:       "channel_redacted",
			ConversationType:     "channel",
			SelectedChannelState: "selected",
			ValidationState:      "valid",
			RedactionStatus:      "redacted",
		}},
		AllowedDMUsers:      []string{"user_hash_1"},
		AllowedDMUserGroups: []string{"group_hash_1"},
		MentionGate:         "agent_mention_required",
		ThreadReplyMode:     "channel_mentions_thread_rooted",
		ValidationState:     "valid",
		ValidatedAt:         now,
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveSlackRoutePolicy returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/slack-main/slack-setup", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_slack_api",
		PrincipalID: "prn_slack_api",
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
	if bytes.Contains(rec.Body.Bytes(), []byte(`"workspaceBindingId":"slack_workspace_binding_1","workspaceId"`)) {
		t.Fatalf("workspace binding projection exposed internal binding id inline: %s", rec.Body.String())
	}
	var body slackHostedSetupResource
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TerminalState != "action-required" || body.WorkspaceBinding == nil || body.WorkspaceBinding.WorkspaceID != "workspace_redacted" {
		t.Fatalf("unexpected setup projection: %+v", body)
	}
	if body.RoutePolicy == nil || len(body.RoutePolicy.SelectedChannels) != 1 {
		t.Fatalf("expected route policy projection, got %+v", body.RoutePolicy)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/slack-main/slack-setup", nil)
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

func TestConnectorSlackDiagnosticsProjectionIsTenantSafeAndFresh(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_slack_api",
		ConnectorID: "slack-main",
		Kind:        "slack",
		DisplayName: "Slack Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Now().UTC()
	diagnostic, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		TenantID:           "ten_slack_api",
		ConnectorID:        "slack-main",
		ConnectorAccountID: "workspace_binding_redacted",
		ReasonCode:         connectors.DiagnosticPermissionMissing,
		EvidenceTimestamp:  now.Add(-2 * time.Minute),
		RedactionReliable:  true,
		SafeEvidence:       map[string]string{"stage": "support_inspection", "workspaceId": "workspace_redacted"},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), diagnostic); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/slack-main/diagnostics", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_slack_api",
		PrincipalID: "prn_slack_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("diagnostics status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"reasonCode":"permission_missing"`)) || !bytes.Contains([]byte(body), []byte(`"freshnessState":"fresh"`)) {
		t.Fatalf("diagnostics response missing expected Slack support projection: %s", body)
	}
	if bytes.Contains([]byte(body), []byte("xoxb-")) || bytes.Contains([]byte(body), []byte("secret")) {
		t.Fatalf("diagnostics response leaked unsafe evidence: %s", body)
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/slack-main/diagnostics", nil)
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
		t.Fatalf("cross-tenant diagnostics status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}
