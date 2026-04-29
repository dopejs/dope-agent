package mail

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type Manager struct {
	mu         sync.RWMutex
	env        string
	backends   map[integrations.BackendKind]Backend
	accounts   map[string]AccountProjection
	operations map[string]Operation
	opOrder    []string
	artifacts  map[string]Artifact
}

func NewManager(environmentScope string) *Manager {
	return &Manager{
		env:        strings.TrimSpace(environmentScope),
		backends:   map[integrations.BackendKind]Backend{integrations.BackendKindFakeLocal: NewFakeBackend()},
		accounts:   make(map[string]AccountProjection),
		operations: make(map[string]Operation),
		artifacts:  make(map[string]Artifact),
	}
}

func (m *Manager) Restore(accounts []AccountProjection, operations []Operation, artifacts []Artifact) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts = make(map[string]AccountProjection, len(accounts))
	for _, item := range accounts {
		m.accounts[item.IntegrationID] = item
	}
	m.operations = make(map[string]Operation, len(operations))
	m.opOrder = m.opOrder[:0]
	for _, item := range operations {
		m.operations[item.OperationID] = item
		m.opOrder = append(m.opOrder, item.OperationID)
	}
	m.artifacts = make(map[string]Artifact, len(artifacts))
	threadsByIntegration := make(map[string][]ThreadSnapshot)
	messagesByIntegration := make(map[string][]MessageSnapshot)
	draftsByIntegration := make(map[string][]DraftSnapshot)
	attachmentsByIntegration := make(map[string][]AttachmentReference)
	for _, item := range artifacts {
		m.artifacts[item.ArtifactID] = item
		switch item.Kind {
		case ArtifactKindThreadSnapshot:
			if item.Thread != nil {
				threadsByIntegration[item.IntegrationID] = append(threadsByIntegration[item.IntegrationID], *item.Thread)
			}
		case ArtifactKindMessageSnapshot:
			if item.Message != nil {
				messagesByIntegration[item.IntegrationID] = append(messagesByIntegration[item.IntegrationID], *item.Message)
			}
		case ArtifactKindDraftSnapshot:
			if item.Draft != nil {
				draftsByIntegration[item.IntegrationID] = append(draftsByIntegration[item.IntegrationID], *item.Draft)
			}
		case ArtifactKindAttachmentRef:
			if item.Attachment != nil {
				attachmentsByIntegration[item.IntegrationID] = append(attachmentsByIntegration[item.IntegrationID], *item.Attachment)
			}
		}
	}
	if backend := m.backends[integrations.BackendKindFakeLocal]; backend != nil {
		for integrationID := range threadsByIntegration {
			backend.RestoreIntegrationState(integrationID, threadsByIntegration[integrationID], messagesByIntegration[integrationID], draftsByIntegration[integrationID], attachmentsByIntegration[integrationID])
		}
		for integrationID := range messagesByIntegration {
			if _, ok := threadsByIntegration[integrationID]; !ok {
				backend.RestoreIntegrationState(integrationID, nil, messagesByIntegration[integrationID], draftsByIntegration[integrationID], attachmentsByIntegration[integrationID])
			}
		}
		for integrationID := range draftsByIntegration {
			if _, ok := threadsByIntegration[integrationID]; ok {
				continue
			}
			if _, ok := messagesByIntegration[integrationID]; ok {
				continue
			}
			backend.RestoreIntegrationState(integrationID, nil, nil, draftsByIntegration[integrationID], attachmentsByIntegration[integrationID])
		}
		for integrationID := range attachmentsByIntegration {
			if _, ok := threadsByIntegration[integrationID]; ok {
				continue
			}
			if _, ok := messagesByIntegration[integrationID]; ok {
				continue
			}
			if _, ok := draftsByIntegration[integrationID]; ok {
				continue
			}
			backend.RestoreIntegrationState(integrationID, nil, nil, nil, attachmentsByIntegration[integrationID])
		}
	}
}

func (m *Manager) ListAccounts(resources []integrations.Resource, selection Selection) ([]AccountProjection, error) {
	if trimmed := strings.TrimSpace(selection.IntegrationID); trimmed != "" {
		account, _, _, _, err := m.selectAccount(resources, selection)
		if err != nil {
			return nil, err
		}
		return []AccountProjection{account}, nil
	}
	items := make([]AccountProjection, 0)
	for _, resource := range resources {
		if resource.DomainKind != "mail" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
			continue
		}
		account, _, _, _, err := m.selectAccount(resources, Selection{IntegrationID: resource.IntegrationID})
		if err != nil {
			return nil, err
		}
		items = append(items, account)
	}
	return items, nil
}

