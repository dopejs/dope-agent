package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type streamableHTTPTransport struct {
	client *http.Client
}

func NewStreamableHTTPTransport(client *http.Client) Transport {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &streamableHTTPTransport{client: client}
}

func (t *streamableHTTPTransport) Open(ctx context.Context, server Server, _ SessionPipes) (Session, error) {
	endpoint := strings.TrimSpace(server.Endpoint)
	if endpoint == "" {
		return nil, ErrTransportUnavailable
	}
	session := &streamableHTTPSession{
		server:    server,
		client:    t.client,
		done:      make(chan error, 1),
		closeOnce: sync.Once{},
		sessionID: fmt.Sprintf("%s-%d", server.ServerID, time.Now().UTC().UnixNano()),
	}
	if err := session.initialize(ctx); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

type streamableHTTPSession struct {
	server Server
	client *http.Client

	closeOnce sync.Once
	done      chan error
	sessionID string
}

func (s *streamableHTTPSession) ID() string {
	return s.sessionID
}

func (s *streamableHTTPSession) ListTools(ctx context.Context) ([]Tool, error) {
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

func (s *streamableHTTPSession) CallTool(ctx context.Context, toolName string, input any) (map[string]any, error) {
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

func (s *streamableHTTPSession) Close() error {
	s.closeOnce.Do(func() {
		select {
		case s.done <- nil:
		default:
		}
		close(s.done)
	})
	return nil
}

func (s *streamableHTTPSession) Done() <-chan error {
	return s.done
}

func (s *streamableHTTPSession) initialize(ctx context.Context) error {
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
	if err := s.notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("send initialized notification for %s: %w", s.server.ServerID, err)
	}
	return nil
}

func (s *streamableHTTPSession) notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("marshal mcp streamable-http notification: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(s.server.Endpoint), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build mcp streamable-http notification: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send mcp streamable-http notification: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read mcp streamable-http notification response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("mcp streamable-http returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (s *streamableHTTPSession) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal mcp streamable-http payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(s.server.Endpoint), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build mcp streamable-http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call mcp streamable-http endpoint: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mcp streamable-http response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcp streamable-http returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var rpc rpcResponse
	if err := json.Unmarshal(payload, &rpc); err != nil {
		return nil, fmt.Errorf("decode mcp streamable-http response: %w", err)
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("mcp %s failed: %s", method, rpc.Error.Message)
	}
	return rpc.Result, nil
}
