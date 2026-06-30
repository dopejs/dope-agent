package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleMailAccounts(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListAccounts(integrationsManager.List(), mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))})
	if err != nil {
		writeMailError(w, err)
		return
	}
	if err := recordMailAccounts(r.Context(), eventBus, sqliteStore, items); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MailAccountListResponse{Items: items})
}

func handleMailAccountRoutes(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	integrationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/mail/accounts/"))
	if integrationID == "" {
		http.NotFound(w, r)
		return
	}
	items, err := manager.ListAccounts(integrationsManager.List(), mail.Selection{IntegrationID: integrationID})
	if err != nil {
		writeMailError(w, err)
		return
	}
	if len(items) == 0 {
		http.NotFound(w, r)
		return
	}
	if err := recordMailAccounts(r.Context(), eventBus, sqliteStore, items[:1]); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items[0])
}

func handleMailThreads(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "GET /v1/mail/threads", w, r)
	if !ok {
		return
	}
	account, items, operation, artifacts, err := manager.ListThreads(integrationsManager.List(), mail.ListThreadsInput{
		Selection: mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		Limit:     limit,
		Cursor:    strings.TrimSpace(r.URL.Query().Get("cursor")),
		Source:    mail.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailThreadListResponse{
		Account:   account,
		Items:     items,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailThreadRoutes(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	threadID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/mail/threads/"))
	if threadID == "" {
		http.NotFound(w, r)
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "GET /v1/mail/threads/{threadId}", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.GetThread(integrationsManager.List(), mail.GetThreadInput{
		Selection: mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		ThreadID:  threadID,
		Source:    mail.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailThreadResponse{
		Account:   account,
		Thread:    item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailMessageRoutes(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/mail/messages/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		handleMailMessageGet(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	case len(parts) == 2 && parts[1] == "reply" && r.Method == http.MethodPost:
		handleMailReplyMessage(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	case len(parts) == 2 && parts[1] == "forward" && r.Method == http.MethodPost:
		handleMailForwardMessage(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	default:
		http.NotFound(w, r)
	}
}

func handleMailMessageGet(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, messageID string) {
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "GET /v1/mail/messages/{messageId}", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.GetMessage(integrationsManager.List(), mail.GetMessageInput{
		Selection: mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		MessageID: strings.TrimSpace(messageID),
		Source:    mail.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailMessageResponse{
		Account:   account,
		Message:   item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailDrafts(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		operationID := mail.NewOperationID()
		reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "GET /v1/mail/drafts", w, r)
		if !ok {
			return
		}
		account, items, operation, artifacts, err := manager.ListDrafts(integrationsManager.List(), mail.ListDraftsInput{
			Selection: mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
			Source:    mail.SourceLinkage{OperationID: operationID},
		})
		if operation.OperationID != "" {
			if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
				writeError(w, http.StatusInternalServerError, recordErr.Error())
				return
			}
		}
		if err != nil {
			if operation.OperationID == "" {
				releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
			}
			writeMailError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, MailDraftListResponse{
			Account:   account,
			Items:     items,
			Operation: operation,
			Artifacts: artifacts,
		})
	case http.MethodPost:
		var request CreateMailDraftRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		operationID := mail.NewOperationID()
		reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/drafts", w, r)
		if !ok {
			return
		}
		account, item, operation, artifacts, err := manager.CreateDraft(integrationsManager.List(), mail.CreateDraftInput{
			Selection:       mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
			ComposeMode:     request.ComposeMode,
			ThreadID:        strings.TrimSpace(request.ThreadID),
			SourceMessageID: strings.TrimSpace(request.SourceMessageID),
			To:              append([]string(nil), request.To...),
			Cc:              append([]string(nil), request.Cc...),
			Bcc:             append([]string(nil), request.Bcc...),
			Subject:         strings.TrimSpace(request.Subject),
			Body:            request.Body,
			AttachmentRefs:  mailAttachmentInputs(request.AttachmentRefs),
			Source:          mailSourceLinkageWithOperation(request.Source, operationID),
		})
		if operation.OperationID != "" {
			if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
				writeError(w, http.StatusInternalServerError, recordErr.Error())
				return
			}
		}
		if err != nil {
			if operation.OperationID == "" {
				releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
			}
			writeMailError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, MailDraftResponse{
			Account:   account,
			Draft:     item,
			Operation: operation,
			Artifacts: artifacts,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleMailDraftRoutes(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/mail/drafts/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		handleMailDraftGet(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	case len(parts) == 2 && parts[1] == "update" && r.Method == http.MethodPost:
		handleMailDraftUpdate(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	case len(parts) == 2 && parts[1] == "send" && r.Method == http.MethodPost:
		handleMailDraftSend(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
	default:
		http.NotFound(w, r)
	}
}

func handleMailDraftGet(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, draftID string) {
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "GET /v1/mail/drafts/{draftId}", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.GetDraft(integrationsManager.List(), mail.GetDraftInput{
		Selection: mail.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		DraftID:   strings.TrimSpace(draftID),
		Source:    mail.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailDraftResponse{
		Account:   account,
		Draft:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailDraftUpdate(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, draftID string) {
	var request UpdateMailDraftRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/drafts/{draftId}/update", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.UpdateDraft(integrationsManager.List(), mail.UpdateDraftInput{
		Selection:      mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		DraftID:        strings.TrimSpace(draftID),
		To:             append([]string(nil), request.To...),
		Cc:             append([]string(nil), request.Cc...),
		Bcc:            append([]string(nil), request.Bcc...),
		Subject:        strings.TrimSpace(request.Subject),
		Body:           request.Body,
		AttachmentRefs: mailAttachmentInputs(request.AttachmentRefs),
		Source:         mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailDraftResponse{
		Account:   account,
		Draft:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

// handleMailAttachmentRoutes serves /v1/mail/attachments/{attachmentRefId}/download (Roadmap 64).
func handleMailAttachmentRoutes(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/mail/attachments/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "download" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	attachmentRefID := strings.TrimSpace(parts[0])
	var request DownloadMailAttachmentRequest
	if err := decodeJSONBody(r, &request); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) && !strings.Contains(err.Error(), "EOF") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/attachments/{attachmentRefId}/download", w, r)
	if !ok {
		return
	}
	account, attachment, operation, artifacts, err := manager.DownloadAttachment(integrationsManager.List(), mail.DownloadAttachmentInput{
		Selection:       mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		MessageID:       strings.TrimSpace(request.MessageID),
		AttachmentRefID: attachmentRefID,
		DisplayName:     strings.TrimSpace(request.DisplayName),
		MediaType:       strings.TrimSpace(request.MediaType),
		SizeBytes:       request.SizeBytes,
		Source:          mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailAttachmentResponse{
		Account:    account,
		Attachment: attachment,
		Operation:  operation,
		Artifacts:  artifacts,
	})
}

func handleMailSendMessage(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "mail dependencies are not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request SendMailMessageRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/messages/send", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.SendMessage(integrationsManager.List(), mail.SendMessageInput{
		Selection:      mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		To:             append([]string(nil), request.To...),
		Cc:             append([]string(nil), request.Cc...),
		Bcc:            append([]string(nil), request.Bcc...),
		Subject:        strings.TrimSpace(request.Subject),
		Body:           request.Body,
		AttachmentRefs: mailAttachmentInputs(request.AttachmentRefs),
		Source:         mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, MailMessageResponse{
		Account:   account,
		Message:   item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailDraftSend(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, draftID string) {
	var request SendMailDraftRequest
	if err := decodeJSONBody(r, &request); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) && !strings.Contains(err.Error(), "EOF") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/drafts/{draftId}/send", w, r)
	if !ok {
		return
	}
	account, _, message, operation, artifacts, err := manager.SendDraft(integrationsManager.List(), mail.SendDraftInput{
		Selection: mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		DraftID:   strings.TrimSpace(draftID),
		Source:    mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, MailMessageResponse{
		Account:   account,
		Message:   message,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleMailReplyMessage(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, messageID string) {
	var request ReplyMailMessageRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/messages/{messageId}/reply", w, r)
	if !ok {
		return
	}
	account, draft, message, operation, artifacts, err := manager.ReplyMessage(integrationsManager.List(), mail.ReplyMessageInput{
		Selection:      mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		MessageID:      strings.TrimSpace(messageID),
		ResultMode:     request.ResultMode,
		Subject:        strings.TrimSpace(request.Subject),
		Body:           request.Body,
		AttachmentRefs: mailAttachmentInputs(request.AttachmentRefs),
		Source:         mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	if draft != nil {
		writeJSON(w, http.StatusOK, MailDraftResponse{Account: account, Draft: *draft, Operation: operation, Artifacts: artifacts})
		return
	}
	writeJSON(w, http.StatusOK, MailMessageResponse{Account: account, Message: *message, Operation: operation, Artifacts: artifacts})
}

func handleMailForwardMessage(cfg config.Config, manager *mail.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, messageID string) {
	var request ForwardMailMessageRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	operationID := mail.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "mail", operationID, "POST /v1/mail/messages/{messageId}/forward", w, r)
	if !ok {
		return
	}
	account, draft, message, operation, artifacts, err := manager.ForwardMessage(integrationsManager.List(), mail.ForwardMessageInput{
		Selection:      mail.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		MessageID:      strings.TrimSpace(messageID),
		ResultMode:     request.ResultMode,
		To:             append([]string(nil), request.To...),
		Cc:             append([]string(nil), request.Cc...),
		Bcc:            append([]string(nil), request.Bcc...),
		Subject:        strings.TrimSpace(request.Subject),
		Body:           request.Body,
		AttachmentRefs: mailAttachmentInputs(request.AttachmentRefs),
		Source:         mailSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordMailActivityAndCommitQuota(r.Context(), eventBus, billingManager, reservation, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "mail operation failed before backend attempt")
		}
		writeMailError(w, err)
		return
	}
	if draft != nil {
		writeJSON(w, http.StatusOK, MailDraftResponse{Account: account, Draft: *draft, Operation: operation, Artifacts: artifacts})
		return
	}
	writeJSON(w, http.StatusOK, MailMessageResponse{Account: account, Message: *message, Operation: operation, Artifacts: artifacts})
}

func handleMailOperations(cfg config.Config, manager *mail.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "mail manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	filter := mail.OperationFilter{
		IntegrationID:  strings.TrimSpace(r.URL.Query().Get("integrationId")),
		RunID:          strings.TrimSpace(r.URL.Query().Get("runId")),
		WorkflowID:     strings.TrimSpace(r.URL.Query().Get("workflowId")),
		ScheduleID:     strings.TrimSpace(r.URL.Query().Get("scheduleId")),
		DeliveryID:     strings.TrimSpace(r.URL.Query().Get("deliveryId")),
		OperationClass: mail.OperationClass(strings.TrimSpace(r.URL.Query().Get("operationClass"))),
		Status:         mail.OperationStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		ResultMode:     mail.ResultMode(strings.TrimSpace(r.URL.Query().Get("resultMode"))),
		ThreadID:       strings.TrimSpace(r.URL.Query().Get("threadId")),
		MessageID:      strings.TrimSpace(r.URL.Query().Get("messageId")),
		DraftID:        strings.TrimSpace(r.URL.Query().Get("draftId")),
	}
	writeJSON(w, http.StatusOK, MailOperationListResponse{Items: manager.ListOperations(filter)})
}

func handleMailOperationRoutes(cfg config.Config, manager *mail.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "mail manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	operationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/mail/operations/"))
	if operationID == "" {
		http.NotFound(w, r)
		return
	}
	operation, ok := manager.GetOperation(operationID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, MailOperationResponse{Operation: operation, Artifacts: manager.ListArtifacts(operationID)})
}

func recordMailAccounts(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, items []mail.AccountProjection) error {
	for _, item := range items {
		if err := persistMailAccount(ctx, sqliteStore, item); err != nil {
			return err
		}
		if err := publishMailAccountProjected(ctx, eventBus, sqliteStore, item); err != nil {
			return err
		}
	}
	return nil
}

func recordMailActivity(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, account mail.AccountProjection, operation mail.Operation, artifacts []mail.Artifact) error {
	if account.IntegrationID != "" {
		if err := persistMailAccount(ctx, sqliteStore, account); err != nil {
			return err
		}
		if err := publishMailAccountProjected(ctx, eventBus, sqliteStore, account); err != nil {
			return err
		}
	}
	if operation.OperationID == "" {
		return nil
	}
	if err := persistMailOperation(ctx, sqliteStore, operation); err != nil {
		return err
	}
	if err := publishMailOperationRequested(ctx, eventBus, sqliteStore, operation); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := persistMailArtifact(ctx, sqliteStore, artifact); err != nil {
			return err
		}
		if err := publishMailArtifactRecorded(ctx, eventBus, sqliteStore, artifact, operation); err != nil {
			return err
		}
	}
	switch operation.Status {
	case mail.OperationStatusCompleted:
		return publishMailOperationCompleted(ctx, eventBus, sqliteStore, operation)
	case mail.OperationStatusFailed, mail.OperationStatusBlocked, mail.OperationStatusCancelled:
		return publishMailOperationFailed(ctx, eventBus, sqliteStore, operation)
	default:
		return nil
	}
}

func recordMailActivityAndCommitQuota(ctx context.Context, eventBus *events.Bus, billingManager *billing.Manager, reservation billing.UsageReservation, sqliteStore *store.SQLiteStore, account mail.AccountProjection, operation mail.Operation, artifacts []mail.Artifact) error {
	if operation.OperationID == "" {
		return nil
	}
	if err := recordMailActivity(ctx, eventBus, sqliteStore, account, operation, artifacts); err != nil {
		return err
	}
	return commitBillingReservation(ctx, billingManager, reservation, "billing.integration_operation_committed", "mail operation recorded after backend attempt")
}

func persistMailAccount(ctx context.Context, sqliteStore *store.SQLiteStore, item mail.AccountProjection) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertMailAccount(ctx, item)
}

func persistMailOperation(ctx context.Context, sqliteStore *store.SQLiteStore, item mail.Operation) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertMailOperation(ctx, item)
}

func persistMailArtifact(ctx context.Context, sqliteStore *store.SQLiteStore, item mail.Artifact) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertMailArtifact(ctx, item)
}

func publishMailAccountProjected(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, account mail.AccountProjection) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "mail",
		Name:     "mail.account_projected",
		Resource: events.Resource{Kind: "mail_account", ID: account.MailAccountID},
		Payload: map[string]any{
			"integrationId":    account.IntegrationID,
			"accountKey":       account.AccountKey,
			"mailboxAddress":   account.MailboxAddress,
			"readinessStatus":  account.ReadinessStatus,
			"canonicalDefault": account.CanonicalDefault,
		},
	})
	return err
}

func publishMailOperationRequested(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation mail.Operation) error {
	return publishMailOperationEvent(ctx, eventBus, sqliteStore, "mail.operation_requested", operation)
}

func publishMailOperationCompleted(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation mail.Operation) error {
	return publishMailOperationEvent(ctx, eventBus, sqliteStore, "mail.operation_completed", operation)
}

func publishMailOperationFailed(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation mail.Operation) error {
	return publishMailOperationEvent(ctx, eventBus, sqliteStore, "mail.operation_failed", operation)
}

func publishMailOperationEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, operation mail.Operation) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "mail",
		Name:     name,
		Resource: events.Resource{Kind: "mail_operation", ID: operation.OperationID},
		Payload: map[string]any{
			"operationId":    operation.OperationID,
			"operationClass": operation.OperationClass,
			"integrationId":  operation.IntegrationID,
			"runId":          operation.RunID,
			"workflowId":     operation.WorkflowID,
			"scheduleId":     operation.ScheduleID,
			"resultMode":     operation.ResultMode,
			"sendPath":       operation.SendPath,
			"threadId":       operation.ThreadID,
			"messageId":      operation.MessageID,
			"draftId":        operation.DraftID,
			"failureClass":   operation.FailureClass,
		},
	})
	return err
}

func publishMailArtifactRecorded(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, artifact mail.Artifact, operation mail.Operation) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "mail",
		Name:     "mail.artifact_recorded",
		Resource: events.Resource{Kind: "mail_artifact", ID: artifact.ArtifactID},
		Payload: map[string]any{
			"artifactId":      artifact.ArtifactID,
			"operationId":     operation.OperationID,
			"threadId":        artifact.ThreadID,
			"messageId":       artifact.MessageID,
			"draftId":         artifact.DraftID,
			"attachmentRefId": artifact.AttachmentRefID,
		},
	})
	return err
}

func mailSourceLinkage(source *MailSourceLinkageRequest) mail.SourceLinkage {
	if source == nil {
		return mail.SourceLinkage{}
	}
	return mail.SourceLinkage{
		RunID:                strings.TrimSpace(source.RunID),
		StepID:               strings.TrimSpace(source.StepID),
		ToolCallID:           strings.TrimSpace(source.ToolCallID),
		WorkflowID:           strings.TrimSpace(source.WorkflowID),
		WorkflowStepID:       strings.TrimSpace(source.WorkflowStepID),
		ScheduleID:           strings.TrimSpace(source.ScheduleID),
		ScheduleAttemptID:    strings.TrimSpace(source.ScheduleAttemptID),
		DeliveryID:           strings.TrimSpace(source.DeliveryID),
		AllowSendSideEffects: source.AllowSendSideEffects,
	}
}

func mailSourceLinkageWithOperation(source *MailSourceLinkageRequest, operationID string) mail.SourceLinkage {
	linkage := mailSourceLinkage(source)
	linkage.OperationID = strings.TrimSpace(operationID)
	return linkage
}

func mailAttachmentInputs(items []MailAttachmentRefRequest) []mail.AttachmentRefInput {
	if len(items) == 0 {
		return nil
	}
	out := make([]mail.AttachmentRefInput, 0, len(items))
	for _, item := range items {
		out = append(out, mail.AttachmentRefInput{
			AttachmentRefID: strings.TrimSpace(item.AttachmentRefID),
			DisplayName:     strings.TrimSpace(item.DisplayName),
			MediaType:       strings.TrimSpace(item.MediaType),
			SizeBytes:       item.SizeBytes,
		})
	}
	return out
}

func writeMailError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mail.ErrMailIntegrationNotFound), errors.Is(err, mail.ErrMailThreadNotFound), errors.Is(err, mail.ErrMailMessageNotFound), errors.Is(err, mail.ErrMailDraftNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, mail.ErrMailRecipientRequired), errors.Is(err, mail.ErrMailAttachmentUnresolved), errors.Is(err, mail.ErrMailBackgroundSendBlocked), errors.Is(err, mail.ErrMailSelectionInvalid), errors.Is(err, mail.ErrMailUnavailable):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
