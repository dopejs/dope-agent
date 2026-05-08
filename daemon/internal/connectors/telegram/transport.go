package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type Transport interface {
	Start(ctx context.Context, handle func(context.Context, InboundUpdate)) error
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
	ReplyCapabilities() imtypes.ReplyCapabilities
	Close(ctx context.Context) error
}

type BotAPITransportConfig struct {
	ConnectorID  string
	BotToken     string
	BotUsername  string
	BaseURL      string
	HTTPClient   *http.Client
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type BotAPITransport struct {
	cfg        BotAPITransportConfig
	httpClient *http.Client
	baseURL    string
	cancel     context.CancelFunc
	mu         sync.Mutex
}

func NewBotAPITransport(cfg BotAPITransportConfig) (*BotAPITransport, error) {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 35 * time.Second}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 25 * time.Second
	}
	return &BotAPITransport{cfg: cfg, httpClient: client, baseURL: baseURL}, nil
}

func (t *BotAPITransport) ValidateCredential(ctx context.Context) (AccountBinding, error) {
	var response telegramAPIResponse[telegramUser]
	if err := t.call(ctx, http.MethodGet, "getMe", nil, &response); err != nil {
		return AccountBinding{}, err
	}
	if !response.OK {
		return AccountBinding{}, telegramAPIError{statusCode: response.ErrorCode, description: response.Description}
	}
	accountID := "telegram_bot_" + strconv.FormatInt(response.Result.ID, 10)
	label := strings.TrimSpace(response.Result.Username)
	if label == "" {
		label = strings.TrimSpace(response.Result.FirstName)
	}
	return AccountBinding{
		ConnectorID:          t.cfg.ConnectorID,
		ConnectorAccountID:   accountID,
		ProviderAccountLabel: label,
		PermissionState:      PermissionValid,
		ValidatedAt:          time.Now().UTC(),
		RedactionStatus:      "redacted",
		SafeEvidence: map[string]string{
			"provider": "telegram_bot_api",
		},
	}, nil
}

func (t *BotAPITransport) Start(ctx context.Context, handle func(context.Context, InboundUpdate)) error {
	if handle == nil {
		return nil
	}
	pollCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.cancel = cancel
	t.mu.Unlock()
	go t.poll(pollCtx, handle)
	return nil
}

func (t *BotAPITransport) SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	payload := map[string]any{
		"chat_id": reply.ChannelID,
		"text":    reply.Content,
	}
	if strings.TrimSpace(reply.ReplyToExternalMessageID) != "" {
		payload["reply_to_message_id"] = reply.ReplyToExternalMessageID
	}
	var response telegramAPIResponse[telegramMessage]
	if err := t.call(ctx, http.MethodPost, "sendMessage", payload, &response); err != nil {
		return imtypes.SentReply{}, err
	}
	if !response.OK {
		return imtypes.SentReply{}, telegramAPIError{statusCode: response.ErrorCode, description: response.Description}
	}
	return imtypes.SentReply{ExternalMessageID: strconv.FormatInt(response.Result.MessageID, 10)}, nil
}

func (t *BotAPITransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{SupportsThinking: false, SupportsStreaming: false, MaxMessageLength: 4096}
}

func (t *BotAPITransport) ConnectorKind() string {
	return "telegram"
}

