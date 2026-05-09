package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	slackconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/slack"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type slackSetupWizardIntegration struct {
	store   *store.SQLiteStore
	secrets setupwizard.SecretManager
	cfg     config.SlackConnectorConfig
}

func newSlackSetupWizardIntegration(sqliteStore *store.SQLiteStore, secretManager setupwizard.SecretManager, cfg config.SlackConnectorConfig) *slackSetupWizardIntegration {
	return &slackSetupWizardIntegration{store: sqliteStore, secrets: secretManager, cfg: cfg}
}

func (i *slackSetupWizardIntegration) AuthorizationURL(_ context.Context, session setupwizard.SetupSession, input setupwizard.OAuthStartInput, defaultURL string) (string, error) {
	if i == nil || session.TargetID != setupwizard.TargetSlackConnector {
		return defaultURL, nil
	}
	clientID := strings.TrimSpace(i.cfg.OAuthClientID)
	if clientID == "" {
		return "", errors.New("slack oauth client id is not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(i.cfg.OAuthAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(i.cfg.APIBaseURL), "/")
	}
	if baseURL == "" {
		baseURL = "https://slack.com"
	}
	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("scope", "app_mentions:read,channels:history,channels:read,chat:write,groups:history,groups:read,im:history,im:read,usergroups:read,users:read")
	values.Set("state", session.OAuthStateRef)
	if redirectURI := absoluteSlackRedirectURI(input.RedirectRoute); redirectURI != "" {
		values.Set("redirect_uri", redirectURI)
	}
	return baseURL + "/oauth/v2/authorize?" + values.Encode(), nil
}

func (i *slackSetupWizardIntegration) RecordOAuthSetup(ctx context.Context, session setupwizard.SetupSession, input setupwizard.OAuthCallbackInput) error {
	if i == nil || i.store == nil || session.TargetID != setupwizard.TargetSlackConnector {
		return nil
	}
	now := time.Now().UTC()
	connectorID := firstNonEmptyString(i.cfg.ConnectorID, "slack-main")
	oauthResult, err := i.exchangeOAuthCode(ctx, input)
	if err != nil {
		return err
	}
	if strings.TrimSpace(oauthResult.BotToken) != "" && i.secrets != nil {
		if err := i.storeBotToken(ctx, session.TenantID, connectorID, oauthResult.BotToken); err != nil {
			return err
		}
	}
	workspaceID := firstNonEmptyString(i.cfg.WorkspaceID, oauthResult.WorkspaceID, slackWorkspaceIDFromRouteRefs(session.ResourceRefs))
	workspaceBindingID := firstNonEmptyString(i.cfg.WorkspaceBindingID, "slack_workspace_"+connectorID)
	routePolicy := slackconnector.NormalizeRoutePolicy(slackRoutePolicyFromConfig(session.TenantID, connectorID, workspaceBindingID, i.cfg, hasSlackRoutePolicyValidation(session.ResourceRefs), now), now)
	oauthState := slackconnector.OAuthGrantMissing
	if input.Result == setupwizard.OAuthResultCompleted {
		oauthState = slackconnector.OAuthGrantValid
	}
	setup := slackconnector.EvaluateHostedSetup(slackconnector.HostedSetupInput{
		TenantID:    session.TenantID,
		ConnectorID: connectorID,
		DisplayName: firstNonEmptyString(i.cfg.DisplayName, "Slack connector"),
		WorkspaceBinding: slackconnector.WorkspaceBinding{
			TenantID:           session.TenantID,
			ConnectorID:        connectorID,
			WorkspaceBindingID: workspaceBindingID,
			WorkspaceID:        workspaceID,
			WorkspaceLabel:     firstNonEmptyString(input.AccountLabel, oauthResult.WorkspaceLabel),
			InstallationID:     firstNonEmptyString(oauthResult.BotUserID, "slack_installation_"+connectorID),
			OAuthGrantState:    "valid",
			RequiredScopeState: oauthResult.ScopeState,
			ValidatedAt:        now,
			RedactionStatus:    baseconnectors.RedactionStatusRedacted,
			SafeEvidence:       map[string]string{"setupSessionId": session.SetupSessionID, "mode": "hosted_oauth"},
		},
		ExpectedWorkspaceID: i.cfg.WorkspaceID,
		OAuthState:          oauthState,
		RoutePolicy:         routePolicy,
		ProviderAvailable:   true,
		NetworkAvailable:    true,
		StartedAt:           session.CreatedAt,
		ValidatedAt:         now,
	})
	record := slackHostedSetupRecord(setup)
	if err := i.store.SaveSlackHostedSetup(ctx, record); err != nil {
		return err
	}
	if setup.TerminalState != slackconnector.TerminalReady {
		diagnostic, err := slackconnector.BuildDiagnosticState(setup.TenantID, setup.ConnectorID, setup.WorkspaceBindingID, connectorReasonForSlackSetup(setup.ReasonCode), map[string]string{
			"setupSessionId": session.SetupSessionID,
			"stage":          "hosted_oauth",
		}, now)
		if err == nil {
			_ = i.store.SaveConnectorDiagnosticState(ctx, diagnostic)
		}
	}
	return nil
}

type slackOAuthExchangeResult struct {
	WorkspaceID    string
	WorkspaceLabel string
	BotUserID      string
	BotToken       string
	ScopeState     string
}

func (i *slackSetupWizardIntegration) exchangeOAuthCode(ctx context.Context, input setupwizard.OAuthCallbackInput) (slackOAuthExchangeResult, error) {
	result := slackOAuthExchangeResult{ScopeState: "valid"}
	if strings.TrimSpace(input.Code) == "" {
		return result, nil
	}
	clientID := strings.TrimSpace(i.cfg.OAuthClientID)
	clientSecret := strings.TrimSpace(i.cfg.OAuthClientSecret)
	if clientID == "" || clientSecret == "" {
		return slackOAuthExchangeResult{}, errors.New("slack oauth client credentials are not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(i.cfg.OAuthAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(i.cfg.APIBaseURL), "/")
	}
	if baseURL == "" {
		baseURL = "https://slack.com"
	}
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", strings.TrimSpace(input.Code))
	if strings.TrimSpace(input.RedirectURI) != "" {
		form.Set("redirect_uri", strings.TrimSpace(input.RedirectURI))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/oauth.v2.access", strings.NewReader(form.Encode()))
	if err != nil {
		return slackOAuthExchangeResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return slackOAuthExchangeResult{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return slackOAuthExchangeResult{}, slackconnector.WebAPIError{StatusCode: res.StatusCode}
	}
	var payload struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		BotUserID   string `json:"bot_user_id"`
		Team        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return slackOAuthExchangeResult{}, err
	}
	if !payload.OK {
		return slackOAuthExchangeResult{}, slackconnector.WebAPIError{Code: payload.Error}
	}
	scopeState := "valid"
	if !slackScopesContain(payload.Scope, "chat:write") {
		scopeState = "missing"
	}
	return slackOAuthExchangeResult{
		WorkspaceID:    payload.Team.ID,
		WorkspaceLabel: payload.Team.Name,
		BotUserID:      payload.BotUserID,
		BotToken:       payload.AccessToken,
		ScopeState:     scopeState,
	}, nil
}

func (i *slackSetupWizardIntegration) storeBotToken(ctx context.Context, tenantID, connectorID, token string) error {
	ref := firstNonEmptyString(i.cfg.BotTokenSecretRef, "slack/"+strings.TrimSpace(connectorID)+"/bot_token")
	_, err := i.secrets.Create(ctx, secrets.CreateInput{
		TenantID:    tenantID,
		SecretRef:   ref,
		DisplayName: "Slack bot token",
		Value:       token,
		Document: map[string]any{
			"connectorKind": "slack",
			"connectorId":   strings.TrimSpace(connectorID),
			"source":        "hosted_oauth",
		},
	})
	if err == nil {
		return nil
	}
	_, rotateErr := i.secrets.Rotate(ctx, secrets.RotateInput{TenantID: tenantID, SecretRef: ref, Value: token})
	return rotateErr
}

func slackScopesContain(scopes, required string) bool {
	for _, scope := range strings.Split(scopes, ",") {
		if strings.TrimSpace(scope) == required {
			return true
		}
	}
	return false
}

func absoluteSlackRedirectURI(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}
	return parsed.String()
}

func slackRoutePolicyFromConfig(tenantID, connectorID, workspaceBindingID string, cfg config.SlackConnectorConfig, validated bool, now time.Time) slackconnector.RoutePolicy {
	state := slackconnector.RoutePolicyBlocked
	if validated {
		state = slackconnector.RoutePolicyValid
	}
	channels := make([]slackconnector.ConversationRoute, 0, len(cfg.AllowedChannelIDs))
	for _, id := range cfg.AllowedChannelIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		channels = append(channels, slackconnector.ConversationRoute{
			ConversationID:       strings.TrimSpace(id),
			ConversationType:     slackconnector.ConversationChannel,
			SelectedChannelState: slackconnector.SelectedChannelSelected,
			ValidationState:      state,
			RedactionStatus:      baseconnectors.RedactionStatusRedacted,
		})
	}
	return slackconnector.RoutePolicy{
		TenantID:            tenantID,
		ConnectorID:         connectorID,
		WorkspaceBindingID:  workspaceBindingID,
		SelectedChannels:    channels,
		AllowedDMUsers:      append([]string(nil), cfg.AllowedDMUserIDs...),
		AllowedDMUserGroups: append([]string(nil), cfg.AllowedDMUserGroups...),
		ValidationState:     state,
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
	}
}

func hasSlackRoutePolicyValidation(refs []setupwizard.ResourceRef) bool {
	for _, ref := range refs {
		if ref.Kind == "slack_route_policy_validation" && strings.TrimSpace(ref.ID) != "" {
			return true
		}
	}
	return false
}

func slackWorkspaceIDFromRouteRefs(refs []setupwizard.ResourceRef) string {
	for _, ref := range refs {
		if ref.Kind != "slack_route_policy_validation" {
			continue
		}
		parts := strings.Split(strings.TrimSpace(ref.ID), "/")
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func connectorReasonForSlackSetup(reason string) baseconnectors.DiagnosticReasonCode {
	switch strings.TrimSpace(reason) {
	case string(baseconnectors.DiagnosticAuthMissing):
		return baseconnectors.DiagnosticAuthMissing
	case string(baseconnectors.DiagnosticPermissionMissing):
		return baseconnectors.DiagnosticPermissionMissing
	case string(baseconnectors.DiagnosticNetworkFailed):
		return baseconnectors.DiagnosticNetworkFailed
	case string(baseconnectors.DiagnosticProviderUnavailable):
		return baseconnectors.DiagnosticProviderUnavailable
	case string(baseconnectors.DiagnosticBlockedRoute):
		return baseconnectors.DiagnosticBlockedRoute
	default:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	}
}

func slackHostedSetupRecord(setup slackconnector.HostedSetup) store.SlackHostedSetupRecord {
	record := store.SlackHostedSetupRecord{
		TenantID:           setup.TenantID,
		ConnectorID:        setup.ConnectorID,
		ConnectorKind:      setup.ConnectorKind,
		DisplayName:        setup.DisplayName,
		Status:             string(setup.Status),
		TerminalState:      string(setup.TerminalState),
		OAuthState:         string(setup.OAuthState),
		RoutePolicyState:   string(setup.RoutePolicyState),
		DeliveryEligible:   setup.DeliveryEligible,
		WorkspaceBindingID: setup.WorkspaceBindingID,
		ReasonCode:         setup.ReasonCode,
		RedactionStatus:    string(setup.RedactionStatus),
		CreatedAt:          setup.CreatedAt,
		UpdatedAt:          setup.UpdatedAt,
		ValidatedAt:        setup.ValidatedAt,
		RetentionExpiresAt: setup.RetentionExpiresAt,
	}
	if strings.TrimSpace(setup.WorkspaceBinding.WorkspaceID) != "" {
		record.WorkspaceBinding = &store.SlackWorkspaceBinding{
			TenantID:           setup.WorkspaceBinding.TenantID,
			ConnectorID:        setup.WorkspaceBinding.ConnectorID,
			WorkspaceBindingID: setup.WorkspaceBinding.WorkspaceBindingID,
			WorkspaceID:        setup.WorkspaceBinding.WorkspaceID,
			WorkspaceLabel:     setup.WorkspaceBinding.WorkspaceLabel,
			InstallationID:     setup.WorkspaceBinding.InstallationID,
			OAuthGrantState:    setup.WorkspaceBinding.OAuthGrantState,
			RequiredScopeState: setup.WorkspaceBinding.RequiredScopeState,
			ValidatedAt:        setup.WorkspaceBinding.ValidatedAt,
			RedactionStatus:    string(setup.WorkspaceBinding.RedactionStatus),
			SafeEvidence:       setup.WorkspaceBinding.SafeEvidence,
		}
	}
	if strings.TrimSpace(setup.RoutePolicy.ConnectorID) != "" {
		policy := store.SlackRoutePolicyRecord{
			TenantID:            setup.RoutePolicy.TenantID,
			ConnectorID:         setup.RoutePolicy.ConnectorID,
			WorkspaceBindingID:  setup.RoutePolicy.WorkspaceBindingID,
			AllowedDMUsers:      append([]string(nil), setup.RoutePolicy.AllowedDMUsers...),
			AllowedDMUserGroups: append([]string(nil), setup.RoutePolicy.AllowedDMUserGroups...),
			MentionGate:         setup.RoutePolicy.MentionGate,
			ThreadReplyMode:     setup.RoutePolicy.ThreadReplyMode,
			ValidationState:     string(setup.RoutePolicy.ValidationState),
			ReasonCode:          setup.RoutePolicy.ReasonCode,
			ValidatedAt:         setup.RoutePolicy.ValidatedAt,
			RedactionStatus:     string(setup.RoutePolicy.RedactionStatus),
			SafeEvidence:        setup.RoutePolicy.SafeEvidence,
		}
		for _, channel := range setup.RoutePolicy.SelectedChannels {
			policy.SelectedChannels = append(policy.SelectedChannels, store.SlackConversationRouteRecord{
				ConversationID:       channel.ConversationID,
				ConversationType:     string(channel.ConversationType),
				SelectedChannelState: string(channel.SelectedChannelState),
				ValidationState:      string(channel.ValidationState),
				ReasonCode:           channel.ReasonCode,
				RedactionStatus:      string(channel.RedactionStatus),
				SafeEvidence:         channel.SafeEvidence,
			})
		}
		record.RoutePolicy = &policy
	}
	return record
}
