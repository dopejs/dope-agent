package feishulark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

// MailProvider implements the adapterprovider.Handler for the Feishu/Lark mail domain (Roadmap
// 63). It maps the Feishu Open Platform Mail API onto the existing mail domain resources. It is
// stateless and records no ledger state; the daemon owns that. Full attachment transfer is out
// of scope (Roadmap 64): attachment references are surfaced unresolved so attachment-bearing
// sends are blocked daemon-side rather than silently sent without the attachment.
type MailProvider struct {
	client *Client
}

// NewMailProvider builds a mail provider over the given client.
func NewMailProvider(client *Client) *MailProvider {
	return &MailProvider{client: client}
}

var _ adapterprovider.Handler = (*MailProvider)(nil)

func (p *MailProvider) Handle(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
	if op.Domain != "mail" {
		return nil, (&providerFault{kind: faultInternal, code: "unsupported_domain", message: "adapter serves the mail domain only"}).toAdapterFault()
	}
	token, fault := parseToken(op.Credential)
	if fault != nil {
		return nil, fault.toAdapterFault()
	}
	var resource integrations.Resource
	if len(op.Resource) > 0 {
		_ = json.Unmarshal(op.Resource, &resource)
	}
	result, pf := p.route(ctx, token, resource, op)
	if pf != nil {
		if pf.isAmbiguous() {
			return nil, adapterprovider.ErrAmbiguous
		}
		return nil, pf.toAdapterFault()
	}
	return result, nil
}

// draftWithAttachments / messageWithAttachments / sendDraftResult mirror the shapes the mail
// AdapterBackend shim decodes.
type draftWithAttachments struct {
	Draft       mail.DraftSnapshot         `json:"draft"`
	Attachments []mail.AttachmentReference `json:"attachments"`
}
type messageWithAttachments struct {
	Message     mail.MessageSnapshot       `json:"message"`
	Attachments []mail.AttachmentReference `json:"attachments"`
}
type sendDraftResult struct {
	Draft       mail.DraftSnapshot         `json:"draft"`
	Message     mail.MessageSnapshot       `json:"message"`
	Attachments []mail.AttachmentReference `json:"attachments"`
}
type optionalDraftMessage struct {
	Draft       *mail.DraftSnapshot        `json:"draft"`
	Message     *mail.MessageSnapshot      `json:"message"`
	Attachments []mail.AttachmentReference `json:"attachments"`
}

