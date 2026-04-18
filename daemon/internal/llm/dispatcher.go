package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProviderRequired = errors.New("provider is required")
	ErrProviderNotFound = errors.New("provider not found")
	ErrModelRequired    = errors.New("model is required")
	ErrMessagesRequired = errors.New("messages are required")
)

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
}

type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

type DispatchStatus string

const (
	DispatchStatusQueued    DispatchStatus = "queued"
	DispatchStatusRunning   DispatchStatus = "running"
	DispatchStatusCompleted DispatchStatus = "completed"
	DispatchStatusFailed    DispatchStatus = "failed"
	DispatchStatusCancelled DispatchStatus = "cancelled"
)

type Dispatch struct {
	DispatchID   string         `json:"dispatchId"`
	Provider     string         `json:"provider"`
	Model        string         `json:"model"`
	Messages     []Message      `json:"messages"`
	Stream       bool           `json:"stream"`
	Status       DispatchStatus `json:"status"`
	Output       string         `json:"output"`
	FinishReason string         `json:"finishReason,omitempty"`
	Usage        Usage          `json:"usage"`
	ErrorCode    string         `json:"errorCode,omitempty"`
	Error        string         `json:"error,omitempty"`
	TimeoutMs    int            `json:"timeoutMs"`
	MaxRetries   int            `json:"maxRetries"`
	AttemptCount int            `json:"attemptCount"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	CompletedAt  *time.Time     `json:"completedAt,omitempty"`
}

type CreateDispatchInput struct {
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	TimeoutMs  int       `json:"timeoutMs"`
	MaxRetries int       `json:"maxRetries"`
}

type ProviderRequest struct {
	DispatchID string
	Provider   string
	Model      string
	Messages   []Message
	Attempt    int
	TimeoutMs  int
}

type ProviderResponse struct {
	Output       string
	FinishReason string
	Usage        Usage
}

type StreamChunk struct {
	Delta        string `json:"delta"`
	Output       string `json:"output,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}

type StreamEmitter func(StreamChunk) error

type Provider interface {
	Name() string
	Complete(ctx context.Context, request ProviderRequest) (ProviderResponse, error)
	Stream(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error)
}

type ProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

type Dispatcher struct {
	mu              sync.RWMutex
	providers       map[string]Provider
	defaultProvider string
	defaultModel    string
	defaultTimeout  time.Duration
	defaultRetries  int
}

func NewDispatcher() *Dispatcher {
	dispatcher := &Dispatcher{
		providers:      make(map[string]Provider),
		defaultTimeout: 30 * time.Second,
		defaultRetries: 0,
	}
	dispatcher.RegisterProvider(NewEchoProvider())
	return dispatcher
}

func (d *Dispatcher) RegisterProvider(provider Provider) {
	if provider == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.providers[provider.Name()] = provider
}

func (d *Dispatcher) HasProvider(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, ok := d.providers[strings.TrimSpace(name)]
	return ok
}

func (d *Dispatcher) SetDefaultProvider(name string) error {
	if strings.TrimSpace(name) == "" {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.defaultProvider = ""
		return nil
	}

	if _, err := d.provider(name); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultProvider = name
	return nil
}

func (d *Dispatcher) SetDefaultModel(model string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultModel = strings.TrimSpace(model)
}

func (d *Dispatcher) SetDefaultTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultTimeout = timeout
}

