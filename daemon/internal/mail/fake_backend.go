package mail

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type fakeState struct {
	account     AccountProjection
	threads     map[string]ThreadSnapshot
	messages    map[string]MessageSnapshot
	drafts      map[string]DraftSnapshot
	attachments map[string]AttachmentReference
}

type FakeBackend struct {
	mu     sync.RWMutex
	states map[string]*fakeState
}

func NewFakeBackend() *FakeBackend {
	return &FakeBackend{states: make(map[string]*fakeState)}
}

func (b *FakeBackend) SupportsResource(resource integrations.Resource) bool {
	return strings.TrimSpace(resource.DomainKind) == "mail" &&
		integrations.BackendKindSupportsDomain(resource.BackendBinding.BackendKind, resource.DomainKind)
}

func (b *FakeBackend) ProjectAccount(resource integrations.Resource) (AccountProjection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	now := time.Now().UTC()
	state.account.ReadinessStatus = string(resource.ReadinessStatus)
	state.account.CanonicalDefault = resource.CanonicalDefault
	state.account.AccountKey = resource.AccountBinding.AccountKey
	state.account.AccountLabel = resource.AccountBinding.AccountLabel
	if state.account.MailboxAddress == "" {
		state.account.MailboxAddress = firstNonEmpty(resource.AccountBinding.AccountKey, "alice@example.com")
	}
	state.account.MailboxLabel = firstNonEmpty(resource.AccountBinding.AccountLabel, "Primary Mailbox")
	state.account.UpdatedAt = now
	state.account.LastSyncedAt = now
	return state.account, nil
}

func (b *FakeBackend) ListThreads(resource integrations.Resource, account AccountProjection, input ListThreadsInput) ([]ThreadSnapshot, error) {
	state := b.ensureState(resource)
	items := make([]ThreadSnapshot, 0, len(state.threads))
	for _, item := range state.threads {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LatestMessageAt.Equal(items[j].LatestMessageAt) {
			return items[i].ThreadID < items[j].ThreadID
		}
		return items[i].LatestMessageAt.After(items[j].LatestMessageAt)
	})
	if input.Limit > 0 && len(items) > input.Limit {
		items = items[:input.Limit]
	}
	return cloneThreads(items), nil
}

func (b *FakeBackend) GetThread(resource integrations.Resource, account AccountProjection, threadID string) (ThreadSnapshot, error) {
	state := b.ensureState(resource)
	item, ok := state.threads[strings.TrimSpace(threadID)]
	if !ok {
		return ThreadSnapshot{}, ErrMailThreadNotFound
	}
	return item, nil
}

func (b *FakeBackend) GetMessage(resource integrations.Resource, account AccountProjection, messageID string) (MessageSnapshot, error) {
	state := b.ensureState(resource)
	item, ok := state.messages[strings.TrimSpace(messageID)]
	if !ok {
		return MessageSnapshot{}, ErrMailMessageNotFound
	}
	return item, nil
}

func (b *FakeBackend) ListDrafts(resource integrations.Resource, account AccountProjection, input ListDraftsInput) ([]DraftSnapshot, error) {
	state := b.ensureState(resource)
	items := make([]DraftSnapshot, 0, len(state.drafts))
	for _, item := range state.drafts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].DraftID < items[j].DraftID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return cloneDrafts(items), nil
}

func (b *FakeBackend) GetDraft(resource integrations.Resource, account AccountProjection, draftID string) (DraftSnapshot, error) {
	state := b.ensureState(resource)
	item, ok := state.drafts[strings.TrimSpace(draftID)]
	if !ok {
		return DraftSnapshot{}, ErrMailDraftNotFound
	}
	return item, nil
}