func (p *MailProvider) route(ctx context.Context, token scopedToken, resource integrations.Resource, op adapterprovider.Operation) (json.RawMessage, *providerFault) {
	switch op.Operation {
	case "ProjectAccount":
		return marshalResult(p.projectAccount(ctx, token, resource))
	case "ListThreads":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.ListThreadsInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.listThreads(ctx, token, in.Account, in.Input))
	case "GetThread":
		var in struct {
			Account  mail.AccountProjection `json:"account"`
			ThreadID string                 `json:"threadId"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.getThread(ctx, token, in.Account, in.ThreadID))
	case "GetMessage":
		var in struct {
			Account   mail.AccountProjection `json:"account"`
			MessageID string                 `json:"messageId"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.getMessage(ctx, token, in.Account, in.MessageID))
	case "ListDrafts":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.ListDraftsInput   `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.listDrafts(ctx, token, in.Account))
	case "GetDraft":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			DraftID string                 `json:"draftId"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.getDraft(ctx, token, in.Account, in.DraftID))
	case "CreateDraft":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.CreateDraftInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.createDraft(ctx, token, in.Account, in.Input))
	case "UpdateDraft":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.UpdateDraftInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.updateDraft(ctx, token, in.Account, in.Input))
	case "SendMessage":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.SendMessageInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.sendMessage(ctx, token, in.Account, in.Input))
	case "SendDraft":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.SendDraftInput    `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.sendDraft(ctx, token, in.Account, in.Input))
	case "ReplyMessage":
		var in struct {
			Account mail.AccountProjection `json:"account"`
			Input   mail.ReplyMessageInput `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.replyMessage(ctx, token, in.Account, in.Input))
	case "ForwardMessage":
		var in struct {
			Account mail.AccountProjection   `json:"account"`
			Input   mail.ForwardMessageInput `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.forwardMessage(ctx, token, in.Account, in.Input))
	case "ResolveAttachments":
		var in struct {
			Account    mail.AccountProjection    `json:"account"`
			Refs       []mail.AttachmentRefInput `json:"refs"`
			ParentKind string                    `json:"parentKind"`
			ParentID   string                    `json:"parentId"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(resolveWithPolicy(in.Refs, in.ParentKind, in.ParentID), nil)
	case "DownloadAttachment":
		var in struct {
			Account mail.AccountProjection       `json:"account"`
			Input   mail.DownloadAttachmentInput `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.downloadAttachment(ctx, token, in.Account, in.Input))
	default:
		return nil, &providerFault{kind: faultInternal, code: "unsupported_operation", message: "unsupported mail operation"}
	}
}

// ---- Feishu mail API response shapes ----

type feishuMailThread struct {
	ThreadID        string   `json:"thread_id"`
	Subject         string   `json:"subject"`
	Participants    []string `json:"participants"`
	MessageIDs      []string `json:"message_ids"`
	LatestMessageAt int64    `json:"latest_message_time"` // unix seconds
	MessageCount    int      `json:"message_count"`
}

type feishuMailMessage struct {
	MessageID     string   `json:"message_id"`
	ThreadID      string   `json:"thread_id"`
	Subject       string   `json:"subject"`
	From          string   `json:"from"`
	To            []string `json:"to"`
	BodyPreview   string   `json:"body_preview"`
	ReplyTo       string   `json:"in_reply_to"`
	ForwardedFrom string   `json:"forwarded_from"`
	SentAt        int64    `json:"sent_time"`
}

type feishuMailDraft struct {
	DraftID     string   `json:"draft_id"`
	ThreadID    string   `json:"thread_id"`
	Subject     string   `json:"subject"`
	To          []string `json:"to"`
	BodyPreview string   `json:"body_preview"`
}

func (p *MailProvider) projectAccount(ctx context.Context, token scopedToken, resource integrations.Resource) (mail.AccountProjection, *providerFault) {
	var out struct {
		MailboxAddress string `json:"mailbox_address"`
		Name           string `json:"name"`
	}
	if pf := p.client.call(ctx, "GET", "/open-apis/mail/v1/user_mailboxes/primary", token.AccessToken, nil, &out, false); pf != nil {
		return mail.AccountProjection{}, pf
	}
	address := firstNonEmpty(out.MailboxAddress, resource.AccountBinding.AccountKey, resource.AccountBinding.ExternalAccountID)
	now := time.Now().UTC()
	return mail.AccountProjection{
		MailAccountID:            "fl_" + address,
		IntegrationID:            resource.IntegrationID,
		DomainKind:               "mail",
		EnvironmentScope:         resource.EnvironmentScope,
		AccountKey:               address,
		AccountLabel:             firstNonEmpty(out.Name, resource.AccountBinding.AccountLabel),
		ReadinessStatus:          string(integrations.ReadinessStatusHealthy),
		CanonicalDefault:         resource.CanonicalDefault,
		MailboxAddress:           address,
		MailboxLabel:             firstNonEmpty(out.Name, address),
		SupportsThreadInspection: true,
		SupportsDrafts:           true,
		SupportsDirectSend:       true,
		SupportsReply:            true,
		SupportsForward:          true,
		LastSyncedAt:             now,
		UpdatedAt:                now,
	}, nil
}

func (p *MailProvider) listThreads(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.ListThreadsInput) ([]mail.ThreadSnapshot, *providerFault) {
	q := url.Values{}
	if input.Limit > 0 {
		q.Set("page_size", fmt.Sprint(input.Limit))
	}
	if strings.TrimSpace(input.Cursor) != "" {
		q.Set("page_token", input.Cursor)
	}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/threads?%s", url.PathEscape(account.MailboxAddress), q.Encode())
	var out struct {
		Items []feishuMailThread `json:"items"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return nil, pf
	}
	threads := make([]mail.ThreadSnapshot, 0, len(out.Items))
	for _, item := range out.Items {
		threads = append(threads, mapThread(account, item))
	}
	return threads, nil
}