func (d *Dispatcher) SetDefaultRetries(retries int) {
	if retries < 0 {
		retries = 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultRetries = retries
}

func (d *Dispatcher) Prepare(input CreateDispatchInput, stream bool) (Dispatch, error) {
	providerName := strings.TrimSpace(input.Provider)
	if providerName == "" {
		providerName = d.defaultProvider
	}
	if providerName == "" {
		return Dispatch{}, ErrProviderRequired
	}
	modelName := strings.TrimSpace(input.Model)
	if modelName == "" {
		modelName = d.defaultModel
	}
	if modelName == "" {
		return Dispatch{}, ErrModelRequired
	}
	if len(input.Messages) == 0 {
		return Dispatch{}, ErrMessagesRequired
	}
	for _, message := range input.Messages {
		if strings.TrimSpace(message.Content) == "" {
			return Dispatch{}, ErrMessagesRequired
		}
	}
	if _, err := d.provider(providerName); err != nil {
		return Dispatch{}, err
	}

	timeoutMs := input.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = int(d.defaultTimeout / time.Millisecond)
	}
	maxRetries := input.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries == 0 {
		maxRetries = d.defaultRetries
	}

	now := time.Now().UTC()
	return Dispatch{
		DispatchID: uuid.NewString(),
		Provider:   providerName,
		Model:      modelName,
		Messages:   cloneMessages(input.Messages),
		Stream:     stream,
		Status:     DispatchStatusQueued,
		TimeoutMs:  timeoutMs,
		MaxRetries: maxRetries,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, dispatch Dispatch) (Dispatch, error) {
	provider, err := d.provider(dispatch.Provider)
	if err != nil {
		dispatch = failPreparedDispatch(dispatch, "provider_not_found", err.Error(), DispatchStatusFailed)
		return dispatch, err
	}
	return d.execute(ctx, dispatch, provider, nil)
}

func (d *Dispatcher) DispatchStream(ctx context.Context, dispatch Dispatch, emit StreamEmitter) (Dispatch, error) {
	provider, err := d.provider(dispatch.Provider)
	if err != nil {
		dispatch = failPreparedDispatch(dispatch, "provider_not_found", err.Error(), DispatchStatusFailed)
		return dispatch, err
	}
	return d.execute(ctx, dispatch, provider, emit)
}

func (d *Dispatcher) provider(name string) (Provider, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrProviderRequired
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	provider, ok := d.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	return provider, nil
}

func (d *Dispatcher) execute(ctx context.Context, dispatch Dispatch, provider Provider, emit StreamEmitter) (Dispatch, error) {
	startedAt := time.Now().UTC()
	dispatch.Status = DispatchStatusRunning
	dispatch.StartedAt = &startedAt
	dispatch.UpdatedAt = startedAt
	dispatch.Output = ""
	dispatch.Error = ""
	dispatch.ErrorCode = ""

	maxAttempts := dispatch.MaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		dispatch.AttemptCount = attempt
		dispatch.UpdatedAt = time.Now().UTC()

		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(dispatch.TimeoutMs)*time.Millisecond)
		request := ProviderRequest{
			DispatchID: dispatch.DispatchID,
			Provider:   dispatch.Provider,
			Model:      dispatch.Model,
			Messages:   cloneMessages(dispatch.Messages),
			Attempt:    attempt,
			TimeoutMs:  dispatch.TimeoutMs,
		}

		var response ProviderResponse
		var err error
		if emit != nil {
			var aggregate strings.Builder
			response, err = provider.Stream(attemptCtx, request, func(chunk StreamChunk) error {
				aggregate.WriteString(chunk.Delta)
				chunk.Output = aggregate.String()
				return emit(chunk)
			})
			if response.Output == "" {
				response.Output = aggregate.String()
			}
		} else {
			response, err = provider.Complete(attemptCtx, request)
		}
		cancel()

		if err == nil {
			completedAt := time.Now().UTC()
			dispatch.Status = DispatchStatusCompleted
			dispatch.Output = response.Output
			dispatch.FinishReason = response.FinishReason
			dispatch.Usage = normalizeUsage(response.Usage)
			dispatch.Error = ""
			dispatch.ErrorCode = ""
			dispatch.UpdatedAt = completedAt
			dispatch.CompletedAt = &completedAt
			return dispatch, nil
		}

		lastErr = err
		outcome := classifyDispatchError(ctx, err)
		if outcome.retryable && attempt < maxAttempts {
			continue
		}

		completedAt := time.Now().UTC()
		dispatch.Status = outcome.status
		dispatch.ErrorCode = outcome.code
		dispatch.Error = outcome.message
		dispatch.UpdatedAt = completedAt
		dispatch.CompletedAt = &completedAt
		return dispatch, err
	}

	completedAt := time.Now().UTC()
	dispatch.Status = DispatchStatusFailed
	dispatch.ErrorCode = "dispatch_failed"
	dispatch.Error = lastErr.Error()
	dispatch.UpdatedAt = completedAt
	dispatch.CompletedAt = &completedAt
	return dispatch, lastErr
}

