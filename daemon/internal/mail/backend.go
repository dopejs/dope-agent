package mail

import (
	"errors"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

var (
	ErrMailUnavailable           = errors.New("mail integration is unavailable")
	ErrMailIntegrationNotFound   = errors.New("mail integration not found")
	ErrMailSelectionInvalid      = errors.New("mail integration selection is invalid")
	ErrMailOperationNotFound     = errors.New("mail operation not found")
	ErrMailAccountNotFound       = errors.New("mail account projection not found")
	ErrMailThreadNotFound        = errors.New("mail thread not found")
	ErrMailMessageNotFound       = errors.New("mail message not found")
	ErrMailDraftNotFound         = errors.New("mail draft not found")
	ErrMailRecipientRequired     = errors.New("explicit recipients are required for new outbound mail")
	ErrMailAttachmentUnresolved  = errors.New("attachment reference could not be resolved")
	ErrMailBackgroundSendBlocked = errors.New("background final send requires explicit allowSendSideEffects permission")
)

type Backend interface {
	SupportsResource(resource integrations.Resource) bool
	ProjectAccount(resource integrations.Resource) (AccountProjection, error)
	ListThreads(resource integrations.Resource, account AccountProjection, input ListThreadsInput) ([]ThreadSnapshot, error)
	GetThread(resource integrations.Resource, account AccountProjection, threadID string) (ThreadSnapshot, error)
	GetMessage(resource integrations.Resource, account AccountProjection, messageID string) (MessageSnapshot, error)
	ListDrafts(resource integrations.Resource, account AccountProjection, input ListDraftsInput) ([]DraftSnapshot, error)
	GetDraft(resource integrations.Resource, account AccountProjection, draftID string) (DraftSnapshot, error)
	CreateDraft(resource integrations.Resource, account AccountProjection, input CreateDraftInput) (DraftSnapshot, []AttachmentReference, error)
	UpdateDraft(resource integrations.Resource, account AccountProjection, input UpdateDraftInput) (DraftSnapshot, []AttachmentReference, error)
	SendMessage(resource integrations.Resource, account AccountProjection, input SendMessageInput) (MessageSnapshot, []AttachmentReference, error)
	SendDraft(resource integrations.Resource, account AccountProjection, input SendDraftInput) (DraftSnapshot, MessageSnapshot, []AttachmentReference, error)
	ReplyMessage(resource integrations.Resource, account AccountProjection, input ReplyMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error)
	ForwardMessage(resource integrations.Resource, account AccountProjection, input ForwardMessageInput) (*DraftSnapshot, *MessageSnapshot, []AttachmentReference, error)
	ResolveAttachments(resource integrations.Resource, account AccountProjection, refs []AttachmentRefInput, parentKind, parentID string) []AttachmentReference
	RestoreIntegrationState(integrationID string, threads []ThreadSnapshot, messages []MessageSnapshot, drafts []DraftSnapshot, attachments []AttachmentReference)
}

func joinRecipients(parts ...[]string) []string {
	out := make([]string, 0)
	for _, group := range parts {
		for _, item := range group {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
