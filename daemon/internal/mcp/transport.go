package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrTransportUnavailable = errors.New("mcp transport is unavailable")
	ErrTransportClosed      = errors.New("mcp transport is closed")
)

type SessionPipes struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

type Transport interface {
	Open(ctx context.Context, server Server, pipes SessionPipes) (Session, error)
}

type Session interface {
	ListTools(ctx context.Context) ([]Tool, error)
	Close() error
	Done() <-chan error
}

type stdioTransport struct{}

func NewStdioTransport() Transport {
	return &stdioTransport{}
}

func (t *stdioTransport) Open(ctx context.Context, server Server, pipes SessionPipes) (Session, error) {
	if pipes.Stdin == nil || pipes.Stdout == nil {
		return nil, ErrTransportUnavailable
	}
	session := &stdioSession{
		server:    server,
		stdin:     pipes.Stdin,
		stdout:    pipes.Stdout,
		stderr:    pipes.Stderr,
		done:      make(chan error, 1),
		pending:   map[string]chan rpcResponse{},
		closeOnce: sync.Once{},
	}
	go session.readLoop()
	go session.consumeStderr()
	if err := session.initialize(ctx); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

type stdioSession struct {
	server Server

	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu        sync.Mutex
	closed    bool
	pending   map[string]chan rpcResponse
	requestID uint64
	done      chan error
	closeOnce sync.Once
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *stdioSession) initialize(ctx context.Context) error {
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

func (s *stdioSession) ListTools(ctx context.Context) ([]Tool, error) {
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

func (s *stdioSession) Done() <-chan error {
	return s.done
}

func (s *stdioSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		for id, ch := range s.pending {
			delete(s.pending, id)
			close(ch)
		}
		s.mu.Unlock()
		if s.stdin != nil {
			if err := s.stdin.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		if s.stdout != nil {
			if err := s.stdout.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		if s.stderr != nil {
			if err := s.stderr.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		select {
		case s.done <- nil:
		default:
		}
	})
	return closeErr
}

func (s *stdioSession) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	requestID := fmt.Sprintf("%d", atomic.AddUint64(&s.requestID, 1))
	responseCh := make(chan rpcResponse, 1)

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrTransportClosed
	}
	s.pending[requestID] = responseCh
	s.mu.Unlock()

	if err := s.writeMessage(rpcRequest{
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

func (s *stdioSession) notify(method string, params any) error {
	return s.writeMessage(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (s *stdioSession) writeMessage(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal mcp transport payload: %w", err)
	}
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
	if _, err := io.WriteString(s.stdin, message); err != nil {
		return fmt.Errorf("write mcp transport payload: %w", err)
	}
	return nil
}

func (s *stdioSession) readLoop() {
	reader := bufio.NewReader(s.stdout)
	for {
		payload, err := readFramedMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				s.finish(nil)
				return
			}
			s.finish(err)
			return
		}
		var response rpcResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			s.finish(fmt.Errorf("decode mcp transport response: %w", err))
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

func (s *stdioSession) consumeStderr() {
	if s.stderr == nil {
		return
	}
	_, _ = io.Copy(io.Discard, s.stderr)
}

func (s *stdioSession) finish(err error) {
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

func readFramedMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
		if value == line {
			value = strings.TrimSpace(strings.TrimPrefix(line, "content-length:"))
		}
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
			return nil, fmt.Errorf("parse content length: %w", err)
		}
		length = parsed
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func schemaFingerprint(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