func (m *Manager) ListThreads(resources []integrations.Resource, input ListThreadsInput) (AccountProjection, []ThreadSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, nil, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassListThreads, selectionMode, summarizeListThreads(input), input.Source)
	items, err := backend.ListThreads(resource, account, input)
	if err != nil {
		return account, nil, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifacts := make([]Artifact, 0, len(items))
	for _, item := range items {
		item.OperationID = operation.OperationID
		item.IntegrationID = account.IntegrationID
		item.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, ThreadArtifact(operation, item))
	}
	operation = m.completeOperation(operation, ResultModeInspection, "", "", "", artifacts)
	return account, items, operation, artifacts, nil
}

func (m *Manager) GetThread(resources []integrations.Resource, input GetThreadInput) (AccountProjection, ThreadSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, ThreadSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassGetThread, selectionMode, input.ThreadID, input.Source)
	item, err := backend.GetThread(resource, account, input.ThreadID)
	if err != nil {
		return account, ThreadSnapshot{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifact := ThreadArtifact(operation, item)
	operation = m.completeOperation(operation, ResultModeInspection, item.ThreadID, "", "", []Artifact{artifact})
	return account, item, operation, []Artifact{artifact}, nil
}

func (m *Manager) GetMessage(resources []integrations.Resource, input GetMessageInput) (AccountProjection, MessageSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, MessageSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassGetMessage, selectionMode, input.MessageID, input.Source)
	item, err := backend.GetMessage(resource, account, input.MessageID)
	if err != nil {
		return account, MessageSnapshot{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifacts := []Artifact{MessageArtifact(operation, item)}
	for _, attachment := range backend.ResolveAttachments(resource, account, attachmentRefsFromIDs(item.AttachmentRefIDs), "message", item.MessageID) {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation = m.completeOperation(operation, ResultModeInspection, item.ThreadID, item.MessageID, "", artifacts)
	return account, item, operation, artifacts, nil
}

func (m *Manager) ListDrafts(resources []integrations.Resource, input ListDraftsInput) (AccountProjection, []DraftSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, nil, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassListDrafts, selectionMode, "", input.Source)
	items, err := backend.ListDrafts(resource, account, input)
	if err != nil {
		return account, nil, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifacts := make([]Artifact, 0, len(items))
	for _, item := range items {
		item.OperationID = operation.OperationID
		item.IntegrationID = account.IntegrationID
		item.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, DraftArtifact(operation, item))
	}
	operation = m.completeOperation(operation, ResultModeInspection, "", "", "", artifacts)
	return account, items, operation, artifacts, nil
}

func (m *Manager) GetDraft(resources []integrations.Resource, input GetDraftInput) (AccountProjection, DraftSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, DraftSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassGetDraft, selectionMode, input.DraftID, input.Source)
	item, err := backend.GetDraft(resource, account, input.DraftID)
	if err != nil {
		return account, DraftSnapshot{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifacts := []Artifact{DraftArtifact(operation, item)}
	for _, attachment := range backend.ResolveAttachments(resource, account, attachmentRefsFromIDs(item.AttachmentRefIDs), "draft", item.DraftID) {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation = m.completeOperation(operation, ResultModeInspection, item.ThreadID, "", item.DraftID, artifacts)
	return account, item, operation, artifacts, nil
}

func (m *Manager) CreateDraft(resources []integrations.Resource, input CreateDraftInput) (AccountProjection, DraftSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, DraftSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassCreateDraft, selectionMode, summarizeDraftInput(input.Subject, input.To, input.Cc, input.Bcc), input.Source)
	item, attachments, err := backend.CreateDraft(resource, account, input)
	if err != nil {
		return account, DraftSnapshot{}, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifacts := []Artifact{DraftArtifact(operation, item)}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation = m.completeOperation(operation, ResultModeDraftOnly, item.ThreadID, "", item.DraftID, artifacts)
	return account, item, operation, artifacts, nil
}

func (m *Manager) UpdateDraft(resources []integrations.Resource, input UpdateDraftInput) (AccountProjection, DraftSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, DraftSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassUpdateDraft, selectionMode, input.DraftID, input.Source)
	item, attachments, err := backend.UpdateDraft(resource, account, input)
	if err != nil {
		return account, DraftSnapshot{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifacts := []Artifact{DraftArtifact(operation, item)}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation = m.completeOperation(operation, ResultModeDraftOnly, item.ThreadID, "", item.DraftID, artifacts)
	return account, item, operation, artifacts, nil
}

func (m *Manager) SendMessage(resources []integrations.Resource, input SendMessageInput) (AccountProjection, MessageSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, MessageSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassSendMessage, selectionMode, summarizeDraftInput(input.Subject, input.To, input.Cc, input.Bcc), input.Source)
	if err := validateExplicitRecipients(joinRecipients(input.To, input.Cc, input.Bcc)); err != nil {
		return account, MessageSnapshot{}, m.blockOperation(operation, ResultModeBlocked, "recipient_required", err.Error()), nil, err
	}
	if blocked, artifacts := m.resolveAndBlockOnAttachments(operation, backend, resource, account, input.AttachmentRefs, "message", "pending_direct_send"); blocked.OperationID != "" {
		return account, MessageSnapshot{}, blocked, artifacts, ErrMailAttachmentUnresolved
	}
	if isBackgroundSend(input.Source) && !input.Source.AllowSendSideEffects {
		return account, MessageSnapshot{}, m.blockOperation(operation, ResultModeBlocked, "send_permission_required", ErrMailBackgroundSendBlocked.Error()), nil, ErrMailBackgroundSendBlocked
	}
	item, attachments, err := backend.SendMessage(resource, account, input)
	if err != nil {
		return account, MessageSnapshot{}, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	item.OperationID = operation.OperationID
	item.IntegrationID = account.IntegrationID
	item.MailAccountID = account.MailAccountID
	artifacts := []Artifact{MessageArtifact(operation, item)}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation.SendPath = SendPathDirect
	operation = m.completeOperation(operation, ResultModeSent, item.ThreadID, item.MessageID, "", artifacts)
	return account, item, operation, artifacts, nil
}

func (m *Manager) SendDraft(resources []integrations.Resource, input SendDraftInput) (AccountProjection, DraftSnapshot, MessageSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, DraftSnapshot{}, MessageSnapshot{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassSendDraft, selectionMode, input.DraftID, input.Source)
	if isBackgroundSend(input.Source) && !input.Source.AllowSendSideEffects {
		return account, DraftSnapshot{}, MessageSnapshot{}, m.blockOperation(operation, ResultModeBlocked, "send_permission_required", ErrMailBackgroundSendBlocked.Error()), nil, ErrMailBackgroundSendBlocked
	}
	draftBefore, draftErr := backend.GetDraft(resource, account, input.DraftID)
	if draftErr != nil {
		return account, DraftSnapshot{}, MessageSnapshot{}, m.failOperation(operation, "not_found", draftErr.Error()), nil, draftErr
	}
	if err := validateExplicitRecipients(draftBefore.RecipientSummary); err != nil && draftBefore.ComposeMode == ComposeModeNewMessage {
		return account, DraftSnapshot{}, MessageSnapshot{}, m.blockOperation(operation, ResultModeBlocked, "recipient_required", err.Error()), nil, err
	}
	if blocked, artifacts := m.resolveAndBlockOnAttachments(operation, backend, resource, account, attachmentRefsFromIDs(draftBefore.AttachmentRefIDs), "draft", draftBefore.DraftID); blocked.OperationID != "" {
		return account, DraftSnapshot{}, MessageSnapshot{}, blocked, artifacts, ErrMailAttachmentUnresolved
	}
	draft, message, attachments, err := backend.SendDraft(resource, account, input)
	if err != nil {
		return account, DraftSnapshot{}, MessageSnapshot{}, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	draft.OperationID = operation.OperationID
	draft.IntegrationID = account.IntegrationID
	draft.MailAccountID = account.MailAccountID
	message.OperationID = operation.OperationID
	message.IntegrationID = account.IntegrationID
	message.MailAccountID = account.MailAccountID
	artifacts := []Artifact{DraftArtifact(operation, draft), MessageArtifact(operation, message)}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	operation.SendPath = SendPathDraft
	operation = m.completeOperation(operation, ResultModeSent, message.ThreadID, message.MessageID, draft.DraftID, artifacts)
	return account, draft, message, operation, artifacts, nil
}

func (m *Manager) ReplyMessage(resources []integrations.Resource, input ReplyMessageInput) (AccountProjection, *DraftSnapshot, *MessageSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, nil, nil, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassReplyMessage, selectionMode, input.MessageID, input.Source)
	if input.ResultMode == ReplyForwardResultModeSend {
		if blocked, artifacts := m.resolveAndBlockOnAttachments(operation, backend, resource, account, input.AttachmentRefs, "message", "pending_reply_send"); blocked.OperationID != "" {
			return account, nil, nil, blocked, artifacts, ErrMailAttachmentUnresolved
		}
		if isBackgroundSend(input.Source) && !input.Source.AllowSendSideEffects {
			return account, nil, nil, m.blockOperation(operation, ResultModeBlocked, "send_permission_required", ErrMailBackgroundSendBlocked.Error()), nil, ErrMailBackgroundSendBlocked
		}
	}
	draft, message, attachments, err := backend.ReplyMessage(resource, account, input)
	if err != nil {
		return account, nil, nil, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifacts := make([]Artifact, 0, 1+len(attachments))
	resultMode := ResultModeDraftOnly
	threadID := ""
	messageID := ""
	draftID := ""
	if draft != nil {
		draft.OperationID = operation.OperationID
		draft.IntegrationID = account.IntegrationID
		draft.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, DraftArtifact(operation, *draft))
		threadID = draft.ThreadID
		draftID = draft.DraftID
	}
	if message != nil {
		resultMode = ResultModeSent
		message.OperationID = operation.OperationID
		message.IntegrationID = account.IntegrationID
		message.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, MessageArtifact(operation, *message))
		threadID = message.ThreadID
		messageID = message.MessageID
	}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	if resultMode == ResultModeSent {
		operation.SendPath = SendPathDirect
	}
	operation = m.completeOperation(operation, resultMode, threadID, messageID, draftID, artifacts)
	return account, draft, message, operation, artifacts, nil
}

func (m *Manager) ForwardMessage(resources []integrations.Resource, input ForwardMessageInput) (AccountProjection, *DraftSnapshot, *MessageSnapshot, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, nil, nil, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassForwardMessage, selectionMode, input.MessageID, input.Source)
	if input.ResultMode == ReplyForwardResultModeSend {
		if err := validateExplicitRecipients(joinRecipients(input.To, input.Cc, input.Bcc)); err != nil {
			return account, nil, nil, m.blockOperation(operation, ResultModeBlocked, "recipient_required", err.Error()), nil, err
		}
		if blocked, artifacts := m.resolveAndBlockOnAttachments(operation, backend, resource, account, input.AttachmentRefs, "message", "pending_forward_send"); blocked.OperationID != "" {
			return account, nil, nil, blocked, artifacts, ErrMailAttachmentUnresolved
		}
		if isBackgroundSend(input.Source) && !input.Source.AllowSendSideEffects {
			return account, nil, nil, m.blockOperation(operation, ResultModeBlocked, "send_permission_required", ErrMailBackgroundSendBlocked.Error()), nil, ErrMailBackgroundSendBlocked
		}
	}
	draft, message, attachments, err := backend.ForwardMessage(resource, account, input)
	if err != nil {
		return account, nil, nil, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifacts := make([]Artifact, 0, 1+len(attachments))
	resultMode := ResultModeDraftOnly
	threadID := ""
	messageID := ""
	draftID := ""
	if draft != nil {
		draft.OperationID = operation.OperationID
		draft.IntegrationID = account.IntegrationID
		draft.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, DraftArtifact(operation, *draft))
		threadID = draft.ThreadID
		draftID = draft.DraftID
	}
	if message != nil {
		resultMode = ResultModeSent
		message.OperationID = operation.OperationID
		message.IntegrationID = account.IntegrationID
		message.MailAccountID = account.MailAccountID
		artifacts = append(artifacts, MessageArtifact(operation, *message))
		threadID = message.ThreadID
		messageID = message.MessageID
	}
	for _, attachment := range attachments {
		attachment.OperationID = operation.OperationID
		artifacts = append(artifacts, AttachmentArtifact(operation, attachment))
	}
	if resultMode == ResultModeSent {
		operation.SendPath = SendPathDirect
	}
	operation = m.completeOperation(operation, resultMode, threadID, messageID, draftID, artifacts)
	return account, draft, message, operation, artifacts, nil
}

func (m *Manager) ListOperations(filter OperationFilter) []Operation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Operation, 0)
	for _, id := range m.opOrder {
		item := m.operations[id]
		if filter.IntegrationID != "" && item.IntegrationID != filter.IntegrationID {
			continue
		}
		if filter.RunID != "" && item.RunID != filter.RunID {
			continue
		}
		if filter.WorkflowID != "" && item.WorkflowID != filter.WorkflowID {
			continue
		}
		if filter.ScheduleID != "" && item.ScheduleID != filter.ScheduleID {
			continue
		}
		if filter.DeliveryID != "" && item.DeliveryID != filter.DeliveryID {
			continue
		}
		if filter.OperationClass != "" && item.OperationClass != filter.OperationClass {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.ResultMode != "" && item.ResultMode != filter.ResultMode {
			continue
		}
		if filter.ThreadID != "" && item.ThreadID != filter.ThreadID {
			continue
		}
		if filter.MessageID != "" && item.MessageID != filter.MessageID {
			continue
		}
		if filter.DraftID != "" && item.DraftID != filter.DraftID {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) GetOperation(operationID string) (Operation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.operations[strings.TrimSpace(operationID)]
	return item, ok
}

func (m *Manager) GetAccount(integrationID string) (AccountProjection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.accounts[strings.TrimSpace(integrationID)]
	return item, ok
}

func (m *Manager) StoreOperation(item Operation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.operations[item.OperationID]; !exists {
		m.opOrder = append(m.opOrder, item.OperationID)
	}
	m.operations[item.OperationID] = item
}

func (m *Manager) ListArtifacts(operationID string) []Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Artifact, 0)
	for _, item := range m.artifacts {
		if operationID != "" && item.OperationID != operationID {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) selectAccount(resources []integrations.Resource, selection Selection) (AccountProjection, integrations.Resource, Backend, string, error) {
	explicit := strings.TrimSpace(selection.IntegrationID)
	if explicit != "" {
		for _, resource := range resources {
			if resource.IntegrationID != explicit {
				continue
			}
			return m.projectResource(resource, "explicit")
		}
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrMailIntegrationNotFound
	}
	var selected *integrations.Resource
	for idx := range resources {
		resource := resources[idx]
		if resource.DomainKind != "mail" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
			continue
		}
		if resource.CanonicalDefault {
			selected = &resource
			break
		}
	}
	if selected == nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrMailSelectionInvalid
	}
	return m.projectResource(*selected, "canonical_default")
}

func (m *Manager) projectResource(resource integrations.Resource, selectionMode string) (AccountProjection, integrations.Resource, Backend, string, error) {
	if resource.DomainKind != "mail" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrMailSelectionInvalid
	}
	if resource.ReadinessStatus != integrations.ReadinessStatusHealthy && resource.ReadinessStatus != integrations.ReadinessStatusDegraded {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrMailUnavailable
	}
	backend := m.backends[resource.BackendBinding.BackendKind]
	if backend == nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", errors.New("mail backend is not configured")
	}
	if !backend.SupportsResource(resource) {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrMailUnavailable
	}
	account, err := backend.ProjectAccount(resource)
	if err != nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", err
	}
	account.SelectionMode = selectionMode
	m.mu.Lock()
	m.accounts[account.IntegrationID] = account
	m.mu.Unlock()
	return account, resource, backend, selectionMode, nil
}