func (p *MailProvider) getThread(ctx context.Context, token scopedToken, account mail.AccountProjection, threadID string) (mail.ThreadSnapshot, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/threads/%s", url.PathEscape(account.MailboxAddress), url.PathEscape(threadID))
	var out struct {
		Thread feishuMailThread `json:"thread"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return mail.ThreadSnapshot{}, pf
	}
	return mapThread(account, out.Thread), nil
}

func (p *MailProvider) getMessage(ctx context.Context, token scopedToken, account mail.AccountProjection, messageID string) (mail.MessageSnapshot, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/messages/%s", url.PathEscape(account.MailboxAddress), url.PathEscape(messageID))
	var out struct {
		Message feishuMailMessage `json:"message"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return mail.MessageSnapshot{}, pf
	}
	return mapMessage(account, out.Message, mail.DirectionInbound), nil
}

func (p *MailProvider) listDrafts(ctx context.Context, token scopedToken, account mail.AccountProjection) ([]mail.DraftSnapshot, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/drafts", url.PathEscape(account.MailboxAddress))
	var out struct {
		Items []feishuMailDraft `json:"items"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return nil, pf
	}
	drafts := make([]mail.DraftSnapshot, 0, len(out.Items))
	for _, item := range out.Items {
		drafts = append(drafts, mapDraft(account, item, mail.ComposeModeNewMessage))
	}
	return drafts, nil
}

func (p *MailProvider) getDraft(ctx context.Context, token scopedToken, account mail.AccountProjection, draftID string) (mail.DraftSnapshot, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/drafts/%s", url.PathEscape(account.MailboxAddress), url.PathEscape(draftID))
	var out struct {
		Draft feishuMailDraft `json:"draft"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return mail.DraftSnapshot{}, pf
	}
	return mapDraft(account, out.Draft, mail.ComposeModeNewMessage), nil
}

func (p *MailProvider) createDraft(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.CreateDraftInput) (draftWithAttachments, *providerFault) {
	body := map[string]any{"subject": input.Subject, "body": input.Body, "to": input.To, "cc": input.Cc, "bcc": input.Bcc}
	if strings.TrimSpace(input.ThreadID) != "" {
		body["thread_id"] = input.ThreadID
	}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/drafts", url.PathEscape(account.MailboxAddress))
	var out struct {
		Draft feishuMailDraft `json:"draft"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return draftWithAttachments{}, pf
	}
	draft := mapDraft(account, out.Draft, input.ComposeMode)
	draft.ThreadID = firstNonEmpty(draft.ThreadID, input.ThreadID)
	return draftWithAttachments{Draft: draft, Attachments: resolveWithPolicy(input.AttachmentRefs, "draft", draft.DraftID)}, nil
}

func (p *MailProvider) updateDraft(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.UpdateDraftInput) (draftWithAttachments, *providerFault) {
	body := map[string]any{"subject": input.Subject, "body": input.Body, "to": input.To, "cc": input.Cc, "bcc": input.Bcc}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/drafts/%s", url.PathEscape(account.MailboxAddress), url.PathEscape(input.DraftID))
	var out struct {
		Draft feishuMailDraft `json:"draft"`
	}
	if pf := p.client.call(ctx, "PATCH", path, token.AccessToken, body, &out, true); pf != nil {
		return draftWithAttachments{}, pf
	}
	draft := mapDraft(account, out.Draft, mail.ComposeModeNewMessage)
	if draft.DraftID == "" {
		draft.DraftID = input.DraftID
	}
	draft.DraftStatus = mail.DraftStatusUpdated
	return draftWithAttachments{Draft: draft, Attachments: resolveWithPolicy(input.AttachmentRefs, "draft", draft.DraftID)}, nil
}

func (p *MailProvider) sendMessage(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.SendMessageInput) (messageWithAttachments, *providerFault) {
	body := map[string]any{"subject": input.Subject, "body": input.Body, "to": input.To, "cc": input.Cc, "bcc": input.Bcc}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/messages/send", url.PathEscape(account.MailboxAddress))
	var out struct {
		Message feishuMailMessage `json:"message"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return messageWithAttachments{}, pf
	}
	return messageWithAttachments{Message: mapMessage(account, out.Message, mail.DirectionOutbound), Attachments: resolveWithPolicy(input.AttachmentRefs, "message", out.Message.MessageID)}, nil
}