type classifiedError struct {
	status    DispatchStatus
	code      string
	message   string
	retryable bool
}

func classifyDispatchError(parentCtx context.Context, err error) classifiedError {
	switch {
	case errors.Is(parentCtx.Err(), context.Canceled):
		return classifiedError{
			status:    DispatchStatusCancelled,
			code:      "cancelled",
			message:   "dispatch cancelled",
			retryable: false,
		}
	case errors.Is(err, context.Canceled):
		return classifiedError{
			status:    DispatchStatusCancelled,
			code:      "cancelled",
			message:   "dispatch cancelled",
			retryable: false,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return classifiedError{
			status:    DispatchStatusFailed,
			code:      "timeout",
			message:   "dispatch timed out",
			retryable: true,
		}
	}

	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		message := providerErr.Message
		if message == "" {
			message = providerErr.Code
		}
		return classifiedError{
			status:    DispatchStatusFailed,
			code:      providerErr.Code,
			message:   message,
			retryable: providerErr.Retryable,
		}
	}

	return classifiedError{
		status:    DispatchStatusFailed,
		code:      "provider_error",
		message:   err.Error(),
		retryable: false,
	}
}

func failPreparedDispatch(dispatch Dispatch, code, message string, status DispatchStatus) Dispatch {
	completedAt := time.Now().UTC()
	dispatch.Status = status
	dispatch.ErrorCode = code
	dispatch.Error = message
	dispatch.UpdatedAt = completedAt
	dispatch.CompletedAt = &completedAt
	return dispatch
}

func cloneMessages(messages []Message) []Message {
	cloned := make([]Message, len(messages))
	copy(cloned, messages)
	return cloned
}

func normalizeUsage(usage Usage) Usage {
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return usage
}

type EchoProvider struct{}

func NewEchoProvider() *EchoProvider {
	return &EchoProvider{}
}

func (p *EchoProvider) Name() string {
	return "echo"
}

func (p *EchoProvider) Complete(_ context.Context, request ProviderRequest) (ProviderResponse, error) {
	output := composeEchoOutput(request.Messages)
	return ProviderResponse{
		Output:       output,
		FinishReason: "stop",
		Usage: Usage{
			InputTokens:  approximateTokens(messagesText(request.Messages)),
			OutputTokens: approximateTokens(output),
		},
	}, nil
}

func (p *EchoProvider) Stream(ctx context.Context, request ProviderRequest, emit StreamEmitter) (ProviderResponse, error) {
	output := composeEchoOutput(request.Messages)
	parts := strings.Fields(output)
	for index, part := range parts {
		if err := ctx.Err(); err != nil {
			return ProviderResponse{}, err
		}

		delta := part
		if index > 0 {
			delta = " " + delta
		}
		if err := emit(StreamChunk{Delta: delta}); err != nil {
			return ProviderResponse{}, err
		}
	}

	return ProviderResponse{
		Output:       output,
		FinishReason: "stop",
		Usage: Usage{
			InputTokens:  approximateTokens(messagesText(request.Messages)),
			OutputTokens: approximateTokens(output),
		},
	}, nil
}

func composeEchoOutput(messages []Message) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, strings.TrimSpace(message.Content))
	}
	return strings.Join(parts, "\n")
}

func messagesText(messages []Message) string {
	return composeEchoOutput(messages)
}

func approximateTokens(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return len(strings.Fields(text))
}