func (m *Manager) newOperation(account AccountProjection, resource integrations.Resource, class OperationClass, selectionMode, summary string, source SourceLinkage) Operation {
	now := time.Now().UTC()
	operationID := strings.TrimSpace(source.OperationID)
	if operationID == "" {
		operationID = NewOperationID()
	}
	operation := Operation{
		OperationID:             operationID,
		OperationClass:          class,
		Status:                  OperationStatusRequested,
		ResultMode:              ResultModeInspection,
		IntegrationID:           resource.IntegrationID,
		MailAccountID:           account.MailAccountID,
		EnvironmentScope:        account.EnvironmentScope,
		SelectionMode:           selectionMode,
		RequestSummary:          strings.TrimSpace(summary),
		BackgroundSendPermitted: source.AllowSendSideEffects,
		RunID:                   source.RunID,
		StepID:                  source.StepID,
		ToolCallID:              source.ToolCallID,
		WorkflowID:              source.WorkflowID,
		WorkflowStepID:          source.WorkflowStepID,
		ScheduleID:              source.ScheduleID,
		ScheduleAttemptID:       source.ScheduleAttemptID,
		DeliveryID:              source.DeliveryID,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	m.StoreOperation(operation)
	return operation
}

func (m *Manager) completeOperation(operation Operation, resultMode ResultMode, threadID, messageID, draftID string, artifacts []Artifact) Operation {
	now := time.Now().UTC()
	operation.Status = OperationStatusCompleted
	operation.ResultMode = resultMode
	operation.ThreadID = strings.TrimSpace(threadID)
	operation.MessageID = strings.TrimSpace(messageID)
	operation.DraftID = strings.TrimSpace(draftID)
	operation.CompletedAt = &now
	operation.UpdatedAt = now
	operation.ArtifactIDs = collectArtifactIDs(artifacts)
	m.mu.Lock()
	for _, artifact := range artifacts {
		m.artifacts[artifact.ArtifactID] = artifact
	}
	if _, exists := m.operations[operation.OperationID]; !exists {
		m.opOrder = append(m.opOrder, operation.OperationID)
	}
	m.operations[operation.OperationID] = operation
	m.mu.Unlock()
	return operation
}

func (m *Manager) failOperation(operation Operation, failureClass, reason string) Operation {
	now := time.Now().UTC()
	operation.Status = OperationStatusFailed
	operation.ResultMode = ResultModeFailed
	operation.FailureClass = strings.TrimSpace(failureClass)
	operation.FailureReason = strings.TrimSpace(reason)
	operation.CompletedAt = &now
	operation.UpdatedAt = now
	m.StoreOperation(operation)
	return operation
}

func (m *Manager) blockOperation(operation Operation, resultMode ResultMode, failureClass, reason string) Operation {
	now := time.Now().UTC()
	operation.Status = OperationStatusBlocked
	operation.ResultMode = resultMode
	operation.FailureClass = strings.TrimSpace(failureClass)
	operation.FailureReason = strings.TrimSpace(reason)
	operation.CompletedAt = &now
	operation.UpdatedAt = now
	m.StoreOperation(operation)
	return operation
}

func (m *Manager) resolveAndBlockOnAttachments(operation Operation, backend Backend, resource integrations.Resource, account AccountProjection, refs []AttachmentRefInput, parentKind, parentID string) (Operation, []Artifact) {
	attachments := backend.ResolveAttachments(resource, account, refs, parentKind, parentID)
	for _, attachment := range attachments {
		if attachment.ResolutionStatus == AttachmentResolutionResolved {
			continue
		}
		attachment.OperationID = operation.OperationID
		artifacts := []Artifact{AttachmentArtifact(operation, attachment)}
		blocked := m.blockOperation(operation, ResultModeBlocked, "attachment_unresolved", firstNonEmpty(attachment.FailureReason, ErrMailAttachmentUnresolved.Error()))
		blocked.ArtifactIDs = collectArtifactIDs(artifacts)
		m.mu.Lock()
		for _, artifact := range artifacts {
			m.artifacts[artifact.ArtifactID] = artifact
		}
		m.operations[blocked.OperationID] = blocked
		m.mu.Unlock()
		return blocked, artifacts
	}
	return Operation{}, nil
}

func summarizeListThreads(input ListThreadsInput) string {
	if input.Limit > 0 {
		return fmt.Sprintf("limit=%d", input.Limit)
	}
	return ""
}

func summarizeDraftInput(subject string, to, cc, bcc []string) string {
	recipients := joinRecipients(to, cc, bcc)
	if len(recipients) == 0 {
		return strings.TrimSpace(subject)
	}
	return fmt.Sprintf("%s -> %s", strings.TrimSpace(subject), strings.Join(recipients, ", "))
}

func attachmentRefsFromIDs(ids []string) []AttachmentRefInput {
	if len(ids) == 0 {
		return nil
	}
	items := make([]AttachmentRefInput, 0, len(ids))
	for _, id := range ids {
		items = append(items, AttachmentRefInput{AttachmentRefID: id})
	}
	return items
}

func validateExplicitRecipients(recipients []string) error {
	if len(joinRecipients(recipients)) == 0 {
		return ErrMailRecipientRequired
	}
	return nil
}

func isBackgroundSend(source SourceLinkage) bool {
	return strings.TrimSpace(source.WorkflowID) != "" || strings.TrimSpace(source.ScheduleID) != "" || strings.TrimSpace(source.ScheduleAttemptID) != ""
}

func collectArtifactIDs(artifacts []Artifact) []string {
	if len(artifacts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		ids = append(ids, item.ArtifactID)
	}
	return ids
}

func NewOperationID() string {
	return newID("mail_op")
}

func newID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
