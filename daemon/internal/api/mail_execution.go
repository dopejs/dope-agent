package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

type mailExecutionResult struct {
	Account   mail.AccountProjection
	Operation mail.Operation
	Artifacts []mail.Artifact
	Output    any
}

func buildMailAction(request *MailWorkflowActionRequest) (*mail.Action, error) {
	if request == nil {
		return nil, nil
	}
	action := &mail.Action{
		OperationClass:       request.OperationClass,
		IntegrationID:        strings.TrimSpace(request.IntegrationID),
		ThreadID:             strings.TrimSpace(request.ThreadID),
		MessageID:            strings.TrimSpace(request.MessageID),
		DraftID:              strings.TrimSpace(request.DraftID),
		ComposeMode:          request.ComposeMode,
		ResultMode:           request.ResultMode,
		To:                   append([]string(nil), request.To...),
		Cc:                   append([]string(nil), request.Cc...),
		Bcc:                  append([]string(nil), request.Bcc...),
		Subject:              strings.TrimSpace(request.Subject),
		Body:                 request.Body,
		AttachmentRefs:       mailAttachmentInputs(request.AttachmentRefs),
		AllowSendSideEffects: request.AllowSendSideEffects,
	}
	if strings.TrimSpace(string(action.OperationClass)) == "" {
		return nil, errors.New("mailAction.operationClass is required")
	}
	return action, nil
}

func decodeMailAction(value any) (mail.Action, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return mail.Action{}, err
	}
	var action mail.Action
	if err := json.Unmarshal(payload, &action); err != nil {
		return mail.Action{}, err
	}
	return action, nil
}

func executeMailAction(manager *mail.Manager, integrationsManager *integrations.Manager, action mail.Action, source mail.SourceLinkage) (mailExecutionResult, error) {
	if manager == nil || integrationsManager == nil {
		return mailExecutionResult{}, errors.New("mail dependencies are not configured")
	}
	switch action.OperationClass {
	case mail.OperationClassListThreads:
		account, items, operation, artifacts, err := manager.ListThreads(integrationsManager.List(), mail.ListThreadsInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailThreadListResponse{Account: account, Items: items, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassGetThread:
		account, item, operation, artifacts, err := manager.GetThread(integrationsManager.List(), mail.GetThreadInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			ThreadID:  strings.TrimSpace(action.ThreadID),
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailThreadResponse{Account: account, Thread: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassGetMessage:
		account, item, operation, artifacts, err := manager.GetMessage(integrationsManager.List(), mail.GetMessageInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			MessageID: strings.TrimSpace(action.MessageID),
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailMessageResponse{Account: account, Message: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassListDrafts:
		account, items, operation, artifacts, err := manager.ListDrafts(integrationsManager.List(), mail.ListDraftsInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailDraftListResponse{Account: account, Items: items, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassGetDraft:
		account, item, operation, artifacts, err := manager.GetDraft(integrationsManager.List(), mail.GetDraftInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			DraftID:   strings.TrimSpace(action.DraftID),
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailDraftResponse{Account: account, Draft: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassCreateDraft:
		account, item, operation, artifacts, err := manager.CreateDraft(integrationsManager.List(), mail.CreateDraftInput{
			Selection:       mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			ComposeMode:     action.ComposeMode,
			ThreadID:        strings.TrimSpace(action.ThreadID),
			SourceMessageID: strings.TrimSpace(action.MessageID),
			To:              append([]string(nil), action.To...),
			Cc:              append([]string(nil), action.Cc...),
			Bcc:             append([]string(nil), action.Bcc...),
			Subject:         strings.TrimSpace(action.Subject),
			Body:            action.Body,
			AttachmentRefs:  append([]mail.AttachmentRefInput(nil), action.AttachmentRefs...),
			Source:          source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailDraftResponse{Account: account, Draft: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassSendMessage:
		account, item, operation, artifacts, err := manager.SendMessage(integrationsManager.List(), mail.SendMessageInput{
			Selection:      mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			To:             append([]string(nil), action.To...),
			Cc:             append([]string(nil), action.Cc...),
			Bcc:            append([]string(nil), action.Bcc...),
			Subject:        strings.TrimSpace(action.Subject),
			Body:           action.Body,
			AttachmentRefs: append([]mail.AttachmentRefInput(nil), action.AttachmentRefs...),
			Source:         source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailMessageResponse{Account: account, Message: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassSendDraft:
		account, _, item, operation, artifacts, err := manager.SendDraft(integrationsManager.List(), mail.SendDraftInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			DraftID:   strings.TrimSpace(action.DraftID),
			Source:    source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailMessageResponse{Account: account, Message: item, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassReplyMessage:
		account, draft, message, operation, artifacts, err := manager.ReplyMessage(integrationsManager.List(), mail.ReplyMessageInput{
			Selection:      mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			MessageID:      strings.TrimSpace(action.MessageID),
			ResultMode:     action.ResultMode,
			Subject:        strings.TrimSpace(action.Subject),
			Body:           action.Body,
			AttachmentRefs: append([]mail.AttachmentRefInput(nil), action.AttachmentRefs...),
			Source:         source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		if draft != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailDraftResponse{Account: account, Draft: *draft, Operation: operation, Artifacts: artifacts}}, nil
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailMessageResponse{Account: account, Message: *message, Operation: operation, Artifacts: artifacts}}, nil
	case mail.OperationClassForwardMessage:
		account, draft, message, operation, artifacts, err := manager.ForwardMessage(integrationsManager.List(), mail.ForwardMessageInput{
			Selection:      mail.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			MessageID:      strings.TrimSpace(action.MessageID),
			ResultMode:     action.ResultMode,
			To:             append([]string(nil), action.To...),
			Cc:             append([]string(nil), action.Cc...),
			Bcc:            append([]string(nil), action.Bcc...),
			Subject:        strings.TrimSpace(action.Subject),
			Body:           action.Body,
			AttachmentRefs: append([]mail.AttachmentRefInput(nil), action.AttachmentRefs...),
			Source:         source,
		})
		if err != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		if draft != nil {
			return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailDraftResponse{Account: account, Draft: *draft, Operation: operation, Artifacts: artifacts}}, nil
		}
		return mailExecutionResult{Account: account, Operation: operation, Artifacts: artifacts, Output: MailMessageResponse{Account: account, Message: *message, Operation: operation, Artifacts: artifacts}}, nil
	default:
		return mailExecutionResult{}, fmt.Errorf("unsupported mail action %q", action.OperationClass)
	}
}

func mailToolCallOutput(result mailExecutionResult) map[string]any {
	return map[string]any{
		"account":   result.Account,
		"operation": result.Operation,
		"artifacts": result.Artifacts,
		"result":    result.Output,
	}
}
