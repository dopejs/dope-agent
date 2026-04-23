package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStoreReminderRecordsRemainEnvironmentScopedAndActionHistoryDoesNotLeak(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	if err := sqliteStore.UpsertReminder(ctx, ReminderRecord{
		ReminderID:       "rem_test",
		EnvironmentScope: "test",
		BehaviorMode:     "notify_only",
		CurrentState:     "due",
		UpdatedAt:        now,
		Document:         mustMarshalJSON(t, map[string]any{"reminderId": "rem_test", "environmentScope": "test", "title": "Test Reminder", "behaviorMode": "notify_only", "currentState": "due", "createdAt": now, "updatedAt": now}),
	}); err != nil {
		t.Fatalf("UpsertReminder(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertReminder(ctx, ReminderRecord{
		ReminderID:       "rem_prod",
		EnvironmentScope: "prod",
		BehaviorMode:     "notify_only",
		CurrentState:     "due",
		UpdatedAt:        now.Add(time.Second),
		Document:         mustMarshalJSON(t, map[string]any{"reminderId": "rem_prod", "environmentScope": "prod", "title": "Prod Reminder", "behaviorMode": "notify_only", "currentState": "due", "createdAt": now, "updatedAt": now.Add(time.Second)}),
	}); err != nil {
		t.Fatalf("UpsertReminder(prod) returned error: %v", err)
	}

	if err := sqliteStore.UpsertReminderOccurrence(ctx, ReminderOccurrenceRecord{
		OccurrenceID:     "occ_test",
		ReminderID:       "rem_test",
		EnvironmentScope: "test",
		State:            "due",
		ScheduledFor:     now,
		UpdatedAt:        now,
		Document:         mustMarshalJSON(t, map[string]any{"occurrenceId": "occ_test", "reminderId": "rem_test", "environmentScope": "test", "state": "due", "scheduledFor": now, "createdAt": now, "updatedAt": now}),
	}); err != nil {
		t.Fatalf("UpsertReminderOccurrence(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertReminderOccurrence(ctx, ReminderOccurrenceRecord{
		OccurrenceID:     "occ_prod",
		ReminderID:       "rem_prod",
		EnvironmentScope: "prod",
		State:            "due",
		ScheduledFor:     now,
		UpdatedAt:        now,
		Document:         mustMarshalJSON(t, map[string]any{"occurrenceId": "occ_prod", "reminderId": "rem_prod", "environmentScope": "prod", "state": "due", "scheduledFor": now, "createdAt": now, "updatedAt": now}),
	}); err != nil {
		t.Fatalf("UpsertReminderOccurrence(prod) returned error: %v", err)
	}

	if err := sqliteStore.AppendReminderAction(ctx, ReminderActionRecord{
		ActionID:     "act_test",
		ReminderID:   "rem_test",
		OccurrenceID: "occ_test",
		ActionKind:   "delivery_linked",
		NewState:     "due",
		DeliveryID:   "delivery_test",
		CreatedAt:    now,
		Document:     mustMarshalJSON(t, map[string]any{"actionId": "act_test", "reminderId": "rem_test", "occurrenceId": "occ_test", "actionKind": "delivery_linked", "newState": "due", "deliveryId": "delivery_test", "createdAt": now}),
	}); err != nil {
		t.Fatalf("AppendReminderAction(test) returned error: %v", err)
	}
	if err := sqliteStore.AppendReminderAction(ctx, ReminderActionRecord{
		ActionID:     "act_prod",
		ReminderID:   "rem_prod",
		OccurrenceID: "occ_prod",
		ActionKind:   "delivery_linked",
		NewState:     "due",
		DeliveryID:   "delivery_prod",
		CreatedAt:    now.Add(time.Second),
		Document:     mustMarshalJSON(t, map[string]any{"actionId": "act_prod", "reminderId": "rem_prod", "occurrenceId": "occ_prod", "actionKind": "delivery_linked", "newState": "due", "deliveryId": "delivery_prod", "createdAt": now.Add(time.Second)}),
	}); err != nil {
		t.Fatalf("AppendReminderAction(prod) returned error: %v", err)
	}

	testReminders, err := sqliteStore.ListReminders(ctx, "test")
	if err != nil {
		t.Fatalf("ListReminders(test) returned error: %v", err)
	}
	if len(testReminders) != 1 || testReminders[0].ReminderID != "rem_test" {
		t.Fatalf("expected only test reminder, got %+v", testReminders)
	}

	testOccurrences, err := sqliteStore.ListReminderOccurrences(ctx, "test", ReminderOccurrenceFilter{})
	if err != nil {
		t.Fatalf("ListReminderOccurrences(test) returned error: %v", err)
	}
	if len(testOccurrences) != 1 || testOccurrences[0].OccurrenceID != "occ_test" {
		t.Fatalf("expected only test occurrence, got %+v", testOccurrences)
	}

	testActions, err := sqliteStore.ListReminderActions(ctx, "test", "rem_test")
	if err != nil {
		t.Fatalf("ListReminderActions(test) returned error: %v", err)
	}
	if len(testActions) != 1 || testActions[0].ActionID != "act_test" {
		t.Fatalf("expected only test action history, got %+v", testActions)
	}

	prodActions, err := sqliteStore.ListReminderActions(ctx, "prod", "rem_prod")
	if err != nil {
		t.Fatalf("ListReminderActions(prod) returned error: %v", err)
	}
	if len(prodActions) != 1 || prodActions[0].ActionID != "act_prod" {
		t.Fatalf("expected only prod action history, got %+v", prodActions)
	}
}
