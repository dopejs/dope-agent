package mail

import (
	"context"
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

const domainMail = "mail"

// AdapterBackend must satisfy the mail Backend interface.
var _ Backend = (*AdapterBackend)(nil)

// AdapterBackend implements mail.Backend by dispatching each operation to an out-of-process
// integration adapter over the capability RPC contract (Roadmap 59). It performs provider
// request/response mapping only; the mail Manager retains the operation ledger, idempotency,
// artifacts, and live-validation classification. The adapter is stateless, so
// RestoreIntegrationState is a no-op.
type AdapterBackend struct {
	client       *adapterrpc.Client
	deadline     time.Duration
	providerKind string
}

// NewAdapterBackend builds a mail adapter backend over the given RPC client. The deadline
// bounds each operation (spec clarification Q1 / FR-007b); zero uses the client default.
func NewAdapterBackend(client *adapterrpc.Client, deadline time.Duration) *AdapterBackend {
	return &AdapterBackend{client: client, deadline: deadline}
}

// WithProviderKind records the integration diagnostics provider kind served by this adapter
// (e.g. "feishu_lark", Roadmap 63) so OAuth/scope/token failures land on the existing provider
// diagnostics reason vocabulary (FR-006). The empty default keeps the generic path.
func (b *AdapterBackend) WithProviderKind(kind string) *AdapterBackend {
	b.providerKind = kind
	return b
}

// ProviderKind reports the diagnostics provider kind this adapter serves (may be empty).
func (b *AdapterBackend) ProviderKind() string { return b.providerKind }

func (b *AdapterBackend) op() (context.Context, context.CancelFunc) {
	if b.deadline > 0 {
		return context.WithTimeout(context.Background(), b.deadline)
	}
	return context.WithCancel(context.Background())
}

func (b *AdapterBackend) SupportsResource(resource integrations.Resource) bool {
	return resource.DomainKind == "mail"
}

func (b *AdapterBackend) ProjectAccount(resource integrations.Resource) (AccountProjection, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out AccountProjection
	err := b.client.Dispatch(ctx, domainMail, "ProjectAccount", resource, nil, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) ListThreads(resource integrations.Resource, account AccountProjection, input ListThreadsInput) ([]ThreadSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []ThreadSnapshot
	err := b.client.Dispatch(ctx, domainMail, "ListThreads", resource, payload(account, input), &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) GetThread(resource integrations.Resource, account AccountProjection, threadID string) (ThreadSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out ThreadSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetThread", resource, map[string]any{"account": account, "threadId": threadID}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) GetMessage(resource integrations.Resource, account AccountProjection, messageID string) (MessageSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out MessageSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetMessage", resource, map[string]any{"account": account, "messageId": messageID}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) ListDrafts(resource integrations.Resource, account AccountProjection, input ListDraftsInput) ([]DraftSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []DraftSnapshot
	err := b.client.Dispatch(ctx, domainMail, "ListDrafts", resource, payload(account, input), &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) GetDraft(resource integrations.Resource, account AccountProjection, draftID string) (DraftSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out DraftSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetDraft", resource, map[string]any{"account": account, "draftId": draftID}, &out)
	return out, b.mapErr(err)
}

type draftWithAttachments struct {
	Draft       DraftSnapshot         `json:"draft"`
	Attachments []AttachmentReference `json:"attachments"`
}

func (b *AdapterBackend) CreateDraft(resource integrations.Resource, account AccountProjection, input CreateDraftInput) (DraftSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out draftWithAttachments
	err := b.client.Dispatch(ctx, domainMail, "CreateDraft", resource, payload(account, input), &out)
	return out.Draft, out.Attachments, b.mapErr(err)
}

func (b *AdapterBackend) UpdateDraft(resource integrations.Resource, account AccountProjection, input UpdateDraftInput) (DraftSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out draftWithAttachments
	err := b.client.Dispatch(ctx, domainMail, "UpdateDraft", resource, payload(account, input), &out)
	return out.Draft, out.Attachments, b.mapErr(err)
}

type messageWithAttachments struct {
	Message     MessageSnapshot       `json:"message"`
	Attachments []AttachmentReference `json:"attachments"`
}

func (b *AdapterBackend) SendMessage(resource integrations.Resource, account AccountProjection, input SendMessageInput) (MessageSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out messageWithAttachments
	err := b.client.Dispatch(ctx, domainMail, "SendMessage", resource, payload(account, input), &out)
	return out.Message, out.Attachments, b.mapErr(err)
}

type sendDraftResult struct {
	Draft       DraftSnapshot         `json:"draft"`
	Message     MessageSnapshot       `json:"message"`
	Attachments []AttachmentReference `json:"attachments"`
}

func (b *AdapterBackend) SendDraft(resource integrations.Resource, account AccountProjection, input SendDraftInput) (DraftSnapshot, MessageSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out sendDraftResult
	err := b.client.Dispatch(ctx, domainMail, "SendDraft", resource, payload(account, input), &out)
	return out.Draft, out.Message, out.Attachments, b.mapErr(err)
}

type optionalDraftMessage struct {
	Draft       *DraftSnapshot        `json:"draft"`
	Message     *MessageSnapshot      `json:"message"`
	Attachments []AttachmentReference `json:"attachments"`
}

func (b *AdapterBackend) ReplyMessage(resource integrations.Resource, account AccountProjection, input ReplyMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out optionalDraftMessage
	err := b.client.Dispatch(ctx, domainMail, "ReplyMessage", resource, payload(account, input), &out)
	return out.Draft, out.Message, out.Attachments, b.mapErr(err)
}

func (b *AdapterBackend) ForwardMessage(resource integrations.Resource, account AccountProjection, input ForwardMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out optionalDraftMessage
	err := b.client.Dispatch(ctx, domainMail, "ForwardMessage", resource, payload(account, input), &out)
	return out.Draft, out.Message, out.Attachments, b.mapErr(err)
}

// ResolveAttachments has no error return; on adapter failure it returns no resolved
// attachments and the manager surfaces ErrMailAttachmentUnresolved downstream as today.
func (b *AdapterBackend) ResolveAttachments(resource integrations.Resource, account AccountProjection, refs []AttachmentRefInput, parentKind, parentID string) []AttachmentReference {
	ctx, cancel := b.op()
	defer cancel()
	var out []AttachmentReference
	_ = b.client.Dispatch(ctx, domainMail, "ResolveAttachments", resource, map[string]any{
		"account": account, "refs": refs, "parentKind": parentKind, "parentId": parentID,
	}, &out)
	return out
}

// RestoreIntegrationState is a no-op: the adapter holds no durable state; restore is daemon-owned.
func (b *AdapterBackend) RestoreIntegrationState(integrationID string, threads []ThreadSnapshot, messages []MessageSnapshot, drafts []DraftSnapshot, attachments []AttachmentReference) {
}

func payload(account AccountProjection, input any) map[string]any {
	return map[string]any{"account": account, "input": input}
}

// AdapterFailure wraps an out-of-process adapter failure with the stable, redacted failure
// class and provider kind the mail Manager records on the single operation ledger and feeds
// into integration diagnostics classification (FR-006, FR-007). Detail carries only a stable,
// non-secret token; no credential or message content is included.
type AdapterFailure struct {
	class        string
	providerKind string
	detail       string
	ambiguous    bool
	unavailable  bool
}

func (e *AdapterFailure) Error() string {
	if e.detail != "" {
		return e.detail
	}
	return e.class
}

// Is keeps existing callers matching ErrMailUnavailable for unavailable outcomes.
func (e *AdapterFailure) Is(target error) bool {
	return e.unavailable && target == ErrMailUnavailable
}

// FailureClass returns the stable failure-class token for the operation ledger.
func (e *AdapterFailure) FailureClass() string { return e.class }

// mapErr translates a transport/adapter error into an *AdapterFailure carrying the stable
// failure class and provider kind. Unconfirmed outcomes (deadline expiry, transport break,
// undecodable response) are classified as ambiguous-commit (FR-007a).
func (b *AdapterBackend) mapErr(err error) error {
	if err == nil {
		return nil
	}
	if adapterrpc.IsAmbiguous(err) {
		return &AdapterFailure{class: "ambiguous_commit", providerKind: b.providerKind, detail: err.Error(), ambiguous: true}
	}
	var ae *adapterrpc.AdapterError
	if errors.As(err, &ae) {
		return &AdapterFailure{
			class:        stableFailureClass(ae),
			providerKind: b.providerKind,
			detail:       ae.Detail,
			unavailable:  ae.Kind == adapterrpc.FailureUnavailable,
		}
	}
	return err
}

// stableFailureClass derives a stable, redacted failure-class token from a typed adapter
// failure, preferring the adapter's own redacted token and otherwise the failure kind.
func stableFailureClass(ae *adapterrpc.AdapterError) string {
	if token := ae.Detail; token != "" {
		return token
	}
	switch ae.Kind {
	case adapterrpc.FailureAuth:
		return "user_access_token_invalid"
	case adapterrpc.FailureScope:
		return "scope_not_granted"
	case adapterrpc.FailureRateLimited:
		return "rate_limited"
	case adapterrpc.FailureUnavailable:
		return "service_unavailable"
	case adapterrpc.FailureMalformed:
		return "malformed_provider_response"
	default:
		return "provider_internal_error"
	}
}

// writeFailureReason classifies a side-effecting mutation failure. Retained for legacy callers;
// prefer failureClassAndProvider which also surfaces the provider kind.
func writeFailureReason(defaultReason string, err error) string {
	class, _ := failureClassAndProvider(defaultReason, err)
	return class
}

// failureClassAndProvider extracts the stable failure class and diagnostics provider kind from
// a backend error so the operation ledger and DiagnosticFailure carry stable reasons (FR-006).
func failureClassAndProvider(defaultClass string, err error) (class, providerKind string) {
	var af *AdapterFailure
	if errors.As(err, &af) {
		return af.FailureClass(), af.providerKind
	}
	if adapterrpc.IsAmbiguous(err) {
		return "ambiguous_commit", ""
	}
	return defaultClass, ""
}