func (b *FakeBackend) CreateDraft(resource integrations.Resource, account AccountProjection, input CreateDraftInput) (DraftSnapshot, []AttachmentReference, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	now := time.Now().UTC()
	draftID := fmt.Sprintf("draft_%s_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano())
	recipients := joinRecipients(input.To, input.Cc, input.Bcc)
	attachments := b.resolveAttachmentsLocked(resource, account, input.AttachmentRefs, "draft", draftID, now)
	draft := DraftSnapshot{
		DraftID:          draftID,
		ThreadID:         strings.TrimSpace(input.ThreadID),
		IntegrationID:    account.IntegrationID,
		MailAccountID:    account.MailAccountID,
		ComposeMode:      input.ComposeMode,
		SourceMessageID:  strings.TrimSpace(input.SourceMessageID),
		RecipientSummary: recipients,
		Subject:          strings.TrimSpace(input.Subject),
		BodyPreview:      previewBody(input.Body),
		AttachmentRefIDs: attachmentIDs(attachments),
		DraftStatus:      DraftStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	state.drafts[draftID] = draft
	b.attachDraftToThreadLocked(state, draft)
	return draft, attachments, nil
}

func (b *FakeBackend) UpdateDraft(resource integrations.Resource, account AccountProjection, input UpdateDraftInput) (DraftSnapshot, []AttachmentReference, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	draft, ok := state.drafts[strings.TrimSpace(input.DraftID)]
	if !ok {
		return DraftSnapshot{}, nil, ErrMailDraftNotFound
	}
	now := time.Now().UTC()
	recipients := joinRecipients(input.To, input.Cc, input.Bcc)
	if len(recipients) > 0 {
		draft.RecipientSummary = recipients
	}
	if trimmed := strings.TrimSpace(input.Subject); trimmed != "" {
		draft.Subject = trimmed
	}
	if trimmed := strings.TrimSpace(input.Body); trimmed != "" {
		draft.BodyPreview = previewBody(trimmed)
	}
	if len(input.AttachmentRefs) > 0 {
		attachments := b.resolveAttachmentsLocked(resource, account, input.AttachmentRefs, "draft", draft.DraftID, now)
		draft.AttachmentRefIDs = attachmentIDs(attachments)
		draft.DraftStatus = DraftStatusUpdated
		draft.UpdatedAt = now
		state.drafts[draft.DraftID] = draft
		b.attachDraftToThreadLocked(state, draft)
		return draft, attachments, nil
	}
	draft.DraftStatus = DraftStatusUpdated
	draft.UpdatedAt = now
	state.drafts[draft.DraftID] = draft
	b.attachDraftToThreadLocked(state, draft)
	return draft, attachmentsByID(state.attachments, draft.AttachmentRefIDs), nil
}

func (b *FakeBackend) SendMessage(resource integrations.Resource, account AccountProjection, input SendMessageInput) (MessageSnapshot, []AttachmentReference, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	now := time.Now().UTC()
	threadID := fmt.Sprintf("thread_%s_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano())
	messageID := fmt.Sprintf("msg_%s_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano())
	attachments := b.resolveAttachmentsLocked(resource, account, input.AttachmentRefs, "message", messageID, now)
	message := MessageSnapshot{
		MessageID:        messageID,
		ThreadID:         threadID,
		IntegrationID:    account.IntegrationID,
		MailAccountID:    account.MailAccountID,
		Direction:        DirectionOutbound,
		SenderSummary:    account.MailboxAddress,
		RecipientSummary: joinRecipients(input.To, input.Cc, input.Bcc),
		Subject:          strings.TrimSpace(input.Subject),
		BodyPreview:      previewBody(input.Body),
		DeliveryState:    DeliveryStateSent,
		AttachmentRefIDs: attachmentIDs(attachments),
		SentAt:           &now,
		CreatedAt:        now,
	}
	thread := ThreadSnapshot{
		ThreadID:           threadID,
		IntegrationID:      account.IntegrationID,
		MailAccountID:      account.MailAccountID,
		Subject:            message.Subject,
		ParticipantSummary: append([]string{account.MailboxAddress}, message.RecipientSummary...),
		MessageIDs:         []string{messageID},
		LatestMessageAt:    now,
		MessageCount:       1,
		CreatedAt:          now,
	}
	state.messages[messageID] = message
	state.threads[threadID] = thread
	return message, attachments, nil
}

func (b *FakeBackend) SendDraft(resource integrations.Resource, account AccountProjection, input SendDraftInput) (DraftSnapshot, MessageSnapshot, []AttachmentReference, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	draft, ok := state.drafts[strings.TrimSpace(input.DraftID)]
	if !ok {
		return DraftSnapshot{}, MessageSnapshot{}, nil, ErrMailDraftNotFound
	}
	now := time.Now().UTC()
	messageID := fmt.Sprintf("msg_%s_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano())
	message := MessageSnapshot{
		MessageID:        messageID,
		ThreadID:         draft.ThreadID,
		IntegrationID:    account.IntegrationID,
		MailAccountID:    account.MailAccountID,
		Direction:        DirectionOutbound,
		SenderSummary:    account.MailboxAddress,
		RecipientSummary: append([]string(nil), draft.RecipientSummary...),
		Subject:          draft.Subject,
		BodyPreview:      draft.BodyPreview,
		DeliveryState:    DeliveryStateSent,
		AttachmentRefIDs: append([]string(nil), draft.AttachmentRefIDs...),
		SentAt:           &now,
		CreatedAt:        now,
	}
	draft.DraftStatus = DraftStatusSentFromDraft
	draft.UpdatedAt = now
	state.drafts[draft.DraftID] = draft
	state.messages[messageID] = message
	b.appendMessageToThreadLocked(state, draft.ThreadID, message)
	return draft, message, attachmentsByID(state.attachments, draft.AttachmentRefIDs), nil
}

func (b *FakeBackend) ReplyMessage(resource integrations.Resource, account AccountProjection, input ReplyMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error) {
	original, err := b.GetMessage(resource, account, input.MessageID)
	if err != nil {
		return nil, nil, nil, err
	}
	if input.ResultMode == ReplyForwardResultModeDraft {
		draft, attachments, err := b.CreateDraft(resource, account, CreateDraftInput{
			ComposeMode:     ComposeModeReply,
			ThreadID:        original.ThreadID,
			SourceMessageID: original.MessageID,
			To:              []string{original.SenderSummary},
			Subject:         replySubject(original.Subject),
			Body:            input.Body,
			AttachmentRefs:  input.AttachmentRefs,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		return &draft, nil, attachments, nil
	}
	message, attachments, err := b.SendMessage(resource, account, SendMessageInput{
		To:             []string{original.SenderSummary},
		Subject:        firstNonEmpty(strings.TrimSpace(input.Subject), replySubject(original.Subject)),
		Body:           input.Body,
		AttachmentRefs: input.AttachmentRefs,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	message.ThreadID = original.ThreadID
	message.ReplyToMessageID = original.MessageID
	b.mu.Lock()
	state := b.ensureStateLocked(resource)
	state.messages[message.MessageID] = message
	b.appendMessageToThreadLocked(state, original.ThreadID, message)
	b.mu.Unlock()
	return nil, &message, attachments, nil
}

func (b *FakeBackend) ForwardMessage(resource integrations.Resource, account AccountProjection, input ForwardMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error) {
	original, err := b.GetMessage(resource, account, input.MessageID)
	if err != nil {
		return nil, nil, nil, err
	}
	if input.ResultMode == ReplyForwardResultModeDraft {
		draft, attachments, err := b.CreateDraft(resource, account, CreateDraftInput{
			ComposeMode:     ComposeModeForward,
			SourceMessageID: original.MessageID,
			To:              append([]string(nil), input.To...),
			Cc:              append([]string(nil), input.Cc...),
			Bcc:             append([]string(nil), input.Bcc...),
			Subject:         firstNonEmpty(strings.TrimSpace(input.Subject), forwardSubject(original.Subject)),
			Body:            input.Body,
			AttachmentRefs:  input.AttachmentRefs,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		return &draft, nil, attachments, nil
	}
	message, attachments, err := b.SendMessage(resource, account, SendMessageInput{
		To:             append([]string(nil), input.To...),
		Cc:             append([]string(nil), input.Cc...),
		Bcc:            append([]string(nil), input.Bcc...),
		Subject:        firstNonEmpty(strings.TrimSpace(input.Subject), forwardSubject(original.Subject)),
		Body:           input.Body,
		AttachmentRefs: input.AttachmentRefs,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	message.ForwardedFromMessageID = original.MessageID
	b.mu.Lock()
	state := b.ensureStateLocked(resource)
	state.messages[message.MessageID] = message
	b.mu.Unlock()
	return nil, &message, attachments, nil
}

func (b *FakeBackend) ResolveAttachments(resource integrations.Resource, account AccountProjection, refs []AttachmentRefInput, parentKind, parentID string) []AttachmentReference {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resolveAttachmentsLocked(resource, account, refs, parentKind, parentID, time.Now().UTC())
}

func (b *FakeBackend) RestoreIntegrationState(integrationID string, threads []ThreadSnapshot, messages []MessageSnapshot, drafts []DraftSnapshot, attachments []AttachmentReference) {
	b.mu.Lock()
	defer b.mu.Unlock()
	trimmed := strings.TrimSpace(integrationID)
	state, ok := b.states[trimmed]
	if !ok {
		state = &fakeState{
			account: AccountProjection{MailAccountID: "mail_acct_" + trimmed, IntegrationID: trimmed, DomainKind: "mail"},
		}
		b.states[trimmed] = state
	}
	state.threads = make(map[string]ThreadSnapshot, len(threads))
	state.messages = make(map[string]MessageSnapshot, len(messages))
	state.drafts = make(map[string]DraftSnapshot, len(drafts))
	state.attachments = make(map[string]AttachmentReference, len(attachments))
	for _, item := range threads {
		state.threads[item.ThreadID] = item
	}
	for _, item := range messages {
		state.messages[item.MessageID] = item
	}
	for _, item := range drafts {
		state.drafts[item.DraftID] = item
	}
	for _, item := range attachments {
		state.attachments[item.AttachmentRefID] = item
	}
}

func (b *FakeBackend) ensureState(resource integrations.Resource) *fakeState {
	b.mu.RLock()
	state, ok := b.states[resource.IntegrationID]
	b.mu.RUnlock()
	if ok {
		return state
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ensureStateLocked(resource)
}

func (b *FakeBackend) ensureStateLocked(resource integrations.Resource) *fakeState {
	if state, ok := b.states[resource.IntegrationID]; ok {
		return state
	}
	now := time.Now().UTC()
	threadID := "thread_seed"
	messageID := "msg_seed"
	draftID := "draft_seed"
	state := &fakeState{
		account: AccountProjection{
			MailAccountID:            "mail_acct_" + resource.IntegrationID,
			IntegrationID:            resource.IntegrationID,
			DomainKind:               resource.DomainKind,
			EnvironmentScope:         resource.EnvironmentScope,
			AccountKey:               resource.AccountBinding.AccountKey,
			AccountLabel:             resource.AccountBinding.AccountLabel,
			ReadinessStatus:          string(resource.ReadinessStatus),
			CanonicalDefault:         resource.CanonicalDefault,
			SelectionMode:            "explicit",
			MailboxAddress:           firstNonEmpty(resource.AccountBinding.AccountKey, "alice@example.com"),
			MailboxLabel:             firstNonEmpty(resource.AccountBinding.AccountLabel, "Alice Mail"),
			SupportsThreadInspection: true,
			SupportsDrafts:           true,
			SupportsDirectSend:       true,
			SupportsReply:            true,
			SupportsForward:          true,
			LastSyncedAt:             now,
			UpdatedAt:                now,
		},
		threads: map[string]ThreadSnapshot{
			threadID: {
				ThreadID:           threadID,
				IntegrationID:      resource.IntegrationID,
				MailAccountID:      "mail_acct_" + resource.IntegrationID,
				Subject:            "Seed message thread",
				ParticipantSummary: []string{"alice@example.com", "bob@example.com"},
				MessageIDs:         []string{messageID},
				DraftIDs:           []string{draftID},
				LatestMessageAt:    time.Date(2026, 4, 23, 16, 0, 0, 0, time.UTC),
				MessageCount:       1,
				DraftCount:         1,
				CreatedAt:          now,
			},
		},
		messages: map[string]MessageSnapshot{
			messageID: {
				MessageID:        messageID,
				ThreadID:         threadID,
				IntegrationID:    resource.IntegrationID,
				MailAccountID:    "mail_acct_" + resource.IntegrationID,
				Direction:        DirectionInbound,
				SenderSummary:    "bob@example.com",
				RecipientSummary: []string{"alice@example.com"},
				Subject:          "Seed message thread",
				BodyPreview:      "Can you review phase 30 today?",
				DeliveryState:    DeliveryStateReceived,
				ReceivedAt:       timePtr(time.Date(2026, 4, 23, 16, 0, 0, 0, time.UTC)),
				CreatedAt:        now,
			},
		},
		drafts: map[string]DraftSnapshot{
			draftID: {
				DraftID:          draftID,
				ThreadID:         threadID,
				IntegrationID:    resource.IntegrationID,
				MailAccountID:    "mail_acct_" + resource.IntegrationID,
				ComposeMode:      ComposeModeReply,
				SourceMessageID:  messageID,
				RecipientSummary: []string{"bob@example.com"},
				Subject:          "Re: Seed message thread",
				BodyPreview:      "Draft response body.",
				DraftStatus:      DraftStatusDraft,
				CreatedAt:        now,
				UpdatedAt:        now,
			},
		},
		attachments: make(map[string]AttachmentReference),
	}
	b.states[resource.IntegrationID] = state
	return state
}

func (b *FakeBackend) resolveAttachmentsLocked(resource integrations.Resource, account AccountProjection, refs []AttachmentRefInput, parentKind, parentID string, now time.Time) []AttachmentReference {
	if len(refs) == 0 {
		return nil
	}
	items := make([]AttachmentReference, 0, len(refs))
	for idx, ref := range refs {
		attachmentID := strings.TrimSpace(ref.AttachmentRefID)
		if attachmentID == "" {
			attachmentID = fmt.Sprintf("attachment_%s_%d_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano(), idx)
		}
		status := AttachmentResolutionResolved
		failureReason := ""
		if looksUnresolvedAttachment(attachmentID, ref.DisplayName) {
			status = AttachmentResolutionUnresolved
			failureReason = "attachment reference not found"
		}
		item := AttachmentReference{
			AttachmentRefID:  attachmentID,
			IntegrationID:    account.IntegrationID,
			ParentKind:       parentKind,
			ParentID:         parentID,
			DisplayName:      strings.TrimSpace(ref.DisplayName),
			MediaType:        strings.TrimSpace(ref.MediaType),
			SizeBytes:        ref.SizeBytes,
			ResolutionStatus: status,
			FailureReason:    failureReason,
			CreatedAt:        now,
		}
		if item.DisplayName == "" {
			item.DisplayName = "attachment.bin"
		}
		b.states[resource.IntegrationID].attachments[item.AttachmentRefID] = item
		items = append(items, item)
	}
	return items
}

func (b *FakeBackend) attachDraftToThreadLocked(state *fakeState, draft DraftSnapshot) {
	if strings.TrimSpace(draft.ThreadID) == "" {
		return
	}
	thread, ok := state.threads[draft.ThreadID]
	if !ok {
		thread = ThreadSnapshot{
			ThreadID:        draft.ThreadID,
			IntegrationID:   draft.IntegrationID,
			MailAccountID:   draft.MailAccountID,
			Subject:         draft.Subject,
			LatestMessageAt: draft.UpdatedAt,
			CreatedAt:       draft.CreatedAt,
		}
	}
	if !contains(thread.DraftIDs, draft.DraftID) {
		thread.DraftIDs = append(thread.DraftIDs, draft.DraftID)
	}
	thread.DraftCount = len(thread.DraftIDs)
	thread.Subject = firstNonEmpty(draft.Subject, thread.Subject)
	state.threads[draft.ThreadID] = thread
}

func (b *FakeBackend) appendMessageToThreadLocked(state *fakeState, threadID string, message MessageSnapshot) {
	id := strings.TrimSpace(threadID)
	if id == "" {
		id = fmt.Sprintf("thread_%s", message.MessageID)
		message.ThreadID = id
		state.messages[message.MessageID] = message
	}
	thread, ok := state.threads[id]
	if !ok {
		thread = ThreadSnapshot{
			ThreadID:      id,
			IntegrationID: message.IntegrationID,
			MailAccountID: message.MailAccountID,
			Subject:       message.Subject,
			CreatedAt:     message.CreatedAt,
		}
	}
	if !contains(thread.MessageIDs, message.MessageID) {
		thread.MessageIDs = append(thread.MessageIDs, message.MessageID)
	}
	thread.MessageCount = len(thread.MessageIDs)
	if message.SentAt != nil {
		thread.LatestMessageAt = message.SentAt.UTC()
	} else if message.ReceivedAt != nil {
		thread.LatestMessageAt = message.ReceivedAt.UTC()
	}
	thread.Subject = firstNonEmpty(message.Subject, thread.Subject)
	participants := append([]string{}, thread.ParticipantSummary...)
	participants = append(participants, message.SenderSummary)
	participants = append(participants, message.RecipientSummary...)
	thread.ParticipantSummary = uniqueStrings(participants)
	state.threads[id] = thread
}

func attachmentIDs(items []AttachmentReference) []string {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.AttachmentRefID)
	}
	return ids
}

func attachmentsByID(all map[string]AttachmentReference, ids []string) []AttachmentReference {
	if len(ids) == 0 {
		return nil
	}
	items := make([]AttachmentReference, 0, len(ids))
	for _, id := range ids {
		if item, ok := all[id]; ok {
			items = append(items, item)
		}
	}
	return items
}

func looksUnresolvedAttachment(refID, name string) bool {
	text := strings.ToLower(strings.TrimSpace(refID) + " " + strings.TrimSpace(name))
	return strings.Contains(text, "missing") || strings.Contains(text, "broken") || strings.Contains(text, "unresolved")
}

func previewBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if len(trimmed) <= 280 {
		return trimmed
	}
	return trimmed[:280]
}

func replySubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if strings.HasPrefix(strings.ToLower(trimmed), "re:") {
		return trimmed
	}
	return "Re: " + trimmed
}

func forwardSubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if strings.HasPrefix(strings.ToLower(trimmed), "fwd:") {
		return trimmed
	}
	return "Fwd: " + trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cloneThreads(items []ThreadSnapshot) []ThreadSnapshot {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]ThreadSnapshot, len(items))
	copy(cloned, items)
	return cloned
}

func cloneDrafts(items []DraftSnapshot) []DraftSnapshot {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]DraftSnapshot, len(items))
	copy(cloned, items)
	return cloned
}
