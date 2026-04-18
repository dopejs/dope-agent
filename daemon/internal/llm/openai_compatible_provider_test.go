package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleProviderComplete(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"hello from upstream"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider returned error: %v", err)
	}

	response, err := provider.Complete(context.Background(), ProviderRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if authHeader != "Bearer secret" {
		t.Fatalf("expected bearer auth header, got %q", authHeader)
	}
	if response.Output != "hello from upstream" {
		t.Fatalf("expected upstream output, got %q", response.Output)
	}
	if response.Usage.TotalTokens != 7 {
		t.Fatalf("expected usage total 7, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenAICompatibleProviderAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider returned error: %v", err)
	}

	_, err = provider.Complete(context.Background(), ProviderRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected auth failure")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Code != "upstream_auth_failed" {
		t.Fatalf("expected upstream_auth_failed, got %s", providerErr.Code)
	}
}

func TestOpenAICompatibleProviderRetryableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"error":{"message":"upstream down"}}`)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider returned error: %v", err)
	}

	_, err = provider.Complete(context.Background(), ProviderRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected upstream failure")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Code != "upstream_unavailable" || !providerErr.Retryable {
		t.Fatalf("expected retryable upstream_unavailable, got %+v", providerErr)
	}
}

func TestOpenAICompatibleProviderStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":2,\"total_tokens\":4}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider returned error: %v", err)
	}

	var deltas []string
	response, err := provider.Stream(context.Background(), ProviderRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}, func(chunk StreamChunk) error {
		deltas = append(deltas, chunk.Delta)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if strings.Join(deltas, "") != "hello world" {
		t.Fatalf("unexpected deltas %q", strings.Join(deltas, ""))
	}
	if response.Output != "hello world" {
		t.Fatalf("expected output hello world, got %q", response.Output)
	}
	if response.Usage.TotalTokens != 4 {
		t.Fatalf("expected usage total 4, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenAICompatibleProviderRespectsContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"late"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = provider.Complete(ctx, ProviderRequest{
		Model:    "gpt-test",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
