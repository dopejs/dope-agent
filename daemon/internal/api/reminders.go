package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/reminders"
)

func handleReminders(manager *reminders.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "reminders are not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := manager.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ReminderListResponse{Items: items})
	case http.MethodPost:
		var request CreateReminderRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := buildCreateReminderInput(request)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := manager.Create(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleReminderRoutes(manager *reminders.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "reminders are not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/reminders/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(path, "occurrences") {
		handleReminderOccurrenceRoutes(manager, w, r, path)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleReminderByID(manager, w, r, parts[0])
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	switch parts[1] {
	case "acknowledge":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Acknowledge(ctxRequest.Context(), parts[0], input)
		})
	case "snooze":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Snooze(ctxRequest.Context(), parts[0], input)
		})
	case "complete":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Complete(ctxRequest.Context(), parts[0], input)
		})
	case "dismiss":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Dismiss(ctxRequest.Context(), parts[0], input)
		})
	case "reschedule":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Reschedule(ctxRequest.Context(), parts[0], input)
		})
	case "cancel":
		handleReminderTransition(manager, w, r, func(ctxRequest *http.Request, input reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error) {
			return manager.Cancel(ctxRequest.Context(), parts[0], input)
		})
	case "actions":
		handleReminderActions(manager, w, r, parts[0])
	default:
		http.NotFound(w, r)
	}
}

func handleReminderByID(manager *reminders.Manager, w http.ResponseWriter, r *http.Request, reminderID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := manager.Get(r.Context(), reminderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleReminderActions(manager *reminders.Manager, w http.ResponseWriter, r *http.Request, reminderID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListActions(r.Context(), reminderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReminderActionListResponse{Items: items})
}

func handleReminderOccurrenceRoutes(manager *reminders.Manager, w http.ResponseWriter, r *http.Request, path string) {
	path = strings.TrimPrefix(path, "occurrences")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		handleReminderOccurrences(manager, w, r)
		return
	}
	if strings.Contains(path, "/") {
		http.NotFound(w, r)
		return
	}
	handleReminderOccurrenceByID(manager, w, r, path)
}

func handleReminderOccurrences(manager *reminders.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	filter, err := reminderOccurrenceFilterFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := manager.ListOccurrences(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReminderOccurrenceListResponse{Items: items})
}

func handleReminderOccurrenceByID(manager *reminders.Manager, w http.ResponseWriter, r *http.Request, occurrenceID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := manager.GetOccurrence(r.Context(), occurrenceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type reminderTransitionHandler func(*http.Request, reminders.TransitionInput) (reminders.Reminder, reminders.Occurrence, reminders.ActionRecord, error)

func handleReminderTransition(manager *reminders.Manager, w http.ResponseWriter, r *http.Request, handler reminderTransitionHandler) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request ReminderTransitionRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input, err := buildReminderTransitionInput(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, occurrence, action, err := handler(r, input)
	if err != nil {
		switch {
		case errors.Is(err, reminders.ErrReminderNotFound), errors.Is(err, reminders.ErrReminderOccurrenceNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reminder":   item,
		"occurrence": occurrence,
		"action":     action,
	})
}

func buildCreateReminderInput(request CreateReminderRequest) (reminders.CreateInput, error) {
	trigger, err := scheduleTriggerFromRequest(request.Trigger)
	if err != nil {
		return reminders.CreateInput{}, err
	}
	behaviorMode := request.BehaviorMode
	if behaviorMode == "" {
		behaviorMode = reminders.BehaviorModeNotifyOnly
	}
	var workflowLaunchConfig *reminders.WorkflowLaunchConfig
	if request.WorkflowLaunchConfig != nil {
		calendarAction, err := buildCalendarAction(request.WorkflowLaunchConfig.CalendarAction)
		if err != nil {
			return reminders.CreateInput{}, err
		}
		mailAction, err := buildMailAction(request.WorkflowLaunchConfig.MailAction)
		if err != nil {
			return reminders.CreateInput{}, err
		}
		workflowLaunchConfig = &reminders.WorkflowLaunchConfig{
			SessionID:      request.WorkflowLaunchConfig.SessionID,
			Entrypoint:     request.WorkflowLaunchConfig.Entrypoint,
			RunGoal:        request.WorkflowLaunchConfig.RunGoal,
			WorkflowGoal:   request.WorkflowLaunchConfig.WorkflowGoal,
			CalendarAction: calendarAction,
			MailAction:     mailAction,
		}
	}

	var followUpLink *reminders.FollowUpLink
	if request.FollowUpLink != nil {
		followUpLink = &reminders.FollowUpLink{
			LinkKind:           request.FollowUpLink.LinkKind,
			SourceID:           strings.TrimSpace(request.FollowUpLink.SourceID),
			EnvironmentScope:   strings.TrimSpace(request.FollowUpLink.EnvironmentScope),
			SourceSummary:      strings.TrimSpace(request.FollowUpLink.SourceSummary),
			SourceDisplayState: strings.TrimSpace(request.FollowUpLink.SourceDisplayState),
		}
	}

	return reminders.CreateInput{
		Title:                strings.TrimSpace(request.Title),
		Details:              strings.TrimSpace(request.Details),
		BehaviorMode:         behaviorMode,
		Trigger:              trigger,
		WorkflowLaunchConfig: workflowLaunchConfig,
		FollowUpLink:         followUpLink,
	}, nil
}

func buildReminderTransitionInput(request ReminderTransitionRequest) (reminders.TransitionInput, error) {
	input := reminders.TransitionInput{
		OccurrenceID: strings.TrimSpace(request.OccurrenceID),
		Reason:       strings.TrimSpace(request.Reason),
		ActorKind:    request.ActorKind,
	}
	if strings.TrimSpace(request.SnoozedUntil) != "" {
		snoozedUntil, err := time.Parse(time.RFC3339, strings.TrimSpace(request.SnoozedUntil))
		if err != nil {
			return reminders.TransitionInput{}, err
		}
		input.SnoozedUntil = &snoozedUntil
	}
	if request.Trigger != nil {
		trigger, err := scheduleTriggerFromRequest(*request.Trigger)
		if err != nil {
			return reminders.TransitionInput{}, err
		}
		input.Trigger = &trigger
	}
	return input, nil
}

func reminderOccurrenceFilterFromRequest(r *http.Request) (reminders.OccurrenceFilter, error) {
	query := r.URL.Query()
	filter := reminders.OccurrenceFilter{
		ReminderID: strings.TrimSpace(query.Get("reminderId")),
		RunID:      strings.TrimSpace(query.Get("runId")),
		WorkflowID: strings.TrimSpace(query.Get("workflowId")),
		DeliveryID: strings.TrimSpace(query.Get("deliveryId")),
	}
	if state := strings.TrimSpace(query.Get("state")); state != "" {
		filter.State = reminders.State(state)
	}
	if value := strings.TrimSpace(query.Get("scheduledBefore")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return reminders.OccurrenceFilter{}, err
		}
		filter.ScheduledBefore = &parsed
	}
	if value := strings.TrimSpace(query.Get("scheduledAfter")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return reminders.OccurrenceFilter{}, err
		}
		filter.ScheduledAfter = &parsed
	}
	return filter, nil
}
