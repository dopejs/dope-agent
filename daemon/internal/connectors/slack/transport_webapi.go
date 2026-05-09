package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type BotTokenProvider interface {
	BotToken(ctx context.Context, connectorID string) (string, error)
}

type WebAPITransportConfig struct {
	ConnectorID   string
	BaseURL       string
	BotToken      string
	TokenProvider BotTokenProvider
	HTTPClient    *http.Client
}

type WebAPITransport struct {
	connectorID   string
	baseURL       string
	botToken      string
	tokenProvider BotTokenProvider
	httpClient    *http.Client
}

type WebAPIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e WebAPIError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Code) != "" {
		return "slack web api error: " + e.Code
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("slack web api status %d", e.StatusCode)
	}
	return "slack web api error"
}

func (e WebAPIError) ErrorClass() string {
	code := strings.TrimSpace(e.Code)
	switch code {
	case "invalid_auth", "not_authed", "token_revoked", "account_inactive":
		return string(baseconnectors.DiagnosticAuthMissing)
	case "missing_scope", "not_in_channel", "channel_not_found", "is_archived", "restricted_action":
		return string(baseconnectors.DiagnosticPermissionMissing)
	case "ratelimited", "rate_limited":
		return string(baseconnectors.DiagnosticRateLimited)
	case "network_failed":
		return string(baseconnectors.DiagnosticNetworkFailed)
	}
	switch {
	case e.StatusCode == http.StatusTooManyRequests:
		return string(baseconnectors.DiagnosticRateLimited)
	case e.StatusCode >= 500:
		return string(baseconnectors.DiagnosticProviderUnavailable)
	case e.StatusCode > 0:
		return string(baseconnectors.DiagnosticProviderUnavailable)
	default:
		return string(baseconnectors.DiagnosticUnknownConnectorFailure)
	}
}

func NewWebAPITransport(cfg WebAPITransportConfig) *WebAPITransport {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://slack.com"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebAPITransport{
		connectorID:   strings.TrimSpace(cfg.ConnectorID),
		baseURL:       baseURL,
		botToken:      strings.TrimSpace(cfg.BotToken),
		tokenProvider: cfg.TokenProvider,
		httpClient:    client,
	}
}

func (t *WebAPITransport) Start(context.Context, func(context.Context, InboundEvent)) error {
	return nil
}

func (t *WebAPITransport) SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	if t == nil {
		return imtypes.SentReply{}, errors.New("slack web api transport is not configured")
	}
	if strings.TrimSpace(reply.ChannelID) == "" {
		return imtypes.SentReply{}, errors.New("slack channel id is required")
	}
	payload := map[string]string{
		"channel": strings.TrimSpace(reply.ChannelID),
		"text":    strings.TrimSpace(reply.Content),
	}
	if strings.TrimSpace(reply.ReplyToExternalMessageID) != "" {
		payload["thread_ts"] = strings.TrimSpace(reply.ReplyToExternalMessageID)
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if err := t.post(ctx, "chat.postMessage", payload, &response); err != nil {
		return imtypes.SentReply{}, err
	}
	if !response.OK {
		return imtypes.SentReply{}, WebAPIError{Code: response.Error}
	}
	return imtypes.SentReply{ExternalMessageID: strings.TrimSpace(response.TS)}, nil
}

func (t *WebAPITransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{MaxMessageLength: 40000}
}

func (t *WebAPITransport) Close(context.Context) error { return nil }

func (t *WebAPITransport) ValidateInstallation(ctx context.Context, binding WorkspaceBinding) (WorkspaceBinding, error) {
	if t == nil {
		return WorkspaceBinding{}, errors.New("slack web api transport is not configured")
	}
	var response struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		TeamID string `json:"team_id"`
		Team   string `json:"team"`
		BotID  string `json:"bot_id"`
		UserID string `json:"user_id"`
	}
	if err := t.post(ctx, "auth.test", map[string]string{}, &response); err != nil {
		return WorkspaceBinding{}, err
	}
	if !response.OK {
		return WorkspaceBinding{}, WebAPIError{Code: response.Error}
	}
	binding.WorkspaceID = firstNonEmpty(binding.WorkspaceID, response.TeamID)
	binding.WorkspaceLabel = firstNonEmpty(binding.WorkspaceLabel, response.Team)
	binding.InstallationID = firstNonEmpty(binding.InstallationID, response.BotID, response.UserID)
	binding.OAuthGrantState = "valid"
	binding.RequiredScopeState = "valid"
	if binding.ValidatedAt.IsZero() {
		binding.ValidatedAt = time.Now().UTC()
	}
	if binding.RedactionStatus == "" {
		binding.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	return binding, nil
}

func (t *WebAPITransport) ValidateRoutePolicy(ctx context.Context, policy RoutePolicy) (RoutePolicy, error) {
	policy = NormalizeRoutePolicy(policy, time.Now().UTC())
	for _, channel := range policy.SelectedChannels {
		if channel.ConversationType != ConversationChannel || strings.TrimSpace(channel.ConversationID) == "" {
			continue
		}
		var response struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := t.post(ctx, "conversations.info", map[string]string{"channel": channel.ConversationID}, &response); err != nil {
			return RoutePolicy{}, err
		}
		if !response.OK {
			return RoutePolicy{}, WebAPIError{Code: response.Error}
		}
	}
	policy.ValidationState = RoutePolicyValid
	return policy, nil
}

func (t *WebAPITransport) post(ctx context.Context, method string, payload any, out any) error {
	token, err := t.resolveToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/api/"+strings.TrimSpace(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := t.httpClient.Do(req)
	if err != nil {
		return WebAPIError{Code: "network_failed", Message: err.Error()}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return WebAPIError{StatusCode: res.StatusCode}
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func (t *WebAPITransport) resolveToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(t.botToken) != "" {
		return strings.TrimSpace(t.botToken), nil
	}
	if t.tokenProvider != nil {
		token, err := t.tokenProvider.BotToken(ctx, t.connectorID)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token), nil
		}
	}
	return "", WebAPIError{Code: "not_authed"}
}
