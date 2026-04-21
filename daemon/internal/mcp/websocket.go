package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type websocketTransport struct {
	dialer *websocket.Dialer
}

func NewWebsocketTransport(dialer *websocket.Dialer) Transport {
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	cloned := *dialer
	if cloned.HandshakeTimeout <= 0 {
		cloned.HandshakeTimeout = 15 * time.Second
	}
	return &websocketTransport{dialer: &cloned}
}

func (t *websocketTransport) Open(ctx context.Context, server Server, _ SessionPipes) (Session, error) {
	endpoint := strings.TrimSpace(server.Endpoint)
	if endpoint == "" {
		return nil, ErrTransportUnavailable
	}
	headers := http.Header{}
	for key, value := range server.ResolvedWebsocketHeaders {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		headers.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	dialer := *t.dialer
	if server.WebsocketConfig != nil && len(server.WebsocketConfig.Subprotocols) > 0 {
		dialer.Subprotocols = cloneStrings(server.WebsocketConfig.Subprotocols)
	}
	conn, _, err := dialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		return nil, fmt.Errorf("open mcp websocket transport: %w", err)
	}
	session := &websocketSession{
		server:    server,
		conn:      conn,
		done:      make(chan error, 1),
		pending:   map[string]chan rpcResponse{},
		closeOnce: sync.Once{},
		sessionID: fmt.Sprintf("%s-%d", server.ServerID, time.Now().UTC().UnixNano()),
	}
	go session.readLoop()
	if err := session.initialize(ctx); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

type websocketSession struct {
	server Server
	conn   *websocket.Conn

	mu        sync.Mutex
	closed    bool
	pending   map[string]chan rpcResponse
	requestID uint64
	done      chan error
	closeOnce sync.Once
	sessionID string
}

func (s *websocketSession) ID() string {
	return s.sessionID
}

func (s *websocketSession) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := s.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tools []struct {
			Name        string         `json:"name"`
			Title       string         `json:"title"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode tools/list response: %w", err)
	}
	now := time.Now().UTC()
	tools := make([]Tool, 0, len(payload.Tools))
	for _, item := range payload.Tools {
		tools = append(tools, Tool{
			ServerID:          s.server.ServerID,
			ToolName:          strings.TrimSpace(item.Name),
			Title:             strings.TrimSpace(item.Title),
			Description:       strings.TrimSpace(item.Description),
			SchemaFingerprint: schemaFingerprint(item.InputSchema),
			DiscoveryStatus:   DiscoveryStatusDiscovered,
			LastDiscoveredAt:  &now,
			UpdatedAt:         now,
		})
	}
	return tools, nil
}

func (s *websocketSession) CallTool(ctx context.Context, toolName string, input any) (map[string]any, error) {
	raw, err := s.call(ctx, "tools/call", map[string]any{
		"name":      strings.TrimSpace(toolName),
		"arguments": normalizeToolArguments(input),
	})
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode tools/call response: %w", err)
	}
	return payload, nil
}

func (s *websocketSession) Done() <-chan error {
	return s.done
}

func (s *websocketSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		for id, ch := range s.pending {
			delete(s.pending, id)
			close(ch)
		}
		s.mu.Unlock()
		if s.conn != nil {
			if err := s.conn.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		select {
		case s.done <- nil:
		default:
		}
		close(s.done)
	})
	return closeErr
}

func (s *websocketSession) initialize(ctx context.Context) error {
	if _, err := s.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "dope-daemon",
			"version": "dev",
		},
	}); err != nil {
		return fmt.Errorf("initialize mcp session for %s: %w", s.server.ServerID, err)
	}
	if err := s.notify("notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("send initialized notification for %s: %w", s.server.ServerID, err)
	}
	return nil
}

func (s *websocketSession) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	requestID := fmt.Sprintf("%d", atomic.AddUint64(&s.requestID, 1))
	responseCh := make(chan rpcResponse, 1)

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrTransportClosed
	}
	s.pending[requestID] = responseCh
	s.mu.Unlock()

	if err := s.writeJSON(rpcRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  method,
		Params:  params,
	}); err != nil {
		s.mu.Lock()
		delete(s.pending, requestID)
		s.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pending, requestID)
		s.mu.Unlock()
		return nil, ctx.Err()
	case err := <-s.done:
		if err == nil {
			err = ErrTransportClosed
		}
		return nil, err
	case response, ok := <-responseCh:
		if !ok {
			return nil, ErrTransportClosed
		}
		if response.Error != nil {
			return nil, fmt.Errorf("mcp %s failed: %s", method, response.Error.Message)
		}
		return response.Result, nil
	}
}

func (s *websocketSession) notify(method string, params any) error {
	return s.writeJSON(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (s *websocketSession) writeJSON(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrTransportClosed
	}
	if err := s.conn.WriteJSON(value); err != nil {
		return fmt.Errorf("write mcp websocket payload: %w", err)
	}
	return nil
}

func (s *websocketSession) readLoop() {
	for {
		_, payload, err := s.conn.ReadMessage()
		if err != nil {
			s.finish(err)
			return
		}
		var response rpcResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			s.finish(fmt.Errorf("decode mcp websocket response: %w", err))
			return
		}
		if strings.TrimSpace(response.ID) == "" {
			continue
		}
		s.mu.Lock()
		ch := s.pending[response.ID]
		delete(s.pending, response.ID)
		s.mu.Unlock()
		if ch != nil {
			ch <- response
			close(ch)
		}
	}
}

func (s *websocketSession) finish(err error) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		for id, ch := range s.pending {
			delete(s.pending, id)
			close(ch)
		}
		s.mu.Unlock()
		s.done <- err
		close(s.done)
	})
}