func (t *BotAPITransport) Close(context.Context) error {
	t.mu.Lock()
	cancel := t.cancel
	t.cancel = nil
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (t *BotAPITransport) poll(ctx context.Context, handle func(context.Context, InboundUpdate)) {
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		payload := map[string]any{
			"timeout": int(t.cfg.PollTimeout / time.Second),
		}
		if offset > 0 {
			payload["offset"] = offset
		}
		var response telegramAPIResponse[[]telegramUpdate]
		err := t.call(ctx, http.MethodPost, "getUpdates", payload, &response)
		if err == nil && response.OK {
			for _, update := range response.Result {
				if update.UpdateID >= offset {
					offset = update.UpdateID + 1
				}
				if inbound, ok := inboundFromTelegramUpdate(update); ok {
					handle(ctx, inbound)
				}
			}
		}
		timer := time.NewTimer(t.cfg.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (t *BotAPITransport) call(ctx context.Context, method, apiMethod string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		document, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(document)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.methodURL(apiMethod), body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return telegramAPIError{class: "network_failed", description: err.Error()}
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return telegramAPIError{class: "provider_unavailable", statusCode: resp.StatusCode, description: err.Error()}
	}
	if resp.StatusCode >= 400 {
		return telegramAPIError{statusCode: resp.StatusCode}
	}
	return nil
}

func (t *BotAPITransport) methodURL(method string) string {
	return t.baseURL + "/bot" + strings.TrimSpace(t.cfg.BotToken) + "/" + method
}

type telegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type telegramUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int64          `json:"message_id"`
	From      telegramUser   `json:"from"`
	Chat      telegramChat   `json:"chat"`
	Text      string         `json:"text"`
	Voice     map[string]any `json:"voice"`
	Document  map[string]any `json:"document"`
	Photo     []any          `json:"photo"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type telegramAPIError struct {
	class       string
	statusCode  int
	description string
}

func (e telegramAPIError) Error() string {
	if e.description != "" {
		return fmt.Sprintf("telegram bot api error: %s", e.description)
	}
	return fmt.Sprintf("telegram bot api error: status %d", e.statusCode)
}

func (e telegramAPIError) ErrorClass() string {
	if e.class != "" {
		return e.class
	}
	switch e.statusCode {
	case http.StatusUnauthorized:
		return "auth_error"
	case http.StatusForbidden:
		return "permission_missing"
	case http.StatusTooManyRequests:
		return "rate_limited"
	}
	if e.statusCode >= 500 {
		return "provider_unavailable"
	}
	return "unknown_connector_failure"
}

func inboundFromTelegramUpdate(update telegramUpdate) (InboundUpdate, bool) {
	if update.Message.MessageID == 0 || update.Message.Chat.ID == 0 {
		return InboundUpdate{}, false
	}
	conversation := ConversationDirect
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		conversation = ConversationGroup
	}
	unsupported := ""
	switch {
	case update.Message.Voice != nil:
		unsupported = "voice"
	case update.Message.Document != nil:
		unsupported = "attachment"
	case len(update.Message.Photo) > 0:
		unsupported = "attachment"
	}
	text := strings.TrimSpace(update.Message.Text)
	return InboundUpdate{
		UpdateID:           strconv.FormatInt(update.UpdateID, 10),
		MessageID:          strconv.FormatInt(update.Message.MessageID, 10),
		ChatID:             strconv.FormatInt(update.Message.Chat.ID, 10),
		SenderID:           strconv.FormatInt(update.Message.From.ID, 10),
		Text:               text,
		ConversationType:   conversation,
		Command:            strings.HasPrefix(text, "/"),
		UnsupportedSurface: unsupported,
		ReceivedAt:         time.Now().UTC(),
	}, true
}

type FakeTransport struct {
	mu            sync.Mutex
	handler       func(context.Context, InboundUpdate)
	replies       []imtypes.OutboundReply
	routeOutcomes []RouteDecision
	failSend      error
}

func NewFakeTransport() *FakeTransport {
	return &FakeTransport{}
}

func (t *FakeTransport) Start(_ context.Context, handle func(context.Context, InboundUpdate)) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handle
	return nil
}

func (t *FakeTransport) Emit(ctx context.Context, update InboundUpdate) {
	t.mu.Lock()
	handler := t.handler
	t.mu.Unlock()
	if handler != nil {
		handler(ctx, update)
	}
}

func (t *FakeTransport) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failSend != nil {
		return imtypes.SentReply{}, t.failSend
	}
	t.replies = append(t.replies, reply)
	return imtypes.SentReply{ExternalMessageID: fmt.Sprintf("telegram_reply_%d", len(t.replies))}, nil
}

func (t *FakeTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{SupportsThinking: false, SupportsStreaming: false, MaxMessageLength: 4096}
}

func (t *FakeTransport) ConnectorKind() string {
	return "telegram"
}

func (t *FakeTransport) Close(context.Context) error {
	return nil
}

func (t *FakeTransport) RecordRouteOutcome(decision RouteDecision) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routeOutcomes = append(t.routeOutcomes, decision)
}

func (t *FakeTransport) LastRouteOutcome() RouteDecision {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.routeOutcomes) == 0 {
		return RouteDecision{}
	}
	return t.routeOutcomes[len(t.routeOutcomes)-1]
}

func NormalizeCommandText(text, botUsername string) (string, bool, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false, false
	}
	command := strings.HasPrefix(trimmed, "/")
	mentioned := false
	if botUsername != "" {
		at := "@" + strings.TrimPrefix(botUsername, "@")
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(at)) {
			mentioned = true
			trimmed = strings.TrimSpace(strings.ReplaceAll(trimmed, at, ""))
		}
	}
	return trimmed, mentioned, command
}
