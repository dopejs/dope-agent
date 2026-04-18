package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const OpenAICompatibleProviderName = "openai_compatible"

type OpenAICompatibleProviderConfig struct {
	BaseURL                   string
	APIKey                    string
	DefaultModel              string
	RequestTimeoutMs          int
	StreamFirstChunkTimeoutMs int
	StreamIdleTimeoutMs       int
	StreamMaxDurationMs       int
	HTTPClient                *http.Client
}

type OpenAICompatibleProvider struct {
	baseURL                   string
	apiKey                    string
	defaultModel              string
	requestTimeoutMs          int
	streamFirstChunkTimeoutMs int
	streamIdleTimeoutMs       int
	streamMaxDurationMs       int
	httpClient                *http.Client
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleProviderConfig) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider base URL is required")
	}
	chatURL, err := NormalizeOpenAICompatibleRequestURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("openai-compatible provider API key is required")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &OpenAICompatibleProvider{
		baseURL:                   chatURL,
		apiKey:                    strings.TrimSpace(cfg.APIKey),
		defaultModel:              strings.TrimSpace(cfg.DefaultModel),
		requestTimeoutMs:          cfg.RequestTimeoutMs,
		streamFirstChunkTimeoutMs: cfg.StreamFirstChunkTimeoutMs,
		streamIdleTimeoutMs:       cfg.StreamIdleTimeoutMs,
		streamMaxDurationMs:       cfg.StreamMaxDurationMs,
		httpClient:                httpClient,
	}, nil
}

func (p *OpenAICompatibleProvider) Name() string {
	return OpenAICompatibleProviderName
}

func (p *OpenAICompatibleProvider) Complete(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
	response, err := p.doChatCompletion(ctx, request, false, nil)
	if err != nil {
		return ProviderResponse{}, err
	}
	return response, nil
}

func (p *OpenAICompatibleProvider) Stream(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
	response, err := p.doChatCompletion(ctx, request, true, emit)
	if err != nil {
		return response, err
	}
	return response, nil
}

func (p *OpenAICompatibleProvider) doChatCompletion(ctx context.Context, request ProviderRequest, stream bool, emit StreamEmitter) (ProviderResponse, error) {
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return ProviderResponse{}, &ProviderError{
			Code:      "upstream_invalid_request",
			Message:   "model is required",
			Retryable: false,
		}
	}

	body := openAICompatibleRequest{
		Model:    model,
		Messages: make([]openAICompatibleMessage, 0, len(request.Messages)),
		Stream:   stream,
	}
	if stream {
		body.StreamOptions = &openAICompatibleStreamOptions{IncludeUsage: true}
	}
	for _, message := range request.Messages {
		body.Messages = append(body.Messages, openAICompatibleMessage{
			Role:    string(message.Role),
			Content: message.Content,
		})
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("marshal openai-compatible request: %w", err)
	}

	requestCtx := ctx
	cancelCause := func(error) {}
	stopRequestTimeout := func() {}
	if stream {
		var streamCancel context.CancelCauseFunc
		requestCtx, streamCancel = context.WithCancelCause(ctx)
		cancelCause = streamCancel
		stopRequestTimeout = startStreamTimer(
			p.effectiveRequestTimeoutMs(request),
			func() error {
				return &ProviderError{Code: "connect_timeout", Message: "openai-compatible request timed out before stream started", Retryable: true}
			},
			cancelCause,
		)
	}

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, p.baseURL, bytes.NewReader(payload))
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("build openai-compatible request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		stopRequestTimeout()
		if causeErr := classifyOpenAIContextCause(requestCtx); causeErr != nil {
			return ProviderResponse{}, causeErr
		}
		return ProviderResponse{}, classifyOpenAITransportError(err)
	}
	defer resp.Body.Close()
	stopRequestTimeout()

	if resp.StatusCode >= http.StatusBadRequest {
		return ProviderResponse{}, decodeOpenAIErrorResponse(resp)
	}

	if stream {
		return decodeOpenAIStreamResponse(requestCtx, cancelCause, resp.Body, emit, openAIStreamTimeouts{
			firstChunkTimeoutMs: p.effectiveStreamFirstChunkTimeoutMs(request),
			idleTimeoutMs:       p.effectiveStreamIdleTimeoutMs(request),
			maxDurationMs:       p.effectiveStreamMaxDurationMs(request),
		})
	}
	return decodeOpenAICompletionResponse(resp.Body)
}

