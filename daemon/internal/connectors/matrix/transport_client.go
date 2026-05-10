package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type AccessTokenProvider interface {
	MatrixAccessToken(ctx context.Context, connectorID string) (string, error)
}

type ClientTransportConfig struct {
	ConnectorID          string
	HomeserverURL        string
	BotAccessToken       string
	AccessTokenSource    AccessTokenProvider
	HTTPClient           *http.Client
	SyncPollInterval     time.Duration
	SyncTimeout          time.Duration
	MaxSyncCycles        int
	SelectedRoomIDs      []string
	AllowedDirectUserIDs []string
}

type ClientTransport struct {
	connectorID          string
	homeserverURL        string
	botAccessToken       string
	accessTokenSource    AccessTokenProvider
	httpClient           *http.Client
	syncPollInterval     time.Duration
	syncTimeout          time.Duration
	maxSyncCycles        int
	selectedRoomIDs      map[string]struct{}
	allowedDirectUserIDs map[string]struct{}
	mu                   sync.Mutex
	botUserID            string
}

type ClientAPIError struct {
	StatusCode int
	ErrCode    string
	Message    string
}

func (e ClientAPIError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.ErrCode) != "" {
		return "matrix client api error: " + e.ErrCode
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("matrix client api status %d", e.StatusCode)
	}
	return "matrix client api error"
}

func NewClientTransport(cfg ClientTransportConfig) (*ClientTransport, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.HomeserverURL), "/")
	if baseURL == "" {
		return nil, errors.New("matrix homeserver URL is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ClientTransport{
		connectorID:          strings.TrimSpace(cfg.ConnectorID),
		homeserverURL:        baseURL,
		botAccessToken:       strings.TrimSpace(cfg.BotAccessToken),
		accessTokenSource:    cfg.AccessTokenSource,
		httpClient:           client,
		syncPollInterval:     cfg.SyncPollInterval,
		syncTimeout:          cfg.SyncTimeout,
		maxSyncCycles:        cfg.MaxSyncCycles,
		selectedRoomIDs:      matrixStringSet(cfg.SelectedRoomIDs),
		allowedDirectUserIDs: matrixStringSet(cfg.AllowedDirectUserIDs),
	}, nil
}