func (p *MailProvider) sendDraft(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.SendDraftInput) (sendDraftResult, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/drafts/%s/send", url.PathEscape(account.MailboxAddress), url.PathEscape(input.DraftID))
	var out struct {
		Draft   feishuMailDraft   `json:"draft"`
		Message feishuMailMessage `json:"message"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, map[string]any{}, &out, true); pf != nil {
		return sendDraftResult{}, pf
	}
	draft := mapDraft(account, out.Draft, mail.ComposeModeNewMessage)
	if draft.DraftID == "" {
		draft.DraftID = input.DraftID
	}
	draft.DraftStatus = mail.DraftStatusSentFromDraft
	return sendDraftResult{Draft: draft, Message: mapMessage(account, out.Message, mail.DirectionOutbound)}, nil
}

func (p *MailProvider) replyMessage(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.ReplyMessageInput) (optionalDraftMessage, *providerFault) {
	body := map[string]any{"subject": input.Subject, "body": input.Body}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/messages/%s/reply", url.PathEscape(account.MailboxAddress), url.PathEscape(input.MessageID))
	if input.ResultMode == mail.ReplyForwardResultModeDraft {
		var out struct {
			Draft feishuMailDraft `json:"draft"`
		}
		if pf := p.client.call(ctx, "POST", path+"?as_draft=true", token.AccessToken, body, &out, true); pf != nil {
			return optionalDraftMessage{}, pf
		}
		draft := mapDraft(account, out.Draft, mail.ComposeModeReply)
		draft.SourceMessageID = input.MessageID
		return optionalDraftMessage{Draft: &draft}, nil
	}
	var out struct {
		Message feishuMailMessage `json:"message"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return optionalDraftMessage{}, pf
	}
	msg := mapMessage(account, out.Message, mail.DirectionOutbound)
	if msg.ReplyToMessageID == "" {
		msg.ReplyToMessageID = input.MessageID
	}
	return optionalDraftMessage{Message: &msg}, nil
}