func (p *OpenAICompatibleProvider) effectiveRequestTimeoutMs(request ProviderRequest) int {
	if request.TimeoutMs > 0 {
		return request.TimeoutMs
	}
	return p.requestTimeoutMs
}

func (p *OpenAICompatibleProvider) effectiveStreamFirstChunkTimeoutMs(request ProviderRequest) int {
	if request.StreamFirstChunkTimeoutMs > 0 {
		return request.StreamFirstChunkTimeoutMs
	}
	if p.streamFirstChunkTimeoutMs > 0 {
		return p.streamFirstChunkTimeoutMs
	}
	return p.effectiveRequestTimeoutMs(request)
}

func (p *OpenAICompatibleProvider) effectiveStreamIdleTimeoutMs(request ProviderRequest) int {
	if request.StreamIdleTimeoutMs > 0 {
		return request.StreamIdleTimeoutMs
	}
	if p.streamIdleTimeoutMs > 0 {
		return p.streamIdleTimeoutMs
	}
	return p.effectiveStreamFirstChunkTimeoutMs(request)
}

func (p *OpenAICompatibleProvider) effectiveStreamMaxDurationMs(request ProviderRequest) int {
	if request.StreamMaxDurationMs > 0 {
		return request.StreamMaxDurationMs
	}
	return p.streamMaxDurationMs
}

type openAIStreamTimeouts struct {
	firstChunkTimeoutMs int
	idleTimeoutMs       int
	maxDurationMs       int
}

func NormalizeOpenAICompatibleRequestURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse openai-compatible base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("parse openai-compatible base URL: absolute URL is required")
	}
	trimmedPath := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case strings.HasSuffix(trimmedPath, "/chat/completions"):
		// already fully qualified
	case trimmedPath == "":
		trimmedPath = "/v1/chat/completions"
	case strings.HasSuffix(trimmedPath, "/v1"):
		trimmedPath += "/chat/completions"
	default:
		trimmedPath = strings.TrimSuffix(trimmedPath, "/") + "/chat/completions"
	}
	parsed.Path = trimmedPath
	parsed.RawPath = ""
	return parsed.String(), nil
}

func classifyOpenAITransportError(err error) error {
	switch {
	case err == nil:
		return nil
	case isStreamTimeoutError(err):
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return err
	case strings.Contains(err.Error(), "certificate"):
		return &ProviderError{Code: "upstream_transport_error", Message: err.Error(), Retryable: false}
	case isRetryableNetworkError(err):
		return &ProviderError{Code: "upstream_transport_error", Message: err.Error(), Retryable: true}
	default:
		return err
	}
}

func isRetryableNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}

func decodeOpenAICompletionResponse(body io.Reader) (ProviderResponse, error) {
	var response openAICompatibleCompletionResponse
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return ProviderResponse{}, fmt.Errorf("decode openai-compatible response: %w", err)
	}
	if len(response.Choices) == 0 {
		return ProviderResponse{}, &ProviderError{
			Code:      "upstream_invalid_response",
			Message:   "openai-compatible response did not include choices",
			Retryable: false,
		}
	}
	output := extractOpenAIContent(response.Choices[0].Message.Content)
	return ProviderResponse{
		Output:       output,
		FinishReason: response.Choices[0].FinishReason,
		Usage: Usage{
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			TotalTokens:  response.Usage.TotalTokens,
		},
	}, nil
}

