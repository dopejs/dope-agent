package mail

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func healthyMailResource(id string, canonicalDefault bool) integrations.Resource {
	return integrations.Resource{
		IntegrationID:    id,
		DomainKind:       "mail",
		DisplayName:      id,
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		CanonicalDefault: canonicalDefault,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "alice@example.com",
			AccountLabel: "Alice Mailbox",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	}
}

func TestManagerListThreadsSupportsExplicitSelectionAndCanonicalDefault(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := []integrations.Resource{
		healthyMailResource("mail-a", true),
		healthyMailResource("mail-b", false),
	}

	explicitAccount, explicitThreads, explicitOp, _, err := manager.ListThreads(resources, ListThreadsInput{
		Selection: Selection{IntegrationID: "mail-b"},
	})
	if err != nil {
		t.Fatalf("ListThreads(explicit) returned error: %v", err)
	}
	if explicitAccount.IntegrationID != "mail-b" || explicitOp.SelectionMode != "explicit" || len(explicitThreads) == 0 {
		t.Fatalf("expected explicit mail selection and seeded threads, got account=%+v op=%+v threads=%+v", explicitAccount, explicitOp, explicitThreads)
	}

	defaultAccount, _, defaultOp, _, err := manager.ListThreads(resources, ListThreadsInput{})
	if err != nil {
		t.Fatalf("ListThreads(default) returned error: %v", err)
	}
	if defaultAccount.IntegrationID != "mail-a" || defaultOp.SelectionMode != "canonical_default" {
		t.Fatalf("expected canonical default mail selection, got account=%+v op=%+v", defaultAccount, defaultOp)
	}
}

func TestManagerBlocksUnsafeNewOutboundSend(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := []integrations.Resource{healthyMailResource("mail-a", true)}

	_, _, operation, artifacts, err := manager.SendMessage(resources, SendMessageInput{
		Selection: Selection{IntegrationID: "mail-a"},
		To:        []string{"carol@example.com"},
		Subject:   "Blocked attachment",
		AttachmentRefs: []AttachmentRefInput{
			{AttachmentRefID: "missing_manager_attachment"},
		},
	})
	if err != ErrMailAttachmentUnresolved {
		t.Fatalf("expected attachment unresolved error, got %v", err)
	}
	if operation.Status != OperationStatusBlocked || operation.ResultMode != ResultModeBlocked || operation.FailureClass != "attachment_unresolved" || len(artifacts) != 1 {
		t.Fatalf("expected blocked attachment operation truth, got op=%+v artifacts=%+v", operation, artifacts)
	}

	_, _, operation, _, err = manager.SendMessage(resources, SendMessageInput{
		Selection: Selection{IntegrationID: "mail-a"},
		Subject:   "Missing recipients",
	})
	if err != ErrMailRecipientRequired {
		t.Fatalf("expected recipient required error, got %v", err)
	}
	if operation.Status != OperationStatusBlocked || operation.FailureClass != "recipient_required" {
		t.Fatalf("expected blocked recipient-required operation, got %+v", operation)
	}

	_, _, operation, _, err = manager.SendMessage(resources, SendMessageInput{
		Selection: Selection{IntegrationID: "mail-a"},
		To:        []string{"carol@example.com"},
		Subject:   "Workflow send",
		Source: SourceLinkage{
			WorkflowID: "wf_1",
		},
	})
	if err != ErrMailBackgroundSendBlocked {
		t.Fatalf("expected background send blocked error, got %v", err)
	}
	if operation.Status != OperationStatusBlocked || operation.FailureClass != "send_permission_required" {
		t.Fatalf("expected blocked background send operation, got %+v", operation)
	}
}

func TestManagerPreservesSendPathAndReplyLinkage(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := []integrations.Resource{healthyMailResource("mail-a", true)}

	_, draft, createOp, _, err := manager.CreateDraft(resources, CreateDraftInput{
		Selection:   Selection{IntegrationID: "mail-a"},
		ComposeMode: ComposeModeNewMessage,
		To:          []string{"carol@example.com"},
		Subject:     "Draft first",
		Body:        "Draft body",
	})
	if err != nil {
		t.Fatalf("CreateDraft returned error: %v", err)
	}
	if createOp.ResultMode != ResultModeDraftOnly {
		t.Fatalf("expected draft-only create op, got %+v", createOp)
	}

	_, sentDraft, sentMessage, sendDraftOp, _, err := manager.SendDraft(resources, SendDraftInput{
		Selection: Selection{IntegrationID: "mail-a"},
		DraftID:   draft.DraftID,
	})
	if err != nil {
		t.Fatalf("SendDraft returned error: %v", err)
	}
	if sendDraftOp.ResultMode != ResultModeSent || sendDraftOp.SendPath != SendPathDraft || sentDraft.DraftStatus != DraftStatusSentFromDraft || sentMessage.MessageID == "" {
		t.Fatalf("expected truthful send-draft result, got draft=%+v message=%+v op=%+v", sentDraft, sentMessage, sendDraftOp)
	}

	_, replyDraft, replyMessage, replyOp, _, err := manager.ReplyMessage(resources, ReplyMessageInput{
		Selection:  Selection{IntegrationID: "mail-a"},
		MessageID:  "msg_seed",
		ResultMode: ReplyForwardResultModeDraft,
		Body:       "Reply later",
	})
	if err != nil {
		t.Fatalf("ReplyMessage(draft) returned error: %v", err)
	}
	if replyMessage != nil || replyDraft == nil || replyDraft.SourceMessageID != "msg_seed" || replyOp.ResultMode != ResultModeDraftOnly {
		t.Fatalf("expected reply draft linkage, got draft=%+v message=%+v op=%+v", replyDraft, replyMessage, replyOp)
	}
}