func (p *MailProvider) forwardMessage(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.ForwardMessageInput) (optionalDraftMessage, *providerFault) {
	body := map[string]any{"subject": input.Subject, "body": input.Body, "to": input.To, "cc": input.Cc, "bcc": input.Bcc}
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/messages/%s/forward", url.PathEscape(account.MailboxAddress), url.PathEscape(input.MessageID))
	if input.ResultMode == mail.ReplyForwardResultModeDraft {
		var out struct {
			Draft feishuMailDraft `json:"draft"`
		}
		if pf := p.client.call(ctx, "POST", path+"?as_draft=true", token.AccessToken, body, &out, true); pf != nil {
			return optionalDraftMessage{}, pf
		}
		draft := mapDraft(account, out.Draft, mail.ComposeModeForward)
		draft.SourceMessageID = input.MessageID
		return optionalDraftMessage{Draft: &draft}, nil
	}
	var out struct {
		Message feishuMailMessage `json:"message"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return optionalDraftMessage{}, pf
	}
	msg := mapMessage(account, out.Message, mail.DirectionOutbound)
	if msg.ForwardedFromMessageID == "" {
		msg.ForwardedFromMessageID = input.MessageID
	}
	return optionalDraftMessage{Message: &msg}, nil
}

// ---- mapping helpers ----

func mapThread(account mail.AccountProjection, item feishuMailThread) mail.ThreadSnapshot {
	return mail.ThreadSnapshot{
		ThreadID:           item.ThreadID,
		IntegrationID:      account.IntegrationID,
		MailAccountID:      account.MailAccountID,
		Subject:            item.Subject,
		ParticipantSummary: item.Participants,
		MessageIDs:         item.MessageIDs,
		LatestMessageAt:    time.Unix(item.LatestMessageAt, 0).UTC(),
		MessageCount:       item.MessageCount,
		CreatedAt:          time.Now().UTC(),
	}
}

func mapMessage(account mail.AccountProjection, item feishuMailMessage, direction mail.Direction) mail.MessageSnapshot {
	now := time.Now().UTC()
	msg := mail.MessageSnapshot{
		MessageID:              item.MessageID,
		ThreadID:               item.ThreadID,
		IntegrationID:          account.IntegrationID,
		MailAccountID:          account.MailAccountID,
		Direction:              direction,
		SenderSummary:          item.From,
		RecipientSummary:       item.To,
		Subject:                item.Subject,
		BodyPreview:            item.BodyPreview,
		ReplyToMessageID:       item.ReplyTo,
		ForwardedFromMessageID: item.ForwardedFrom,
		DeliveryState:          mail.DeliveryStateSent,
		CreatedAt:              now,
	}
	if direction == mail.DirectionInbound {
		msg.DeliveryState = mail.DeliveryStateReceived
	}
	if item.SentAt > 0 {
		sent := time.Unix(item.SentAt, 0).UTC()
		msg.SentAt = &sent
	}
	return msg
}

func mapDraft(account mail.AccountProjection, item feishuMailDraft, mode mail.ComposeMode) mail.DraftSnapshot {
	now := time.Now().UTC()
	return mail.DraftSnapshot{
		DraftID:          item.DraftID,
		ThreadID:         item.ThreadID,
		IntegrationID:    account.IntegrationID,
		MailAccountID:    account.MailAccountID,
		ComposeMode:      mode,
		RecipientSummary: item.To,
		Subject:          item.Subject,
		BodyPreview:      item.BodyPreview,
		DraftStatus:      mail.DraftStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// resolveWithPolicy resolves attachment references under transfer policy (Roadmap 64): within
// policy they resolve (resolved) with retention/redaction metadata; over-limit or unsafe ones
// fail explicitly (too_large / unsupported_type) so the daemon blocks the send with no partial.
func resolveWithPolicy(refs []mail.AttachmentRefInput, parentKind, parentID string) []mail.AttachmentReference {
	if len(refs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	out := make([]mail.AttachmentReference, 0, len(refs))
	for _, r := range refs {
		ref := mail.AttachmentReference{
			AttachmentRefID:  r.AttachmentRefID,
			ParentKind:       parentKind,
			ParentID:         parentID,
			DisplayName:      r.DisplayName,
			MediaType:        r.MediaType,
			SizeBytes:        r.SizeBytes,
			ResolutionStatus: mail.AttachmentResolutionResolved,
			CreatedAt:        now,
		}
		mail.ApplyAttachmentPolicy(&ref)
		out = append(out, ref)
	}
	return out
}

func (p *MailProvider) downloadAttachment(ctx context.Context, token scopedToken, account mail.AccountProjection, input mail.DownloadAttachmentInput) (mail.AttachmentReference, *providerFault) {
	path := fmt.Sprintf("/open-apis/mail/v1/user_mailboxes/%s/messages/%s/attachments/%s", url.PathEscape(account.MailboxAddress), url.PathEscape(input.MessageID), url.PathEscape(input.AttachmentRefID))
	var out struct {
		DisplayName string `json:"display_name"`
		MediaType   string `json:"media_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return mail.AttachmentReference{}, pf
	}
	ref := mail.AttachmentReference{
		AttachmentRefID:  input.AttachmentRefID,
		IntegrationID:    account.IntegrationID,
		ParentKind:       "message",
		ParentID:         input.MessageID,
		DisplayName:      firstNonEmpty(out.DisplayName, input.DisplayName, "attachment.bin"),
		MediaType:        firstNonEmpty(out.MediaType, input.MediaType),
		SizeBytes:        nonZero(out.SizeBytes, input.SizeBytes),
		ResolutionStatus: mail.AttachmentResolutionResolved,
		CreatedAt:        time.Now().UTC(),
	}
	mail.ApplyAttachmentPolicy(&ref)
	if ref.ResolutionStatus == mail.AttachmentResolutionResolved {
		ref.Downloaded = true
	}
	return ref, nil
}

func nonZero(a, b int64) int64 {
	if a != 0 {
		return a
	}
	return b
}