func decodeOpenAIStreamResponse(ctx context.Context, cancel context.CancelCauseFunc, body io.Reader, emit StreamEmitter, timeouts openAIStreamTimeouts) (ProviderResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 16*1024), 1024*1024)

	var aggregate strings.Builder
	var finishReason string
	usage := Usage{}
	done := false
	firstChunkSeen := false

	stopIdleTimer := startStreamTimer(
		timeouts.firstChunkTimeoutMs,
		func() error {
			return &ProviderError{Code: "first_chunk_timeout", Message: "openai-compatible stream did not produce a first chunk in time", Retryable: true}
		},
		cancel,
	)
	defer stopIdleTimer()
	stopHardCap := startStreamTimer(
		timeouts.maxDurationMs,
		func() error {
			return &ProviderError{Code: "max_duration_exceeded", Message: "openai-compatible stream exceeded configured maximum duration", Retryable: false}
		},
		cancel,
	)
	defer stopHardCap()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			done = true
			break
		}

		if !firstChunkSeen {
			firstChunkSeen = true
		}
		stopIdleTimer()
		stopIdleTimer = startStreamTimer(
			timeouts.idleTimeoutMs,
			func() error {
				return &ProviderError{Code: "idle_timeout", Message: "openai-compatible stream stalled without progress", Retryable: true}
			},
			cancel,
		)

		var chunk openAICompatibleStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return ProviderResponse{}, fmt.Errorf("decode openai-compatible stream chunk: %w", err)
		}

		if chunk.Usage != nil {
			usage = Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}
		}

		for _, choice := range chunk.Choices {
			delta := extractOpenAIContent(choice.Delta.Content)
			if delta != "" {
				aggregate.WriteString(delta)
				if emit != nil {
					if err := emit(StreamChunk{Delta: delta}); err != nil {
						return ProviderResponse{}, err
					}
				}
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}
	response := ProviderResponse{
		Output:       aggregate.String(),
		FinishReason: finishReason,
		Usage:        usage,
	}
	if err := scanner.Err(); err != nil {
		if causeErr := classifyOpenAIContextCause(ctx); causeErr != nil {
			return response, causeErr
		}
		return ProviderResponse{}, fmt.Errorf("read openai-compatible stream: %w", err)
	}
	if causeErr := classifyOpenAIContextCause(ctx); causeErr != nil {
		return response, causeErr
	}
	if !done {
		return response, &ProviderError{
			Code:      "upstream_stream_incomplete",
			Message:   "openai-compatible stream ended without [DONE]",
			Retryable: true,
		}
	}

	return response, nil
}

func startStreamTimer(timeoutMs int, errFactory func() error, cancel context.CancelCauseFunc) func() {
	if timeoutMs <= 0 || cancel == nil {
		return func() {}
	}
	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		cancel(errFactory())
	})
	return func() {
		if timer != nil {
			timer.Stop()
		}
	}
}

func classifyOpenAIContextCause(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	cause := context.Cause(ctx)
	if cause == nil {
		return nil
	}
	if providerErr := asProviderError(cause); providerErr != nil {
		return providerErr
	}
	return nil
}

func asProviderError(err error) *ProviderError {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr
	}
	return nil
}

func isStreamTimeoutError(err error) bool {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	switch providerErr.Code {
	case "connect_timeout", "first_chunk_timeout", "idle_timeout", "max_duration_exceeded":
		return true
	default:
		return false
	}
}

func decodeOpenAIErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var payload openAICompatibleErrorResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}

	message := strings.TrimSpace(payload.Error.Message)
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Code: "upstream_auth_failed", Message: message, Retryable: false}
	case http.StatusTooManyRequests:
		return &ProviderError{Code: "upstream_rate_limited", Message: message, Retryable: true}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &ProviderError{Code: "upstream_invalid_request", Message: message, Retryable: false}
	default:
		if resp.StatusCode >= http.StatusInternalServerError {
			return &ProviderError{Code: "upstream_unavailable", Message: message, Retryable: true}
		}
		return &ProviderError{Code: "upstream_error", Message: message, Retryable: false}
	}
}

func extractOpenAIContent(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []any:
		var builder strings.Builder
		for _, item := range typed {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := itemMap["text"].(string); ok {
				builder.WriteString(text)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

type openAICompatibleRequest struct {
	Model         string                         `json:"model"`
	Messages      []openAICompatibleMessage      `json:"messages"`
	Stream        bool                           `json:"stream,omitempty"`
	StreamOptions *openAICompatibleStreamOptions `json:"stream_options,omitempty"`
}

type openAICompatibleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAICompatibleStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAICompatibleCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage openAICompatibleUsage `json:"usage"`
}

type openAICompatibleStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content any `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAICompatibleUsage `json:"usage,omitempty"`
}

type openAICompatibleUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAICompatibleErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
