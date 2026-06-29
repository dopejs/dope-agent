package mail

import (
	"context"
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
	client   *adapterrpc.Client
	deadline time.Duration
}

// NewAdapterBackend builds a mail adapter backend over the given RPC client. The deadline
// bounds each operation (spec clarification Q1 / FR-007b); zero uses the client default.
func NewAdapterBackend(client *adapterrpc.Client, deadline time.Duration) *AdapterBackend {
	return &AdapterBackend{client: client, deadline: deadline}
}

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
	return out, mapMailAdapterError(err)
}

func (b *AdapterBackend) ListThreads(resource integrations.Resource, account AccountProjection, input ListThreadsInput) ([]ThreadSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []ThreadSnapshot
	err := b.client.Dispatch(ctx, domainMail, "ListThreads", resource, payload(account, input), &out)
	return out, mapMailAdapterError(err)
}

func (b *AdapterBackend) GetThread(resource integrations.Resource, account AccountProjection, threadID string) (ThreadSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out ThreadSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetThread", resource, map[string]any{"account": account, "threadId": threadID}, &out)
	return out, mapMailAdapterError(err)
}

func (b *AdapterBackend) GetMessage(resource integrations.Resource, account AccountProjection, messageID string) (MessageSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out MessageSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetMessage", resource, map[string]any{"account": account, "messageId": messageID}, &out)
	return out, mapMailAdapterError(err)
}

func (b *AdapterBackend) ListDrafts(resource integrations.Resource, account AccountProjection, input ListDraftsInput) ([]DraftSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []DraftSnapshot
	err := b.client.Dispatch(ctx, domainMail, "ListDrafts", resource, payload(account, input), &out)
	return out, mapMailAdapterError(err)
}

func (b *AdapterBackend) GetDraft(resource integrations.Resource, account AccountProjection, draftID string) (DraftSnapshot, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out DraftSnapshot
	err := b.client.Dispatch(ctx, domainMail, "GetDraft", resource, map[string]any{"account": account, "draftId": draftID}, &out)
	return out, mapMailAdapterError(err)
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
	return out.Draft, out.Attachments, mapMailAdapterError(err)
}

func (b *AdapterBackend) UpdateDraft(resource integrations.Resource, account AccountProjection, input UpdateDraftInput) (DraftSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out draftWithAttachments
	err := b.client.Dispatch(ctx, domainMail, "UpdateDraft", resource, payload(account, input), &out)
	return out.Draft, out.Attachments, mapMailAdapterError(err)
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
	return out.Message, out.Attachments, mapMailAdapterError(err)
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
	return out.Draft, out.Message, out.Attachments, mapMailAdapterError(err)
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
	return out.Draft, out.Message, out.Attachments, mapMailAdapterError(err)
}

func (b *AdapterBackend) ForwardMessage(resource integrations.Resource, account AccountProjection, input ForwardMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out optionalDraftMessage
	err := b.client.Dispatch(ctx, domainMail, "ForwardMessage", resource, payload(account, input), &out)
	return out.Draft, out.Message, out.Attachments, mapMailAdapterError(err)
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

// writeFailureReason classifies a side-effecting mutation failure. Unconfirmed outcomes
// (deadline expiry, transport break, undecodable response) are recorded as ambiguous-commit
// in the single operation ledger (FR-007a).
func writeFailureReason(defaultReason string, err error) string {
	if adapterrpc.IsAmbiguous(err) {
		return "ambiguous_commit"
	}
	return defaultReason
}

func mapMailAdapterError(err error) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*adapterrpc.AdapterError); ok && ae.Kind == adapterrpc.FailureUnavailable {
		return ErrMailUnavailable
	}
	return err
}
