package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
)

func handleSchedules(sched *scheduler.Scheduler, deliveryManager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if sched == nil {
		writeError(w, http.StatusInternalServerError, "scheduler is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := sched.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items, err = projectScheduleDeliverySummaries(r.Context(), deliveryManager, items)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items, err = projectSchedulesCalendarSummaries(r.Context(), sched.Store(), items)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ScheduleListResponse{Items: items})
	case http.MethodPost:
		var input CreateScheduleRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		trigger, err := scheduleTriggerFromRequest(input.Trigger)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var workflowTarget *scheduler.WorkflowTarget
		if input.Target.Workflow != nil {
			action, err := buildCalendarAction(input.Target.Workflow.CalendarAction)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			workflowTarget = &scheduler.WorkflowTarget{
				SessionID:      input.Target.Workflow.SessionID,
				Entrypoint:     input.Target.Workflow.Entrypoint,
				RunGoal:        input.Target.Workflow.RunGoal,
				WorkflowGoal:   input.Target.Workflow.WorkflowGoal,
				CalendarAction: action,
			}
		}
		target := scheduler.Target{
			Kind:     input.Target.Kind,
			Run:      input.Target.Run,
			Workflow: workflowTarget,
		}
		item, err := sched.Create(r.Context(), scheduler.CreateInput{
			Trigger:     trigger,
			Target:      target,
			RetryPolicy: input.RetryPolicy,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleScheduleRoutes(sched *scheduler.Scheduler, deliveryManager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if sched == nil {
		writeError(w, http.StatusInternalServerError, "scheduler is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/schedules/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleScheduleByID(sched, deliveryManager, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "pause" {
		handleSchedulePause(sched, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		handleScheduleResume(sched, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		handleScheduleCancel(sched, w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func handleScheduleByID(sched *scheduler.Scheduler, deliveryManager *delivery.Manager, w http.ResponseWriter, r *http.Request, scheduleID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := sched.Get(r.Context(), scheduleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	item, err = projectScheduleDeliverySummary(r.Context(), deliveryManager, item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item, err = projectScheduleCalendarSummaries(r.Context(), sched.Store(), item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleSchedulePause(sched *scheduler.Scheduler, w http.ResponseWriter, r *http.Request, scheduleID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := sched.Pause(r.Context(), scheduleID)
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

func handleScheduleResume(sched *scheduler.Scheduler, w http.ResponseWriter, r *http.Request, scheduleID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := sched.Resume(r.Context(), scheduleID)
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

func handleScheduleCancel(sched *scheduler.Scheduler, w http.ResponseWriter, r *http.Request, scheduleID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := sched.Cancel(r.Context(), scheduleID)
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

func scheduleTriggerFromRequest(input ScheduleTriggerRequest) (scheduler.Trigger, error) {
	trigger := scheduler.Trigger{
		Kind:     input.Kind,
		CronExpr: strings.TrimSpace(input.CronExpr),
		Timezone: strings.TrimSpace(input.Timezone),
	}
	switch input.Kind {
	case scheduler.TriggerKindOnce:
		fireAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.FireAt))
		if err != nil {
			return scheduler.Trigger{}, err
		}
		fireAt = fireAt.UTC()
		trigger.FireAt = &fireAt
	case scheduler.TriggerKindCron:
		if trigger.Timezone == "" {
			return scheduler.Trigger{}, fmt.Errorf("cron schedule requires timezone")
		}
	default:
		return scheduler.Trigger{}, fmt.Errorf("unsupported trigger kind %q", input.Kind)
	}
	return trigger, nil
}
