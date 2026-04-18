package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type testProvider struct {
	name          string
	completeFunc  func(ctx context.Context, request ProviderRequest) (ProviderResponse, error)
	streamFunc    func(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error)
	completeCalls int
	streamCalls   int
}

func (p *testProvider) Name() string { return p.name }

func (p *testProvider) Complete(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
	p.completeCalls++
	return p.completeFunc(ctx, request)
}

func (p *testProvider) Stream(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
	p.streamCalls++
	return p.streamFunc(ctx, request, emit)
}

func TestDispatcherDispatchesSuccessfully(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "test",
		completeFunc: func(_ context.Context, request ProviderRequest) (ProviderResponse, error) {
			return ProviderResponse{
				Output:       "done",
				FinishReason: "stop",
				Usage:        Usage{InputTokens: 3, OutputTokens: 1},
			}, nil
		},
		streamFunc: func(_ context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			return ProviderResponse{}, errors.New("not used")
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider: "test",
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hello world"}},
	}, false)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	final, err := dispatcher.Dispatch(context.Background(), dispatch)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if final.Status != DispatchStatusCompleted {
		t.Fatalf("expected completed dispatch, got %s", final.Status)
	}
	if final.Output != "done" {
		t.Fatalf("expected output done, got %q", final.Output)
	}
	if final.Usage.TotalTokens != 4 {
		t.Fatalf("expected total tokens 4, got %d", final.Usage.TotalTokens)
	}
	if provider.completeCalls != 1 {
		t.Fatalf("expected one provider call, got %d", provider.completeCalls)
	}
}

func TestDispatcherRetriesRetryableFailure(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "retryable",
		completeFunc: func(_ context.Context, request ProviderRequest) (ProviderResponse, error) {
			if request.Attempt == 1 {
				return ProviderResponse{}, &ProviderError{Code: "upstream_unavailable", Message: "upstream unavailable", Retryable: true}
			}
			return ProviderResponse{
				Output:       "recovered",
				FinishReason: "stop",
				Usage:        Usage{InputTokens: 2, OutputTokens: 1},
			}, nil
		},
		streamFunc: func(_ context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			return ProviderResponse{}, errors.New("not used")
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider:   "retryable",
		Model:      "test-model",
		Messages:   []Message{{Role: RoleUser, Content: "retry me"}},
		MaxRetries: 2,
	}, false)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	final, err := dispatcher.Dispatch(context.Background(), dispatch)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if final.AttemptCount != 2 {
		t.Fatalf("expected 2 attempts, got %d", final.AttemptCount)
	}
	if provider.completeCalls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", provider.completeCalls)
	}
}

func TestDispatcherTimesOut(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "slow",
		completeFunc: func(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
			select {
			case <-ctx.Done():
				return ProviderResponse{}, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return ProviderResponse{Output: "too slow"}, nil
			}
		},
		streamFunc: func(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			return ProviderResponse{}, errors.New("not used")
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider:   "slow",
		Model:      "test-model",
		Messages:   []Message{{Role: RoleUser, Content: "timeout"}},
		TimeoutMs:  25,
		MaxRetries: 0,
	}, false)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	final, err := dispatcher.Dispatch(context.Background(), dispatch)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if final.ErrorCode != "timeout" {
		t.Fatalf("expected timeout error code, got %s", final.ErrorCode)
	}
	if final.Status != DispatchStatusFailed {
		t.Fatalf("expected failed status, got %s", final.Status)
	}
}

func TestDispatcherStreamsSuccessfully(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "stream",
		completeFunc: func(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
			return ProviderResponse{}, errors.New("not used")
		},
		streamFunc: func(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			if err := emit(StreamChunk{Delta: "hello"}); err != nil {
				return ProviderResponse{}, err
			}
			if err := emit(StreamChunk{Delta: " world"}); err != nil {
				return ProviderResponse{}, err
			}
			return ProviderResponse{
				Output:       "hello world",
				FinishReason: "stop",
				Usage:        Usage{InputTokens: 2, OutputTokens: 2},
			}, nil
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider: "stream",
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}, true)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	var chunks []string
	final, err := dispatcher.DispatchStream(context.Background(), dispatch, func(chunk StreamChunk) error {
		chunks = append(chunks, chunk.Output)
		return nil
	})
	if err != nil {
		t.Fatalf("DispatchStream returned error: %v", err)
	}
	if final.Status != DispatchStatusCompleted {
		t.Fatalf("expected completed stream dispatch, got %s", final.Status)
	}
	if strings.Join(chunks, "|") != "hello|hello world" {
		t.Fatalf("unexpected stream chunks: %q", strings.Join(chunks, "|"))
	}
}

func TestDispatcherCancelsInterruptedStream(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "interrupt",
		completeFunc: func(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
			return ProviderResponse{}, errors.New("not used")
		},
		streamFunc: func(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			if err := emit(StreamChunk{Delta: "partial"}); err != nil {
				return ProviderResponse{}, err
			}
			return ProviderResponse{}, context.Canceled
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider: "interrupt",
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}, true)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())
	cancel()
	final, err := dispatcher.DispatchStream(context.Background(), dispatch, func(chunk StreamChunk) error {
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if final.Status != DispatchStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", final.Status)
	}
	if final.ErrorCode != "cancelled" {
		t.Fatalf("expected cancelled error code, got %s", final.ErrorCode)
	}
}

func TestDispatcherMarksPartialFailedAfterVisibleStreamOutput(t *testing.T) {
	dispatcher := NewDispatcher()
	provider := &testProvider{
		name: "partial-timeout",
		streamFunc: func(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
			if err := emit(StreamChunk{Delta: "hello"}); err != nil {
				return ProviderResponse{}, err
			}
			return ProviderResponse{Output: "hello"}, &ProviderError{
				Code:      "idle_timeout",
				Message:   "stream stalled",
				Retryable: true,
			}
		},
	}
	dispatcher.RegisterProvider(provider)

	dispatch, err := dispatcher.Prepare(CreateDispatchInput{
		Provider: "partial-timeout",
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}, true)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	final, err := dispatcher.DispatchStream(context.Background(), dispatch, func(chunk StreamChunk) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected partial stream failure")
	}
	if final.Status != DispatchStatusPartialFailed {
		t.Fatalf("expected partial_failed, got %s", final.Status)
	}
	if !final.Partial {
		t.Fatal("expected partial flag to be true")
	}
	if final.Output != "hello" {
		t.Fatalf("expected partial output to be preserved, got %q", final.Output)
	}
	if final.ErrorCode != "idle_timeout" {
		t.Fatalf("expected idle_timeout error code, got %s", final.ErrorCode)
	}
}
