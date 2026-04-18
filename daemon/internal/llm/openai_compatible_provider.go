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
)

const OpenAICompatibleProviderName = "openai_compatible"

type OpenAICompatibleProviderConfig struct {
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

type OpenAICompatibleProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleProviderConfig) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider base URL is required")
	}
	chatURL, err := normalizeChatCompletionsURL(baseURL)
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
		baseURL:      chatURL,
		apiKey:       strings.TrimSpace(cfg.APIKey),
		defaultModel: strings.TrimSpace(cfg.DefaultModel),
		httpClient:   httpClient,
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
		return ProviderResponse{}, err
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(payload))
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("build openai-compatible request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ProviderResponse{}, classifyOpenAITransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return ProviderResponse{}, decodeOpenAIErrorResponse(resp)
	}

	if stream {
		return decodeOpenAIStreamResponse(resp.Body, emit)
	}
	return decodeOpenAICompletionResponse(resp.Body)
}

func normalizeChatCompletionsURL(raw string) (string, error) {
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

func decodeOpenAIStreamResponse(body io.Reader, emit StreamEmitter) (ProviderResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 16*1024), 1024*1024)

	var aggregate strings.Builder
	var finishReason string
	usage := Usage{}
	done := false

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
	if err := scanner.Err(); err != nil {
		return ProviderResponse{}, fmt.Errorf("read openai-compatible stream: %w", err)
	}
	if !done {
		return ProviderResponse{}, &ProviderError{
			Code:      "upstream_stream_incomplete",
			Message:   "openai-compatible stream ended without [DONE]",
			Retryable: true,
		}
	}

	return ProviderResponse{
		Output:       aggregate.String(),
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
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