func (t *ClientTransport) Start(ctx context.Context, handle func(context.Context, InboundEvent)) error {
	if t == nil || handle == nil {
		return nil
	}
	token, err := t.accessToken(ctx)
	if err != nil {
		return err
	}
	pollInterval := t.syncPollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	syncTimeout := t.syncTimeout
	if syncTimeout <= 0 {
		syncTimeout = 30 * time.Second
	}
	var since string
	for cycle := 0; ; cycle++ {
		if t.maxSyncCycles > 0 && cycle >= t.maxSyncCycles {
			return nil
		}
		response, err := t.syncOnce(ctx, token, since, syncTimeout)
		if err != nil {
			return err
		}
		since = strings.TrimSpace(response.NextBatch)
		for _, event := range response.inboundEvents(t.connectorID, since, t.homeserverURL, t.selectedRoomIDs, t.allowedDirectUserIDs) {
			handle(ctx, event)
		}
		if t.maxSyncCycles > 0 && cycle+1 >= t.maxSyncCycles {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (t *ClientTransport) SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	if t == nil {
		return imtypes.SentReply{}, errors.New("matrix client transport is not configured")
	}
	roomID := strings.TrimSpace(reply.ChannelID)
	if roomID == "" {
		return imtypes.SentReply{}, errors.New("matrix room id is required")
	}
	token, err := t.accessToken(ctx)
	if err != nil {
		return imtypes.SentReply{}, err
	}
	transactionID := "dope_" + strings.ReplaceAll(strings.TrimSpace(reply.ReplyToExternalMessageID), "$", "")
	if transactionID == "dope_" {
		transactionID = "dope_" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	payload := map[string]string{
		"msgtype": "m.text",
		"body":    strings.TrimSpace(reply.Content),
	}
	var response struct {
		EventID string `json:"event_id"`
		ErrCode string `json:"errcode"`
		Error   string `json:"error"`
	}
	if err := t.call(ctx, http.MethodPut, "/_matrix/client/v3/rooms/"+url.PathEscape(roomID)+"/send/m.room.message/"+url.PathEscape(transactionID), token, payload, &response); err != nil {
		return imtypes.SentReply{}, err
	}
	if strings.TrimSpace(response.EventID) == "" {
		return imtypes.SentReply{}, ClientAPIError{ErrCode: response.ErrCode, Message: response.Error}
	}
	return imtypes.SentReply{ExternalMessageID: strings.TrimSpace(response.EventID)}, nil
}

func (t *ClientTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{MaxMessageLength: 40000}
}

func (t *ClientTransport) Close(context.Context) error { return nil }

func (t *ClientTransport) ValidateHomeserverBinding(ctx context.Context, binding HomeserverBinding) (HomeserverBinding, error) {
	if t == nil {
		return HomeserverBinding{}, errors.New("matrix client transport is not configured")
	}
	token, err := t.accessToken(ctx)
	if err != nil {
		binding.AuthorizationState = AuthorizationMissing
		return binding, err
	}
	var response struct {
		UserID   string `json:"user_id"`
		DeviceID string `json:"device_id"`
	}
	if err := t.call(ctx, http.MethodGet, "/_matrix/client/v3/account/whoami", token, nil, &response); err != nil {
		binding.AuthorizationState, binding.CapabilityState = matrixValidationStatesForError(err)
		return binding, err
	}
	userID := strings.TrimSpace(response.UserID)
	if strings.TrimSpace(binding.BotUserID) != "" && userID != strings.TrimSpace(binding.BotUserID) {
		binding.AuthorizationState = AuthorizationOwnershipMismatch
		binding.CapabilityState = HomeserverCapabilityValid
		return binding, ErrHomeserverBindingInvalid
	}
	if userID != "" {
		binding.BotUserID = userID
	}
	binding.BotDeviceID = strings.TrimSpace(response.DeviceID)
	binding.AuthorizationState = AuthorizationValid
	binding.CapabilityState = HomeserverCapabilityValid
	binding.ValidatedAt = time.Now().UTC()
	binding.RedactionStatus = "redacted"
	if binding.SafeEvidence == nil {
		binding.SafeEvidence = map[string]string{"provider": "matrix_client_server_api"}
	}
	t.mu.Lock()
	t.botUserID = binding.BotUserID
	t.mu.Unlock()
	return binding, nil
}

func (t *ClientTransport) ValidateRoutePolicy(ctx context.Context, policy RoutePolicy) (RoutePolicy, error) {
	if t == nil {
		return RoutePolicy{}, errors.New("matrix client transport is not configured")
	}
	token, err := t.accessToken(ctx)
	if err != nil {
		return policy, err
	}
	policy = NormalizeRoutePolicy(policy, time.Now().UTC())
	botUserID := t.currentBotUserID()
	for i := range policy.SelectedRooms {
		room := &policy.SelectedRooms[i]
		if room.ConversationType != ConversationRoom || strings.TrimSpace(room.ConversationID) == "" {
			continue
		}
		if botUserID == "" {
			room.ValidationState = RoutePolicyBlocked
			room.ReasonCode = "matrix_bot_identity_missing"
			policy.ValidationState = RoutePolicyBlocked
			return policy, ErrHomeserverBindingInvalid
		}
		var response struct {
			Membership string `json:"membership"`
		}
		path := "/_matrix/client/v3/rooms/" + url.PathEscape(room.ConversationID) + "/state/m.room.member/" + url.PathEscape(botUserID)
		if err := t.call(ctx, http.MethodGet, path, token, nil, &response); err != nil {
			room.ValidationState = RoutePolicyBlocked
			room.RoomSelectionState = RoomMissingMembership
			room.ReasonCode = "matrix_room_permission_missing"
			policy.ValidationState = RoutePolicyBlocked
			policy.ReasonCode = room.ReasonCode
			return policy, err
		}
		if response.Membership != "join" {
			room.ValidationState = RoutePolicyBlocked
			room.RoomSelectionState = RoomMissingMembership
			room.ReasonCode = "matrix_room_membership_missing"
			policy.ValidationState = RoutePolicyBlocked
			policy.ReasonCode = room.ReasonCode
			return policy, ErrHomeserverBindingInvalid
		}
		room.ValidationState = RoutePolicyValid
		room.RoomSelectionState = RoomSelected
	}
	policy.ValidationState = RoutePolicyValid
	policy.ValidatedAt = time.Now().UTC()
	policy.RedactionStatus = "redacted"
	if policy.SafeEvidence == nil {
		policy.SafeEvidence = map[string]string{"provider": "matrix_client_server_api"}
	}
	return policy, nil
}

func (t *ClientTransport) accessToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(t.botAccessToken) != "" {
		return strings.TrimSpace(t.botAccessToken), nil
	}
	if t.accessTokenSource == nil {
		return "", errors.New("matrix bot access token is not configured")
	}
	token, err := t.accessTokenSource.MatrixAccessToken(ctx, t.connectorID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", errors.New("matrix bot access token is not configured")
	}
	return strings.TrimSpace(token), nil
}

func (t *ClientTransport) syncOnce(ctx context.Context, token, since string, timeout time.Duration) (matrixSyncResponse, error) {
	values := url.Values{}
	values.Set("timeout", fmt.Sprintf("%d", timeout.Milliseconds()))
	if strings.TrimSpace(since) != "" {
		values.Set("since", strings.TrimSpace(since))
	}
	var response matrixSyncResponse
	err := t.call(ctx, http.MethodGet, "/_matrix/client/v3/sync?"+values.Encode(), token, nil, &response)
	return response, err
}

func (t *ClientTransport) call(ctx context.Context, method, path, token string, payload any, output any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.homeserverURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := t.httpClient.Do(req)
	if err != nil {
		return ClientAPIError{ErrCode: "network_failed", Message: err.Error()}
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(output); err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiError struct {
			ErrCode string `json:"errcode"`
			Error   string `json:"error"`
		}
		encoded, _ := json.Marshal(output)
		_ = json.Unmarshal(encoded, &apiError)
		return ClientAPIError{StatusCode: res.StatusCode, ErrCode: apiError.ErrCode, Message: apiError.Error}
	}
	return nil
}

func (t *ClientTransport) currentBotUserID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.botUserID)
}

type matrixSyncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []matrixSyncEvent `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

type matrixSyncEvent struct {
	Type           string         `json:"type"`
	EventID        string         `json:"event_id"`
	Sender         string         `json:"sender"`
	OriginServerTS int64          `json:"origin_server_ts"`
	Content        map[string]any `json:"content"`
}

func (r matrixSyncResponse) inboundEvents(connectorID, syncBatchID, homeserverURL string, selectedRoomIDs, allowedDirectUserIDs map[string]struct{}) []InboundEvent {
	items := make([]InboundEvent, 0)
	for roomID, room := range r.Rooms.Join {
		for _, event := range room.Timeline.Events {
			messageKind := MessageUnsupported
			text := ""
			if event.Type == "m.room.message" {
				if msgType, _ := event.Content["msgtype"].(string); msgType == "m.text" {
					messageKind = MessageUnencryptedText
					text, _ = event.Content["body"].(string)
				}
			}
			if event.Type == "m.room.encrypted" {
				messageKind = MessageEncryptedUnsupported
			}
			if strings.TrimSpace(event.EventID) == "" {
				continue
			}
			receivedAt := time.Now().UTC()
			if event.OriginServerTS > 0 {
				receivedAt = time.UnixMilli(event.OriginServerTS).UTC()
			}
			conversationType := ConversationRoom
			if _, ok := selectedRoomIDs[strings.TrimSpace(roomID)]; !ok {
				if _, allowed := allowedDirectUserIDs[strings.TrimSpace(event.Sender)]; allowed {
					conversationType = ConversationDirectMessage
				}
			}
			items = append(items, InboundEvent{
				ConnectorID:      connectorID,
				HomeserverID:     matrixHomeserverID(event.Sender, homeserverURL),
				ConversationID:   roomID,
				MatrixEventID:    event.EventID,
				SyncBatchID:      syncBatchID,
				SenderID:         event.Sender,
				ConversationType: conversationType,
				MessageKind:      messageKind,
				Text:             text,
				ReceivedAt:       receivedAt,
			})
		}
	}
	return items
}

func matrixStringSet(values []string) map[string]struct{} {
	items := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items[strings.TrimSpace(value)] = struct{}{}
	}
	return items
}

func matrixHomeserverID(senderID, homeserverURL string) string {
	if _, domain, ok := strings.Cut(strings.TrimSpace(senderID), ":"); ok && strings.TrimSpace(domain) != "" {
		return strings.TrimSpace(domain)
	}
	parsed, err := url.Parse(homeserverURL)
	if err == nil && strings.TrimSpace(parsed.Hostname()) != "" {
		return strings.TrimSpace(parsed.Hostname())
	}
	return ""
}

func matrixValidationStatesForError(err error) (AuthorizationState, HomeserverCapabilityState) {
	var apiError ClientAPIError
	if errors.As(err, &apiError) {
		switch {
		case apiError.StatusCode == http.StatusUnauthorized || apiError.StatusCode == http.StatusForbidden:
			return AuthorizationRevoked, HomeserverCapabilityUnknown
		case apiError.StatusCode == http.StatusTooManyRequests:
			return AuthorizationValid, HomeserverCapabilityRateLimited
		case apiError.StatusCode >= 500:
			return AuthorizationProviderUnavailable, HomeserverCapabilityUnknown
		}
		if apiError.ErrCode == "network_failed" {
			return AuthorizationNetworkFailed, HomeserverCapabilityUnknown
		}
	}
	return AuthorizationUnknown, HomeserverCapabilityUnknown
}
