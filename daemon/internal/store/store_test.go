package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

func TestSQLiteStorePersistsRunsStepsAndEvents(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	session := router.Session{
		SessionID:    "session_test",
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastActiveAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := store.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}

	run := runtime.Run{
		RunID:      "run_test",
		SessionID:  session.SessionID,
		Entrypoint: "chat",
		Status:     runtime.RunStatusRunning,
		Goal:       "ship persistence",
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	step := runtime.Step{
		StepID:    "step_test",
		RunID:     run.RunID,
		Title:     "persist runtime state",
		Kind:      "task",
		Status:    runtime.StepStatusExecutingTool,
		Input:     map[string]any{"attempt": 1},
		Output:    map[string]any{"result": "pending"},
		CreatedAt: time.Now().UTC().Add(-30 * time.Second),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	event := events.Event{
		EventID:    "evt_test",
		Category:   "step",
		Name:       "step.status_changed",
		OccurredAt: time.Now().UTC(),
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
			StepID:    step.StepID,
		},
		Resource: events.Resource{
			Kind: "step",
			ID:   step.StepID,
		},
		Payload: map[string]any{
			"status": step.Status,
		},
	}
	persistedEvent, err := store.AppendEvent(ctx, event)
	if err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if persistedEvent.Sequence == 0 {
		t.Fatal("expected persisted event sequence")
	}

	// Roadmap 35 (T028) deleted Store.ListRuns. The test verifies that
	// the just-upserted run is readable; pull the row directly via SQL
	// to avoid coupling the assertion to tenant scoping (the test does
	// not bootstrap a tenant).
	runs, err := listRunsForTest(t, store, ctx)
	if err != nil {
		t.Fatalf("listRunsForTest returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].RunID != run.RunID {
		t.Fatalf("expected run ID %s, got %s", run.RunID, runs[0].RunID)
	}

	steps, err := store.ListSteps(ctx, run.RunID)
	if err != nil {
		t.Fatalf("ListSteps returned error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].StepID != step.StepID {
		t.Fatalf("expected step ID %s, got %s", step.StepID, steps[0].StepID)
	}

	items, err := store.ListEvents(ctx, events.Filter{RunID: run.RunID})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 event, got %d", len(items))
	}
	if items[0].EventID != event.EventID {
		t.Fatalf("expected event ID %s, got %s", event.EventID, items[0].EventID)
	}
	if items[0].Sequence != persistedEvent.Sequence {
		t.Fatalf("expected event sequence %d, got %d", persistedEvent.Sequence, items[0].Sequence)
	}
	if items[0].Scope.StepID != step.StepID {
		t.Fatalf("expected event step ID %s, got %s", step.StepID, items[0].Scope.StepID)
	}
}

func TestSQLiteStoreDeliveryResourcesRemainEnvironmentScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	ctx := context.Background()
	testUpdatedAt := time.Now().UTC()
	prodUpdatedAt := testUpdatedAt.Add(time.Second)
	if err := sqliteStore.UpsertDeliveryOutcome(ctx, DeliveryOutcomeRecord{
		DeliveryID:       "delivery_test_env",
		EnvironmentScope: "test",
		SourceKind:       "run",
		SourceID:         "run_test_env",
		Status:           "delivered",
		UpdatedAt:        testUpdatedAt,
		Document:         mustMarshalJSON(t, map[string]any{"deliveryId": "delivery_test_env", "environmentScope": "test", "sourceKind": "run", "sourceId": "run_test_env", "status": "delivered", "updatedAt": testUpdatedAt}),
	}); err != nil {
		t.Fatalf("UpsertDeliveryOutcome(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertDeliveryOutcome(ctx, DeliveryOutcomeRecord{
		DeliveryID:       "delivery_prod_env",
		EnvironmentScope: "prod",
		SourceKind:       "run",
		SourceID:         "run_prod_env",
		Status:           "delivered",
		UpdatedAt:        prodUpdatedAt,
		Document:         mustMarshalJSON(t, map[string]any{"deliveryId": "delivery_prod_env", "environmentScope": "prod", "sourceKind": "run", "sourceId": "run_prod_env", "status": "delivered", "updatedAt": prodUpdatedAt}),
	}); err != nil {
		t.Fatalf("UpsertDeliveryOutcome(prod) returned error: %v", err)
	}

	testItems, err := sqliteStore.ListDeliveryOutcomes(ctx, "test", DeliveryOutcomeFilter{})
	if err != nil {
		t.Fatalf("ListDeliveryOutcomes(test) returned error: %v", err)
	}
	prodItems, err := sqliteStore.ListDeliveryOutcomes(ctx, "prod", DeliveryOutcomeFilter{})
	if err != nil {
		t.Fatalf("ListDeliveryOutcomes(prod) returned error: %v", err)
	}
	if len(testItems) != 1 || testItems[0].DeliveryID != "delivery_test_env" {
		t.Fatalf("expected only test-scoped outcome, got %+v", testItems)
	}
	if len(prodItems) != 1 || prodItems[0].DeliveryID != "delivery_prod_env" {
		t.Fatalf("expected only prod-scoped outcome, got %+v", prodItems)
	}
}

func TestSQLiteStoreMailRecordsRemainEnvironmentScopedAndFilterable(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	ctx := context.Background()
	now := time.Now().UTC()
	testAccount := mail.AccountProjection{
		MailAccountID:            "mail_acct_test",
		IntegrationID:            "mail-test",
		DomainKind:               "mail",
		EnvironmentScope:         "test",
		ReadinessStatus:          "healthy",
		CanonicalDefault:         true,
		MailboxAddress:           "alice@example.com",
		MailboxLabel:             "Alice Mailbox",
		SupportsThreadInspection: true,
		SupportsDrafts:           true,
		SupportsDirectSend:       true,
		SupportsReply:            true,
		SupportsForward:          true,
		LastSyncedAt:             now,
		UpdatedAt:                now,
	}
	prodAccount := testAccount
	prodAccount.MailAccountID = "mail_acct_prod"
	prodAccount.IntegrationID = "mail-prod"
	prodAccount.EnvironmentScope = "prod"
	if err := sqliteStore.UpsertMailAccount(ctx, testAccount); err != nil {
		t.Fatalf("UpsertMailAccount(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertMailAccount(ctx, prodAccount); err != nil {
		t.Fatalf("UpsertMailAccount(prod) returned error: %v", err)
	}

	testCompletedAt := now.Add(time.Second)
	testOperation := mail.Operation{
		OperationID:      "mail_op_test",
		OperationClass:   mail.OperationClassSendMessage,
		Status:           mail.OperationStatusCompleted,
		ResultMode:       mail.ResultModeSent,
		SendPath:         mail.SendPathDirect,
		IntegrationID:    testAccount.IntegrationID,
		MailAccountID:    testAccount.MailAccountID,
		EnvironmentScope: "test",
		ThreadID:         "thread_test",
		MessageID:        "msg_test",
		WorkflowID:       "wf_1",
		ScheduleID:       "sched_1",
		DeliveryID:       "delivery_1",
		CreatedAt:        now,
		CompletedAt:      &testCompletedAt,
		UpdatedAt:        testCompletedAt,
	}
	prodOperation := testOperation
	prodOperation.OperationID = "mail_op_prod"
	prodOperation.IntegrationID = prodAccount.IntegrationID
	prodOperation.MailAccountID = prodAccount.MailAccountID
	prodOperation.EnvironmentScope = "prod"
	prodOperation.WorkflowID = "wf_prod"
	prodOperation.DeliveryID = "delivery_prod"
	if err := sqliteStore.UpsertMailOperation(ctx, testOperation); err != nil {
		t.Fatalf("UpsertMailOperation(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertMailOperation(ctx, prodOperation); err != nil {
		t.Fatalf("UpsertMailOperation(prod) returned error: %v", err)
	}

	testArtifact := mail.Artifact{
		ArtifactID:       "mail_artifact_test",
		OperationID:      testOperation.OperationID,
		Kind:             mail.ArtifactKindMessageSnapshot,
		IntegrationID:    testAccount.IntegrationID,
		EnvironmentScope: "test",
		MessageID:        "msg_test",
		Message: &mail.MessageSnapshot{
			MessageID:     "msg_test",
			OperationID:   testOperation.OperationID,
			IntegrationID: testAccount.IntegrationID,
			MailAccountID: testAccount.MailAccountID,
			Direction:     mail.DirectionOutbound,
			Subject:       "Stored message",
			DeliveryState: mail.DeliveryStateSent,
			CreatedAt:     now,
		},
		CreatedAt: now,
	}
	prodArtifact := testArtifact
	prodArtifact.ArtifactID = "mail_artifact_prod"
	prodArtifact.OperationID = prodOperation.OperationID
	prodArtifact.IntegrationID = prodAccount.IntegrationID
	prodArtifact.EnvironmentScope = "prod"
	if err := sqliteStore.UpsertMailArtifact(ctx, testArtifact); err != nil {
		t.Fatalf("UpsertMailArtifact(test) returned error: %v", err)
	}
	if err := sqliteStore.UpsertMailArtifact(ctx, prodArtifact); err != nil {
		t.Fatalf("UpsertMailArtifact(prod) returned error: %v", err)
	}

	testAccounts, err := sqliteStore.ListMailAccounts(ctx, "test")
	if err != nil {
		t.Fatalf("ListMailAccounts(test) returned error: %v", err)
	}
	if len(testAccounts) != 1 || testAccounts[0].IntegrationID != "mail-test" {
		t.Fatalf("expected only test-scoped mail account, got %+v", testAccounts)
	}

	filteredOps, err := sqliteStore.ListMailOperations(ctx, "test", MailOperationFilter{
		WorkflowID: "wf_1",
		ScheduleID: "sched_1",
		DeliveryID: "delivery_1",
		Status:     string(mail.OperationStatusCompleted),
	})
	if err != nil {
		t.Fatalf("ListMailOperations(test) returned error: %v", err)
	}
	if len(filteredOps) != 1 || filteredOps[0].OperationID != testOperation.OperationID {
		t.Fatalf("expected filtered test mail operation, got %+v", filteredOps)
	}

	prodOps, err := sqliteStore.ListMailOperations(ctx, "prod", MailOperationFilter{})
	if err != nil {
		t.Fatalf("ListMailOperations(prod) returned error: %v", err)
	}
	if len(prodOps) != 1 || prodOps[0].OperationID != prodOperation.OperationID {
		t.Fatalf("expected only prod-scoped mail operation, got %+v", prodOps)
	}

	testArtifacts, err := sqliteStore.ListMailArtifacts(ctx, "test", testOperation.OperationID)
	if err != nil {
		t.Fatalf("ListMailArtifacts(test) returned error: %v", err)
	}
	if len(testArtifacts) != 1 || testArtifacts[0].ArtifactID != testArtifact.ArtifactID {
		t.Fatalf("expected only test-scoped mail artifact, got %+v", testArtifacts)
	}
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return data
}

func TestSQLiteStorePersistsComputerUseRecords(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	run := runtime.Run{
		RunID:      "run_cu",
		Entrypoint: "operator",
		Status:     runtime.RunStatusRunning,
		Goal:       "persist computer-use records",
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	session := computeruse.Session{
		ComputerUseSessionID: "cusess_1",
		EnvironmentScope:     "test",
		RunID:                run.RunID,
		Status:               computeruse.SessionStatusActive,
		DriverKind:           "browser",
		StartedAt:            time.Now().UTC().Add(-time.Minute),
		UpdatedAt:            time.Now().UTC(),
		CurrentPage:          &computeruse.PageSummary{URL: "https://example.test", Title: "example.test"},
	}
	if err := store.UpsertComputerUseSession(ctx, session); err != nil {
		t.Fatalf("UpsertComputerUseSession returned error: %v", err)
	}

	action := computeruse.Action{
		ComputerUseActionID:  "cuact_1",
		EnvironmentScope:     "test",
		ComputerUseSessionID: session.ComputerUseSessionID,
		RunID:                run.RunID,
		ActionKind:           computeruse.ActionKindNavigate,
		Status:               computeruse.ActionStatusCompleted,
		RiskLevel:            computeruse.RiskLevelLow,
		RequestedAt:          time.Now().UTC().Add(-30 * time.Second),
		UpdatedAt:            time.Now().UTC(),
	}
	if err := store.UpsertComputerUseAction(ctx, action); err != nil {
		t.Fatalf("UpsertComputerUseAction returned error: %v", err)
	}

	artifact := computeruse.Artifact{
		ArtifactID:           "cuart_1",
		EnvironmentScope:     "test",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ComputerUseActionID:  action.ComputerUseActionID,
		RunID:                run.RunID,
		Kind:                 computeruse.ArtifactKindPageSnapshot,
		Status:               computeruse.ArtifactStatusAvailable,
		FileName:             "page-snapshot.json",
		ByteSize:             12,
		StorageKey:           "computer-use/cusess_1/cuart_1",
		CreatedAt:            time.Now().UTC(),
	}
	if err := store.UpsertComputerUseArtifact(ctx, artifact); err != nil {
		t.Fatalf("UpsertComputerUseArtifact returned error: %v", err)
	}

	sessions, err := store.ListComputerUseSessions(ctx, "test", run.RunID)
	if err != nil {
		t.Fatalf("ListComputerUseSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ComputerUseSessionID != session.ComputerUseSessionID {
		t.Fatalf("expected persisted session, got %+v", sessions)
	}

	actions, err := store.ListComputerUseActions(ctx, "test", run.RunID, session.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("ListComputerUseActions returned error: %v", err)
	}
	if len(actions) != 1 || actions[0].ComputerUseActionID != action.ComputerUseActionID {
		t.Fatalf("expected persisted action, got %+v", actions)
	}

	artifacts, err := store.ListComputerUseArtifactsForAction(ctx, "test", run.RunID, action.ComputerUseActionID)
	if err != nil {
		t.Fatalf("ListComputerUseArtifactsForAction returned error: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ArtifactID != artifact.ArtifactID {
		t.Fatalf("expected persisted artifact, got %+v", artifacts)
	}
}

func TestSQLiteStoreFiltersComputerUseRecordsByEnvironment(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	run := runtime.Run{
		RunID:      "run_env",
		Entrypoint: "operator",
		Status:     runtime.RunStatusRunning,
		Goal:       "env filter",
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	testSession := computeruse.Session{
		ComputerUseSessionID: "cusess_test",
		EnvironmentScope:     "test",
		RunID:                run.RunID,
		Status:               computeruse.SessionStatusActive,
		DriverKind:           "browser",
		StartedAt:            time.Now().UTC().Add(-time.Minute),
		UpdatedAt:            time.Now().UTC(),
	}
	prodSession := testSession
	prodSession.ComputerUseSessionID = "cusess_prod"
	prodSession.EnvironmentScope = "prod"
	if err := store.UpsertComputerUseSession(ctx, testSession); err != nil {
		t.Fatalf("UpsertComputerUseSession(test) returned error: %v", err)
	}
	if err := store.UpsertComputerUseSession(ctx, prodSession); err != nil {
		t.Fatalf("UpsertComputerUseSession(prod) returned error: %v", err)
	}

	testAction := computeruse.Action{
		ComputerUseActionID:  "cuact_test",
		EnvironmentScope:     "test",
		ComputerUseSessionID: testSession.ComputerUseSessionID,
		RunID:                run.RunID,
		ActionKind:           computeruse.ActionKindNavigate,
		Status:               computeruse.ActionStatusCompleted,
		RiskLevel:            computeruse.RiskLevelLow,
		RequestedAt:          time.Now().UTC().Add(-30 * time.Second),
		UpdatedAt:            time.Now().UTC(),
	}
	prodAction := testAction
	prodAction.ComputerUseActionID = "cuact_prod"
	prodAction.EnvironmentScope = "prod"
	prodAction.ComputerUseSessionID = prodSession.ComputerUseSessionID
	if err := store.UpsertComputerUseAction(ctx, testAction); err != nil {
		t.Fatalf("UpsertComputerUseAction(test) returned error: %v", err)
	}
	if err := store.UpsertComputerUseAction(ctx, prodAction); err != nil {
		t.Fatalf("UpsertComputerUseAction(prod) returned error: %v", err)
	}

	testArtifact := computeruse.Artifact{
		ArtifactID:           "cuart_test",
		EnvironmentScope:     "test",
		ComputerUseSessionID: testSession.ComputerUseSessionID,
		ComputerUseActionID:  testAction.ComputerUseActionID,
		RunID:                run.RunID,
		Kind:                 computeruse.ArtifactKindPageSnapshot,
		Status:               computeruse.ArtifactStatusAvailable,
		StorageKey:           "computer-use/test/cuart_test",
		CreatedAt:            time.Now().UTC(),
	}
	prodArtifact := testArtifact
	prodArtifact.ArtifactID = "cuart_prod"
	prodArtifact.EnvironmentScope = "prod"
	prodArtifact.ComputerUseSessionID = prodSession.ComputerUseSessionID
	prodArtifact.ComputerUseActionID = prodAction.ComputerUseActionID
	if err := store.UpsertComputerUseArtifact(ctx, testArtifact); err != nil {
		t.Fatalf("UpsertComputerUseArtifact(test) returned error: %v", err)
	}
	if err := store.UpsertComputerUseArtifact(ctx, prodArtifact); err != nil {
		t.Fatalf("UpsertComputerUseArtifact(prod) returned error: %v", err)
	}

	sessions, err := store.ListComputerUseSessions(ctx, "test", run.RunID)
	if err != nil {
		t.Fatalf("ListComputerUseSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ComputerUseSessionID != testSession.ComputerUseSessionID {
		t.Fatalf("expected only test session, got %+v", sessions)
	}

	actions, err := store.ListComputerUseActions(ctx, "test", run.RunID, testSession.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("ListComputerUseActions returned error: %v", err)
	}
	if len(actions) != 1 || actions[0].ComputerUseActionID != testAction.ComputerUseActionID {
		t.Fatalf("expected only test action, got %+v", actions)
	}

	artifact, ok, err := store.GetComputerUseArtifact(ctx, "test", testArtifact.ArtifactID)
	if err != nil {
		t.Fatalf("GetComputerUseArtifact returned error: %v", err)
	}
	if !ok || artifact.ArtifactID != testArtifact.ArtifactID {
		t.Fatalf("expected test artifact, got ok=%v artifact=%+v", ok, artifact)
	}
	if _, ok, err := store.GetComputerUseArtifact(ctx, "test", prodArtifact.ArtifactID); err != nil || ok {
		t.Fatalf("expected prod artifact to be hidden from test, got ok=%v err=%v", ok, err)
	}
}

func TestSQLiteStoreMarksInFlightComputerUseInterrupted(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	run := runtime.Run{
		RunID:      "run_interrupt",
		Entrypoint: "operator",
		Status:     runtime.RunStatusRunning,
		Goal:       "interrupt computer-use truth",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now,
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	inFlightSession := computeruse.Session{
		ComputerUseSessionID: "cusess_interrupt",
		EnvironmentScope:     "test",
		RunID:                run.RunID,
		Status:               computeruse.SessionStatusBlocked,
		DriverKind:           "browser",
		StartedAt:            now.Add(-time.Minute),
		UpdatedAt:            now.Add(-time.Second),
	}
	closedSession := inFlightSession
	closedSession.ComputerUseSessionID = "cusess_closed"
	closedSession.Status = computeruse.SessionStatusClosed
	prodSession := inFlightSession
	prodSession.ComputerUseSessionID = "cusess_prod"
	prodSession.EnvironmentScope = "prod"
	if err := store.UpsertComputerUseSession(ctx, inFlightSession); err != nil {
		t.Fatalf("UpsertComputerUseSession(in-flight) returned error: %v", err)
	}
	if err := store.UpsertComputerUseSession(ctx, closedSession); err != nil {
		t.Fatalf("UpsertComputerUseSession(closed) returned error: %v", err)
	}
	if err := store.UpsertComputerUseSession(ctx, prodSession); err != nil {
		t.Fatalf("UpsertComputerUseSession(prod) returned error: %v", err)
	}

	waitingAction := computeruse.Action{
		ComputerUseActionID:  "cuact_waiting",
		EnvironmentScope:     "test",
		ComputerUseSessionID: inFlightSession.ComputerUseSessionID,
		RunID:                run.RunID,
		ActionKind:           computeruse.ActionKindClick,
		Status:               computeruse.ActionStatusWaitingApproval,
		RiskLevel:            computeruse.RiskLevelHigh,
		RequestedAt:          now.Add(-40 * time.Second),
		UpdatedAt:            now.Add(-time.Second),
	}
	completedAction := waitingAction
	completedAction.ComputerUseActionID = "cuact_done"
	completedAction.Status = computeruse.ActionStatusCompleted
	completedAction.ComputerUseSessionID = closedSession.ComputerUseSessionID
	prodAction := waitingAction
	prodAction.ComputerUseActionID = "cuact_prod"
	prodAction.EnvironmentScope = "prod"
	prodAction.ComputerUseSessionID = prodSession.ComputerUseSessionID
	if err := store.UpsertComputerUseAction(ctx, waitingAction); err != nil {
		t.Fatalf("UpsertComputerUseAction(waiting) returned error: %v", err)
	}
	if err := store.UpsertComputerUseAction(ctx, completedAction); err != nil {
		t.Fatalf("UpsertComputerUseAction(completed) returned error: %v", err)
	}
	if err := store.UpsertComputerUseAction(ctx, prodAction); err != nil {
		t.Fatalf("UpsertComputerUseAction(prod) returned error: %v", err)
	}

	interruptedAt := now.Add(time.Second)
	sessions, actions, err := store.MarkInFlightComputerUseInterrupted(ctx, "test", interruptedAt)
	if err != nil {
		t.Fatalf("MarkInFlightComputerUseInterrupted returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ComputerUseSessionID != inFlightSession.ComputerUseSessionID {
		t.Fatalf("expected only in-flight test session interrupted, got %+v", sessions)
	}
	if len(actions) != 1 || actions[0].ComputerUseActionID != waitingAction.ComputerUseActionID {
		t.Fatalf("expected only in-flight test action interrupted, got %+v", actions)
	}

	restoredSession, ok, err := store.GetComputerUseSession(ctx, "test", run.RunID, inFlightSession.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("GetComputerUseSession returned error: %v", err)
	}
	if !ok || restoredSession.Status != computeruse.SessionStatusInterrupted || restoredSession.InterruptedAt == nil {
		t.Fatalf("expected interrupted persisted session, got ok=%v session=%+v", ok, restoredSession)
	}

	restoredAction, ok, err := store.GetComputerUseAction(ctx, "test", run.RunID, inFlightSession.ComputerUseSessionID, waitingAction.ComputerUseActionID)
	if err != nil {
		t.Fatalf("GetComputerUseAction returned error: %v", err)
	}
	if !ok || restoredAction.Status != computeruse.ActionStatusInterrupted || restoredAction.FailureClass != string(computeruse.FailureClassInterrupted) {
		t.Fatalf("expected interrupted persisted action, got ok=%v action=%+v", ok, restoredAction)
	}

	closedRestored, ok, err := store.GetComputerUseSession(ctx, "test", run.RunID, closedSession.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("GetComputerUseSession(closed) returned error: %v", err)
	}
	if !ok || closedRestored.Status != computeruse.SessionStatusClosed {
		t.Fatalf("expected closed session untouched, got ok=%v session=%+v", ok, closedRestored)
	}

	prodRestored, ok, err := store.GetComputerUseSession(ctx, "prod", run.RunID, prodSession.ComputerUseSessionID)
	if err != nil {
		t.Fatalf("GetComputerUseSession(prod) returned error: %v", err)
	}
	if !ok || prodRestored.Status != computeruse.SessionStatusBlocked {
		t.Fatalf("expected prod session untouched, got ok=%v session=%+v", ok, prodRestored)
	}
}

func TestSQLiteStorePersistsScheduleScopedRunsAndEvents(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	scheduleRecord := ScheduleRecord{
		ScheduleID:       "sched_test",
		EnvironmentScope: "test",
		Kind:             "one_time",
		Status:           "scheduled",
		TargetRefID:      "sched_target_test",
		CreatedAt:        now,
		UpdatedAt:        now,
		Document:         []byte(`{"scheduleId":"sched_test","kind":"one_time","status":"scheduled","targetRefId":"sched_target_test","createdAt":"2026-04-22T10:00:00Z","updatedAt":"2026-04-22T10:00:00Z"}`),
	}
	if err := store.UpsertSchedule(ctx, scheduleRecord); err != nil {
		t.Fatalf("UpsertSchedule returned error: %v", err)
	}
	if err := store.UpsertScheduleTarget(ctx, ScheduleTargetRecord{
		TargetRefID: "sched_target_test",
		ScheduleID:  "sched_test",
		TargetKind:  "run",
		Revision:    1,
		Active:      true,
		UpdatedAt:   now,
		Document:    []byte(`{"kind":"run","revision":1,"active":true,"run":{"entrypoint":"operator","goal":"scheduled"}}`),
	}); err != nil {
		t.Fatalf("UpsertScheduleTarget returned error: %v", err)
	}

	run := runtime.Run{
		RunID:             "run_scheduled",
		ScheduleID:        "sched_test",
		ScheduleAttemptID: "sched_attempt_test",
		Entrypoint:        "operator",
		Status:            runtime.RunStatusQueued,
		Goal:              "scheduled run",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	persistedEvent, err := store.AppendEvent(ctx, events.Event{
		EventID:    "evt_schedule_test",
		Category:   "schedule",
		Name:       "schedule.dispatch_recorded",
		OccurredAt: now,
		Scope: events.Scope{
			ScheduleID:        "sched_test",
			ScheduleAttemptID: "sched_attempt_test",
			RunID:             run.RunID,
		},
		Resource: events.Resource{
			Kind: "schedule",
			ID:   "sched_test",
		},
		Payload: map[string]any{"dispatchStatus": "dispatched"},
	})
	if err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if persistedEvent.Sequence == 0 {
		t.Fatal("expected persisted event sequence")
	}

	// Roadmap 35 (T028) deleted Store.ListRuns. The test verifies that
	// the just-upserted run is readable; pull the row directly via SQL
	// to avoid coupling the assertion to tenant scoping (the test does
	// not bootstrap a tenant).
	runs, err := listRunsForTest(t, store, ctx)
	if err != nil {
		t.Fatalf("listRunsForTest returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].ScheduleID != "sched_test" || runs[0].ScheduleAttemptID != "sched_attempt_test" {
		t.Fatalf("expected persisted schedule linkage on run, got %+v", runs)
	}

	items, err := store.ListEvents(ctx, events.Filter{ScheduleID: "sched_test"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one schedule-scoped event, got %+v", items)
	}
	if items[0].Scope.ScheduleAttemptID != "sched_attempt_test" {
		t.Fatalf("expected schedule attempt scope, got %+v", items[0])
	}
}

func TestSQLiteStoreListsSchedulesByEnvironmentScope(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	now := time.Now().UTC()
	if err := store.UpsertSchedule(context.Background(), ScheduleRecord{
		ScheduleID:       "sched_test_visible",
		EnvironmentScope: "test",
		Kind:             "one_time",
		Status:           "scheduled",
		TargetRefID:      "target_test_visible",
		CreatedAt:        now,
		UpdatedAt:        now,
		Document:         []byte(`{"scheduleId":"sched_test_visible","environmentScope":"test","kind":"one_time","status":"scheduled","targetRefId":"target_test_visible"}`),
	}); err != nil {
		t.Fatalf("UpsertSchedule(test) returned error: %v", err)
	}
	if err := store.UpsertSchedule(context.Background(), ScheduleRecord{
		ScheduleID:       "sched_prod_hidden",
		EnvironmentScope: "prod",
		Kind:             "one_time",
		Status:           "scheduled",
		TargetRefID:      "target_prod_hidden",
		CreatedAt:        now,
		UpdatedAt:        now,
		Document:         []byte(`{"scheduleId":"sched_prod_hidden","environmentScope":"prod","kind":"one_time","status":"scheduled","targetRefId":"target_prod_hidden"}`),
	}); err != nil {
		t.Fatalf("UpsertSchedule(prod) returned error: %v", err)
	}

	items, err := store.ListSchedules(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListSchedules returned error: %v", err)
	}
	if len(items) != 1 || items[0].ScheduleID != "sched_test_visible" {
		t.Fatalf("expected only test schedule, got %+v", items)
	}
}

func TestSQLiteStorePersistsSessions(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	session := router.Session{
		SessionID:    "sess_test",
		Kind:         router.SessionKindGroup,
		Status:       router.SessionStatusActive,
		Channel:      "telegram",
		AccountID:    "acct_1",
		PeerID:       "group_1",
		ThreadID:     "thread_1",
		RoutingKey:   "group:telegram:acct_1:group_1:thread_1",
		Generation:   2,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC(),
		LastActiveAt: time.Now().UTC(),
	}

	if err := store.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}

	sessions, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != session.SessionID {
		t.Fatalf("expected session ID %s, got %s", session.SessionID, sessions[0].SessionID)
	}
	if sessions[0].Kind != router.SessionKindGroup {
		t.Fatalf("expected group session kind, got %s", sessions[0].Kind)
	}
}

func TestSQLiteStorePersistsConnectorsAndCapabilities(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	connectorHeartbeat := time.Now().UTC().Add(-15 * time.Second)
	connector := connectors.Connector{
		ConnectorID:     "telegram-main",
		Kind:            "telegram",
		DisplayName:     "Telegram Main",
		Status:          connectors.StatusHealthy,
		FailureCount:    1,
		RestartCount:    2,
		BackoffSeconds:  0,
		LastHeartbeatAt: &connectorHeartbeat,
		CreatedAt:       time.Now().UTC().Add(-time.Hour),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := store.UpsertConnector(ctx, connector); err != nil {
		t.Fatalf("UpsertConnector returned error: %v", err)
	}

	capabilityRestart := time.Now().UTC().Add(-10 * time.Second)
	capability := capabilities.Capability{
		CapabilityID:   "browser",
		Kind:           "browser",
		DisplayName:    "Browser",
		Status:         capabilities.StatusBackingOff,
		FailureCount:   2,
		RestartCount:   1,
		BackoffSeconds: 10,
		LastRestartAt:  &capabilityRestart,
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := store.UpsertCapability(ctx, capability); err != nil {
		t.Fatalf("UpsertCapability returned error: %v", err)
	}

	connectorsList, err := store.ListConnectors(ctx)
	if err != nil {
		t.Fatalf("ListConnectors returned error: %v", err)
	}
	if len(connectorsList) != 1 {
		t.Fatalf("expected 1 connector, got %d", len(connectorsList))
	}
	if connectorsList[0].ConnectorID != connector.ConnectorID {
		t.Fatalf("expected connector ID %s, got %s", connector.ConnectorID, connectorsList[0].ConnectorID)
	}
	if connectorsList[0].Status != connectors.StatusHealthy {
		t.Fatalf("expected connector status healthy, got %s", connectorsList[0].Status)
	}

	capabilitiesList, err := store.ListCapabilities(ctx)
	if err != nil {
		t.Fatalf("ListCapabilities returned error: %v", err)
	}
	if len(capabilitiesList) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(capabilitiesList))
	}
	if capabilitiesList[0].CapabilityID != capability.CapabilityID {
		t.Fatalf("expected capability ID %s, got %s", capability.CapabilityID, capabilitiesList[0].CapabilityID)
	}
	if capabilitiesList[0].BackoffSeconds != 10 {
		t.Fatalf("expected capability backoff 10, got %d", capabilitiesList[0].BackoffSeconds)
	}
}

func TestSQLiteStoreCreateConnectorMessageIfAbsentDeduplicatesByExternalID(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	first, created, err := store.CreateConnectorMessageIfAbsent(ctx, imtypes.MessageRecord{
		DeliveryID:        "delivery_1",
		ConnectorID:       "discord-main",
		Direction:         imtypes.DeliveryDirectionInbound,
		ExternalMessageID: "discord_msg_1",
		ChannelID:         "channel_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Status:            imtypes.DeliveryStatusReceived,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(first) returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first connector message insert to be created")
	}

	second, created, err := store.CreateConnectorMessageIfAbsent(ctx, imtypes.MessageRecord{
		DeliveryID:        "delivery_2",
		ConnectorID:       "discord-main",
		Direction:         imtypes.DeliveryDirectionInbound,
		ExternalMessageID: "discord_msg_1",
		ChannelID:         "channel_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello again",
		Status:            imtypes.DeliveryStatusReceived,
		CreatedAt:         now.Add(time.Second),
		UpdatedAt:         now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(second) returned error: %v", err)
	}
	if created {
		t.Fatal("expected duplicate connector message insert to be rejected")
	}
	if second.DeliveryID != first.DeliveryID {
		t.Fatalf("expected duplicate lookup to return first delivery ID %s, got %s", first.DeliveryID, second.DeliveryID)
	}
}

func TestSQLiteStorePersistsLLMDispatches(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	startedAt := time.Now().UTC().Add(-5 * time.Second)
	completedAt := time.Now().UTC().Add(-2 * time.Second)
	dispatch := llm.Dispatch{
		DispatchID:   "disp_1",
		Provider:     "echo",
		Model:        "test-model",
		Messages:     []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		Stream:       false,
		Status:       llm.DispatchStatusCompleted,
		Output:       "hello",
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
		TimeoutMs:    30000,
		MaxRetries:   1,
		AttemptCount: 1,
		CreatedAt:    time.Now().UTC().Add(-10 * time.Second),
		UpdatedAt:    time.Now().UTC().Add(-time.Second),
		StartedAt:    &startedAt,
		CompletedAt:  &completedAt,
	}
	if err := store.UpsertLLMDispatch(context.Background(), dispatch); err != nil {
		t.Fatalf("UpsertLLMDispatch returned error: %v", err)
	}

	items, err := store.ListLLMDispatches(context.Background())
	if err != nil {
		t.Fatalf("ListLLMDispatches returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 llm dispatch, got %d", len(items))
	}
	if items[0].DispatchID != dispatch.DispatchID {
		t.Fatalf("expected dispatch ID %s, got %s", dispatch.DispatchID, items[0].DispatchID)
	}
	if items[0].Usage.TotalTokens != 2 {
		t.Fatalf("expected total tokens 2, got %d", items[0].Usage.TotalTokens)
	}
	if len(items[0].Messages) != 1 || items[0].Messages[0].Content != "hello" {
		t.Fatalf("expected persisted llm messages, got %+v", items[0].Messages)
	}

	got, ok, err := store.GetLLMDispatch(context.Background(), dispatch.DispatchID)
	if err != nil {
		t.Fatalf("GetLLMDispatch returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected llm dispatch to exist")
	}
	if got.Status != llm.DispatchStatusCompleted {
		t.Fatalf("expected completed llm dispatch, got %s", got.Status)
	}
}

func TestSQLiteStorePersistsAuthAndPolicyState(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	completedAt := time.Now().UTC().Add(-2 * time.Minute)
	pairing := auth.Pairing{
		PairingID:   "pair_1",
		Mode:        auth.PairingModeLocal,
		Label:       "web-ui",
		Status:      auth.PairingStatusCompleted,
		CodeHash:    "hash",
		CodePreview: "",
		CreatedAt:   time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-5 * time.Minute),
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
		CompletedAt: &completedAt,
	}
	if err := store.UpsertPairing(context.Background(), pairing); err != nil {
		t.Fatalf("UpsertPairing returned error: %v", err)
	}

	lastUsedAt := time.Now().UTC().Add(-time.Minute)
	token := auth.AccessToken{
		TokenID:      "tok_1",
		Label:        "web-ui",
		Mode:         auth.PairingModeLocal,
		TokenHash:    "token-hash",
		TokenPreview: "dope_preview",
		CreatedAt:    time.Now().UTC().Add(-9 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastUsedAt:   &lastUsedAt,
	}
	if err := store.UpsertAccessToken(context.Background(), token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}

	approval := policy.Approval{
		ApprovalID:   "approval_1",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "needs approval",
		RequestedBy:  "web-ui",
		Status:       policy.ApprovalStatusApproved,
		CreatedAt:    time.Now().UTC().Add(-8 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-7 * time.Minute),
		ResolvedAt:   &completedAt,
		Resolution:   string(policy.ApprovalStatusApproved),
		Comment:      "approved",
	}
	if err := store.UpsertApproval(context.Background(), approval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	decision := policy.Decision{
		DecisionID:   "decision_1",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Outcome:      policy.DecisionOutcomeApproved,
		Reason:       "needs approval",
		ApprovalID:   approval.ApprovalID,
		CreatedAt:    time.Now().UTC().Add(-7 * time.Minute),
	}
	if err := store.UpsertDecision(context.Background(), decision); err != nil {
		t.Fatalf("UpsertDecision returned error: %v", err)
	}

	pairings, err := store.ListPairings(context.Background())
	if err != nil {
		t.Fatalf("ListPairings returned error: %v", err)
	}
	if len(pairings) != 1 || pairings[0].PairingID != pairing.PairingID {
		t.Fatalf("expected persisted pairing, got %+v", pairings)
	}

	tokens, err := store.ListAccessTokens(context.Background())
	if err != nil {
		t.Fatalf("ListAccessTokens returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].TokenID != token.TokenID {
		t.Fatalf("expected persisted token, got %+v", tokens)
	}

	approvals, err := store.ListApprovals(context.Background())
	if err != nil {
		t.Fatalf("ListApprovals returned error: %v", err)
	}
	if len(approvals) != 1 || approvals[0].ApprovalID != approval.ApprovalID {
		t.Fatalf("expected persisted approval, got %+v", approvals)
	}

	decisions, err := store.ListDecisions(context.Background())
	if err != nil {
		t.Fatalf("ListDecisions returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].DecisionID != decision.DecisionID {
		t.Fatalf("expected persisted decision, got %+v", decisions)
	}
}

func TestSQLiteStorePersistsIntegrationsAndBindingSnapshots(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	integration := integrations.Resource{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateHealthy,
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "acct_calendar",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
		Provenance: integrations.Provenance{
			SecretResolution:      "resolved",
			SecretMaterialPresent: true,
			EnvironmentScope:      "test",
			BackedBy:              string(integrations.BackendKindFakeLocal),
		},
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now,
		LastTransitionAt: now,
		LastReadyAt:      &now,
	}
	if err := store.UpsertIntegration(ctx, integration); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	items, err := store.ListIntegrations(ctx, "test")
	if err != nil {
		t.Fatalf("ListIntegrations returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 integration, got %d", len(items))
	}
	if items[0].IntegrationID != integration.IntegrationID || items[0].CanonicalDefault != integration.CanonicalDefault {
		t.Fatalf("expected persisted integration %+v, got %+v", integration, items[0])
	}

	binding := integrations.BindingSummary{
		IntegrationID:         integration.IntegrationID,
		DomainKind:            integration.DomainKind,
		DisplayName:           integration.DisplayName,
		AccountKey:            integration.AccountBinding.AccountKey,
		CanonicalDefault:      true,
		ReadinessAtInvocation: integrations.ReadinessStatusHealthy,
		BackendKind:           integrations.BackendKindFakeLocal,
		SecretResolution:      "resolved",
		EnvironmentScope:      "test",
		CapturedAt:            now,
	}

	approval := policy.Approval{
		ApprovalID:          "approval_integration_1",
		Action:              "integration.probe.mutate",
		ResourceKind:        "integration",
		ResourceID:          integration.IntegrationID,
		Reason:              "mutation probe",
		RequestedBy:         "tests",
		Status:              policy.ApprovalStatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
		IntegrationBindings: []integrations.BindingSummary{binding},
	}
	if err := store.UpsertApproval(ctx, approval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	run := runtime.Run{
		RunID:      "run_integration_store",
		Entrypoint: "operator",
		Status:     runtime.RunStatusRunning,
		Goal:       "persist integration bindings",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now,
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	step := runtime.Step{
		StepID:    "step_integration_store",
		RunID:     run.RunID,
		Title:     "probe integration",
		Kind:      "integration_probe",
		Status:    runtime.StepStatusExecutingTool,
		CreatedAt: now.Add(-30 * time.Second),
		UpdatedAt: now,
	}
	if err := store.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}
	toolCall := runtime.ToolCall{
		ToolCallID:          "tool_call_integration_store",
		RunID:               run.RunID,
		StepID:              step.StepID,
		CapabilityID:        "integration_probe",
		ToolName:            "inspect",
		Status:              runtime.ToolCallStatusCompleted,
		CreatedAt:           now,
		UpdatedAt:           now,
		IntegrationBindings: []integrations.BindingSummary{binding},
		Output:              map[string]any{"ok": true},
	}
	if err := store.UpsertToolCall(ctx, toolCall); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}

	approvals, err := store.ListApprovals(ctx)
	if err != nil {
		t.Fatalf("ListApprovals returned error: %v", err)
	}
	if len(approvals) != 1 || len(approvals[0].IntegrationBindings) != 1 || approvals[0].IntegrationBindings[0].IntegrationID != integration.IntegrationID {
		t.Fatalf("expected approval integration bindings to round-trip, got %+v", approvals)
	}

	toolCalls, err := store.ListToolCalls(ctx, run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(toolCalls) != 1 || len(toolCalls[0].IntegrationBindings) != 1 || toolCalls[0].IntegrationBindings[0].IntegrationID != integration.IntegrationID {
		t.Fatalf("expected tool call integration bindings to round-trip, got %+v", toolCalls)
	}
}

func TestSQLiteStorePersistsLatestCheckpointPerRun(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	run := runtime.Run{
		RunID:      "run_ckpt",
		Entrypoint: "chat",
		Status:     runtime.RunStatusQueued,
		Goal:       "recover runtime",
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
		UpdatedAt:  time.Now().UTC().Add(-time.Minute),
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	first := runtime.RunCheckpoint{
		Run: run,
		Steps: []runtime.Step{
			{StepID: "step_a", RunID: run.RunID, Title: "first", Kind: "task", Status: runtime.StepStatusQueued, CreatedAt: time.Now().UTC().Add(-time.Minute), UpdatedAt: time.Now().UTC().Add(-time.Minute)},
		},
		CapturedAt: time.Now().UTC().Add(-30 * time.Second),
	}
	second := runtime.RunCheckpoint{
		Run: runtime.Run{
			RunID:      run.RunID,
			Entrypoint: run.Entrypoint,
			Status:     runtime.RunStatusRunning,
			Goal:       run.Goal,
			CreatedAt:  run.CreatedAt,
			UpdatedAt:  time.Now().UTC(),
		},
		Steps: []runtime.Step{
			{StepID: "step_b", RunID: run.RunID, Title: "second", Kind: "task", Status: runtime.StepStatusPlanning, CreatedAt: time.Now().UTC().Add(-20 * time.Second), UpdatedAt: time.Now().UTC()},
		},
		ToolCalls: []runtime.ToolCall{
			{ToolCallID: "tool_a", RunID: run.RunID, StepID: "step_b", CapabilityID: "shell", ToolName: "shell", Status: runtime.ToolCallStatusRequested, CreatedAt: time.Now().UTC().Add(-10 * time.Second), UpdatedAt: time.Now().UTC().Add(-10 * time.Second)},
		},
		CapturedAt: time.Now().UTC(),
	}

	if err := store.SaveCheckpoint(ctx, first); err != nil {
		t.Fatalf("SaveCheckpoint(first) returned error: %v", err)
	}
	if err := store.SaveCheckpoint(ctx, second); err != nil {
		t.Fatalf("SaveCheckpoint(second) returned error: %v", err)
	}

	checkpoints, err := store.ListLatestCheckpoints(ctx)
	if err != nil {
		t.Fatalf("ListLatestCheckpoints returned error: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 latest checkpoint, got %d", len(checkpoints))
	}
	if checkpoints[0].Run.Status != runtime.RunStatusRunning {
		t.Fatalf("expected latest checkpoint run status running, got %s", checkpoints[0].Run.Status)
	}
	if len(checkpoints[0].Steps) != 1 || checkpoints[0].Steps[0].StepID != "step_b" {
		t.Fatalf("expected latest checkpoint step_b, got %+v", checkpoints[0].Steps)
	}
	if len(checkpoints[0].ToolCalls) != 1 || checkpoints[0].ToolCalls[0].ToolCallID != "tool_a" {
		t.Fatalf("expected latest checkpoint tool call tool_a, got %+v", checkpoints[0].ToolCalls)
	}
}

func TestSQLiteStorePersistsSandboxExecutions(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	now := time.Now().UTC()
	record := SandboxExecutionRecord{
		ExecutionID: "sandbox_exec_1",
		ProfileID:   "docker_default",
		BackendKind: "docker",
		Status:      "unsupported",
		RequestedAt: now.Add(-time.Second),
		UpdatedAt:   now,
		CompletedAt: ptrTime(now),
		Document:    []byte(`{"executionId":"sandbox_exec_1","status":"unsupported","decision":{"selectionOutcome":"unsupported","hostStatus":"missing_prerequisite","mismatchReason":"backend_unavailable"}}`),
	}
	if err := store.UpsertSandboxExecution(context.Background(), record); err != nil {
		t.Fatalf("UpsertSandboxExecution returned error: %v", err)
	}

	items, err := store.ListSandboxExecutions(context.Background())
	if err != nil {
		t.Fatalf("ListSandboxExecutions returned error: %v", err)
	}
	if len(items) != 1 || items[0].ExecutionID != record.ExecutionID {
		t.Fatalf("expected persisted sandbox execution, got %+v", items)
	}
	if items[0].BackendKind != "docker" || items[0].Status != "unsupported" {
		t.Fatalf("expected stronger-backend unsupported persistence, got %+v", items[0])
	}
	if string(items[0].Document) != string(record.Document) {
		t.Fatalf("expected document %s, got %s", string(record.Document), string(items[0].Document))
	}
}

func TestSQLiteStorePersistsToolCalls(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	run := runtime.Run{
		RunID:      "run_tool",
		Entrypoint: "chat",
		Status:     runtime.RunStatusRunning,
		Goal:       "persist tool calls",
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	step := runtime.Step{
		StepID:    "step_tool",
		RunID:     run.RunID,
		Title:     "tool step",
		Kind:      "task",
		Status:    runtime.StepStatusExecutingTool,
		CreatedAt: time.Now().UTC().Add(-30 * time.Second),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	toolCall := runtime.ToolCall{
		ToolCallID:         "tool_call_1",
		RunID:              run.RunID,
		StepID:             step.StepID,
		InvocationKind:     runtime.ToolCallInvocationKindSkill,
		SkillID:            "docker-skill",
		ToolName:           "docker-skill",
		Status:             runtime.ToolCallStatusFailed,
		SandboxExecutionID: "sandbox_exec_1",
		FailureClass:       "backend_unavailable",
		Input:              map[string]any{"cmd": "pwd"},
		Output:             map[string]any{"status": "unsupported", "backendKind": "docker"},
		Sandbox: map[string]any{
			"policyRecord": map[string]any{"policyRecordId": "policy_docker_1"},
		},
		Error:     "docker backend is not available on this host",
		CreatedAt: time.Now().UTC().Add(-20 * time.Second),
		UpdatedAt: time.Now().UTC().Add(-20 * time.Second),
	}
	if err := store.UpsertToolCall(ctx, toolCall); err != nil {
		t.Fatalf("UpsertToolCall returned error: %v", err)
	}

	items, err := store.ListToolCalls(ctx, run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(items))
	}
	if items[0].SkillID != "docker-skill" || items[0].InvocationKind != runtime.ToolCallInvocationKindSkill {
		t.Fatalf("expected skill-backed tool call persistence, got %+v", items[0])
	}
	if items[0].SandboxExecutionID != "sandbox_exec_1" || items[0].FailureClass != "backend_unavailable" {
		t.Fatalf("expected stronger-backend provenance fields, got %+v", items[0])
	}

	mcpToolCall := runtime.ToolCall{
		ToolCallID:          "tool_call_2",
		RunID:               run.RunID,
		StepID:              step.StepID,
		InvocationKind:      runtime.ToolCallInvocationKindMCPTool,
		MCPServerID:         "filesystem-test",
		MCPServerName:       "Filesystem",
		MCPToolName:         "lookup",
		MCPTransportKind:    "stdio",
		MCPSessionID:        "session_1",
		AuthorizationResult: "allowed",
		ToolName:            "lookup",
		Status:              runtime.ToolCallStatusCompleted,
		Output:              map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}},
		CreatedAt:           time.Now().UTC().Add(-10 * time.Second),
		UpdatedAt:           time.Now().UTC().Add(-10 * time.Second),
	}
	if err := store.UpsertToolCall(ctx, mcpToolCall); err != nil {
		t.Fatalf("UpsertToolCall(mcp) returned error: %v", err)
	}

	items, err = store.ListToolCalls(ctx, run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls(second read) returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 tool calls after mcp insert, got %d", len(items))
	}
	if items[1].InvocationKind != runtime.ToolCallInvocationKindMCPTool || items[1].MCPServerID != "filesystem-test" || items[1].MCPSessionID != "session_1" {
		t.Fatalf("expected persisted mcp tool call provenance, got %+v", items[1])
	}
	active, err := store.HasActiveMCPToolCalls(ctx, "filesystem-test")
	if err != nil {
		t.Fatalf("HasActiveMCPToolCalls(completed) returned error: %v", err)
	}
	if active {
		t.Fatal("expected completed mcp tool call to not count as active")
	}
	mcpToolCall.Status = runtime.ToolCallStatusRunning
	mcpToolCall.UpdatedAt = time.Now().UTC()
	if err := store.UpsertToolCall(ctx, mcpToolCall); err != nil {
		t.Fatalf("UpsertToolCall(mcp running) returned error: %v", err)
	}
	active, err = store.HasActiveMCPToolCalls(ctx, "filesystem-test")
	if err != nil {
		t.Fatalf("HasActiveMCPToolCalls(running) returned error: %v", err)
	}
	if !active {
		t.Fatal("expected running mcp tool call to count as active")
	}
}

func TestSQLiteStorePersistsConsumerPolicyRecordsAndSecretScopeBindings(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	startedAt := time.Now().UTC().Add(-time.Minute)
	recordDoc := []byte(`{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"` + startedAt.Format(time.RFC3339Nano) + `","status":"approval_pending"}}`)
	if err := store.UpsertConsumerPolicyRecord(ctx, ConsumerPolicyRecordRecord{
		PolicyRecordID:   "policy_local_tool_shell_1",
		ConsumerKind:     "local_tool",
		ConsumerID:       "shell",
		OperationKind:    "tool_call.execute",
		DeclarationID:    "local_tool:shell:tool_call.execute",
		Status:           "approval_pending",
		Decision:         "ask",
		ApprovalStatus:   "pending",
		SecretResolution: "not_applicable",
		RequestedBy:      "web-ui",
		StartedAt:        startedAt,
		Document:         recordDoc,
	}); err != nil {
		t.Fatalf("UpsertConsumerPolicyRecord returned error: %v", err)
	}

	bindingDoc := []byte(`{"bindingId":"managed_provider:codex_managed","consumerKind":"managed_provider","consumerId":"codex_managed","defaultSource":"instance_override","environmentScope":"test","secretRef":"auth_file","deliveryKind":"local_state_access","redactionRule":"class_summary_only","active":true}`)
	if err := store.UpsertSecretScopeBinding(ctx, SecretScopeBindingRecord{
		BindingID:        "managed_provider:codex_managed",
		ConsumerKind:     "managed_provider",
		ConsumerID:       "codex_managed",
		EnvironmentScope: "test",
		SecretRef:        "auth_file",
		DefaultSource:    "instance_override",
		DeliveryKind:     "local_state_access",
		Active:           true,
		Document:         bindingDoc,
	}); err != nil {
		t.Fatalf("UpsertSecretScopeBinding returned error: %v", err)
	}

	records, err := store.ListConsumerPolicyRecords(ctx)
	if err != nil {
		t.Fatalf("ListConsumerPolicyRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 consumer policy record, got %d", len(records))
	}
	if records[0].PolicyRecordID != "policy_local_tool_shell_1" {
		t.Fatalf("expected policy_local_tool_shell_1, got %s", records[0].PolicyRecordID)
	}
	if records[0].Document == nil || !strings.Contains(string(records[0].Document), `"approvalId":"approval_1"`) || !strings.Contains(string(records[0].Document), `"declarationId":"local_tool:shell:tool_call.execute"`) {
		t.Fatalf("expected persisted approval linkage in consumer policy record, got %s", string(records[0].Document))
	}

	bindings, err := store.ListSecretScopeBindings(ctx, "managed_provider", "codex_managed")
	if err != nil {
		t.Fatalf("ListSecretScopeBindings returned error: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 secret scope binding, got %d", len(bindings))
	}
	if bindings[0].EnvironmentScope != "test" {
		t.Fatalf("expected test environment scope, got %s", bindings[0].EnvironmentScope)
	}
}

func TestSQLiteStoreListsEventsAfterCursor(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	first, err := store.AppendEvent(ctx, events.Event{
		EventID:    "evt_cursor_1",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: time.Now().UTC().Add(-time.Second),
		Resource:   events.Resource{Kind: "run", ID: "run_1"},
	})
	if err != nil {
		t.Fatalf("AppendEvent(first) returned error: %v", err)
	}
	second, err := store.AppendEvent(ctx, events.Event{
		EventID:    "evt_cursor_2",
		Category:   "run",
		Name:       "run.status_changed",
		OccurredAt: time.Now().UTC(),
		Resource:   events.Resource{Kind: "run", ID: "run_1"},
	})
	if err != nil {
		t.Fatalf("AppendEvent(second) returned error: %v", err)
	}

	items, err := store.ListEvents(ctx, events.Filter{Category: "run", Cursor: first.Sequence})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 event after cursor, got %d", len(items))
	}
	if items[0].Sequence != second.Sequence {
		t.Fatalf("expected sequence %d, got %d", second.Sequence, items[0].Sequence)
	}
}

func TestSQLiteStoreListsEventsByEnvironmentAndScheduleAttempt(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	if _, err := store.AppendEvent(ctx, events.Event{
		EventID:          "evt_sched_test_visible",
		EnvironmentScope: "test",
		Category:         "schedule",
		Name:             "schedule.dispatch_recorded",
		OccurredAt:       time.Now().UTC().Add(-time.Second),
		Scope: events.Scope{
			ScheduleID:        "sched_test_visible",
			ScheduleAttemptID: "sched_attempt_test_visible",
		},
		Resource: events.Resource{Kind: "schedule", ID: "sched_test_visible"},
	}); err != nil {
		t.Fatalf("AppendEvent(test visible) returned error: %v", err)
	}
	if _, err := store.AppendEvent(ctx, events.Event{
		EventID:          "evt_sched_prod_hidden",
		EnvironmentScope: "prod",
		Category:         "schedule",
		Name:             "schedule.dispatch_recorded",
		OccurredAt:       time.Now().UTC(),
		Scope: events.Scope{
			ScheduleID:        "sched_prod_hidden",
			ScheduleAttemptID: "sched_attempt_prod_hidden",
		},
		Resource: events.Resource{Kind: "schedule", ID: "sched_prod_hidden"},
	}); err != nil {
		t.Fatalf("AppendEvent(prod hidden) returned error: %v", err)
	}

	items, err := store.ListEvents(ctx, events.Filter{
		EnvironmentScope:  "test",
		ScheduleID:        "sched_test_visible",
		ScheduleAttemptID: "sched_attempt_test_visible",
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one environment-scoped schedule event, got %+v", items)
	}
	if items[0].EnvironmentScope != "test" || items[0].Scope.ScheduleAttemptID != "sched_attempt_test_visible" {
		t.Fatalf("unexpected filtered event %+v", items[0])
	}
}

func TestSQLiteStorePersistsProviderChecks(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	check := providers.Check{
		CheckID:     "provider_check_1",
		ProviderID:  "echo",
		Family:      providers.FamilyBuiltinEcho,
		AuthMode:    providers.AuthModeNone,
		Status:      providers.CheckStatusPassed,
		Model:       "echo-v1",
		Usage:       llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
		CreatedAt:   time.Now().UTC().Add(-time.Second),
		CompletedAt: time.Now().UTC(),
	}
	if err := store.UpsertProviderCheck(ctx, check); err != nil {
		t.Fatalf("UpsertProviderCheck returned error: %v", err)
	}

	items, err := store.ListProviderChecks(ctx, "echo")
	if err != nil {
		t.Fatalf("ListProviderChecks returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 provider check, got %d", len(items))
	}
	if items[0].CheckID != check.CheckID {
		t.Fatalf("expected check ID %s, got %s", check.CheckID, items[0].CheckID)
	}

	item, ok, err := store.GetProviderCheck(ctx, "echo", check.CheckID)
	if err != nil {
		t.Fatalf("GetProviderCheck returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected provider check to be found")
	}
	if item.Status != providers.CheckStatusPassed {
		t.Fatalf("expected passed status, got %s", item.Status)
	}
}

func TestSQLiteStorePersistsManagedProviderState(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	authState := providers.AuthState{
		ProviderID:          "codex_managed",
		Family:              providers.FamilyCodexCLI,
		AuthMode:            providers.AuthModeLocalCLIBridge,
		Status:              providers.AuthStatusAuthenticated,
		CLIPath:             "/usr/bin/codex",
		CLIAvailable:        true,
		AccountLabel:        "user@example.com",
		AccountID:           "acct_1",
		Plan:                "pro",
		AuthMethod:          "chatgpt",
		LoginCommand:        []string{"codex", "login"},
		LogoutCommand:       []string{"codex", "logout"},
		LastCheckedAt:       time.Now().UTC().Add(-time.Minute),
		LastAuthenticatedAt: ptrTime(time.Now().UTC().Add(-2 * time.Minute)),
		Metadata:            map[string]string{"source": "test"},
	}
	if err := store.UpsertProviderAuthState(ctx, authState); err != nil {
		t.Fatalf("UpsertProviderAuthState returned error: %v", err)
	}

	models := []providers.Model{
		{
			ProviderID:      "codex_managed",
			ModelID:         "gpt-5.4",
			DisplayName:     "GPT-5.4",
			Description:     "Primary coding model",
			Default:         true,
			Available:       true,
			Source:          "cache",
			Chat:            true,
			Stream:          true,
			Coding:          true,
			ToolUse:         false,
			ReasoningLevels: []string{"medium", "high"},
		},
	}
	if err := store.ReplaceProviderModels(ctx, "codex_managed", models); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}

	preference := providers.Preference{
		ProviderID:   "codex_managed",
		DefaultModel: "gpt-5.4",
		UpdatedAt:    time.Now().UTC(),
	}
	if err := store.UpsertProviderPreference(ctx, preference); err != nil {
		t.Fatalf("UpsertProviderPreference returned error: %v", err)
	}

	authStates, err := store.ListProviderAuthStates(ctx)
	if err != nil {
		t.Fatalf("ListProviderAuthStates returned error: %v", err)
	}
	if len(authStates) != 1 || authStates[0].ProviderID != "codex_managed" {
		t.Fatalf("unexpected auth states: %+v", authStates)
	}

	persistedModels, err := store.ListProviderModelsByProvider(ctx, "codex_managed")
	if err != nil {
		t.Fatalf("ListProviderModelsByProvider returned error: %v", err)
	}
	if len(persistedModels) != 1 || persistedModels[0].ModelID != "gpt-5.4" {
		t.Fatalf("unexpected provider models: %+v", persistedModels)
	}
	if len(persistedModels[0].ReasoningLevels) != 2 {
		t.Fatalf("expected reasoning levels to persist, got %+v", persistedModels[0].ReasoningLevels)
	}

	preferences, err := store.ListProviderPreferences(ctx)
	if err != nil {
		t.Fatalf("ListProviderPreferences returned error: %v", err)
	}
	if len(preferences) != 1 || preferences[0].DefaultModel != "gpt-5.4" {
		t.Fatalf("unexpected provider preferences: %+v", preferences)
	}
}

func TestSQLiteStoreUsesCurrentSchemaVersion(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	version, err := store.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion returned error: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", CurrentSchemaVersion, version)
	}
}

func TestSQLiteStoreUpgradesLegacyBaselineSchema(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}

	for _, stmt := range schemaMigrations[0].Statements {
		if strings.Contains(stmt, "schema_migrations") {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("db.Exec legacy baseline statement returned error: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	version, err := store.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion returned error: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("expected upgraded schema version %d, got %d", CurrentSchemaVersion, version)
	}
}

func TestSQLiteStoreRejectsFutureSchemaVersion(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("db.Exec create schema_migrations returned error: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, CurrentSchemaVersion+1, "future", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("db.Exec insert future schema version returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if _, err := NewSQLiteStore(dataDir); err == nil {
		t.Fatal("expected NewSQLiteStore to reject future schema version")
	}
}

func TestSQLiteStoreRoundTripsCalendarDomainRecords(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	account := calendar.AccountProjection{
		CalendarAccountID:       "acct_calendar_store",
		IntegrationID:           "calendar-a",
		DomainKind:              "calendar",
		EnvironmentScope:        "test",
		AccountKey:              "acct_primary",
		AccountLabel:            "Primary",
		ReadinessStatus:         "healthy",
		CanonicalDefault:        true,
		SelectionMode:           "canonical_default",
		PrimaryCalendarRef:      "primary",
		PrimaryCalendarLabel:    "Primary Calendar",
		PrimaryTimezone:         "America/Los_Angeles",
		SupportsEventInspection: true,
		SupportsBusyFree:        true,
		SupportsTimedMutation:   true,
		LastSyncedAt:            now,
		UpdatedAt:               now,
	}
	if err := store.UpsertCalendarAccount(ctx, account); err != nil {
		t.Fatalf("UpsertCalendarAccount returned error: %v", err)
	}

	operation := calendar.Operation{
		OperationID:       "calendar_op_store_1",
		OperationClass:    calendar.OperationClassCreateEvent,
		Status:            calendar.OperationStatusCompleted,
		IntegrationID:     account.IntegrationID,
		CalendarAccountID: account.CalendarAccountID,
		EnvironmentScope:  "test",
		CalendarRef:       "primary",
		SelectionMode:     "explicit",
		TimezoneUsed:      "America/Los_Angeles",
		RequestSummary:    "create test event",
		ExternalEventID:   "evt_store_1",
		RunID:             "run_calendar_store",
		WorkflowID:        "wf_calendar_store",
		ScheduleID:        "sched_calendar_store",
		DeliveryID:        "delivery_calendar_store",
		ArtifactIDs:       []string{"artifact_calendar_store_1"},
		CreatedAt:         now,
		CompletedAt:       ptrTime(now),
		UpdatedAt:         now,
	}
	if err := store.UpsertCalendarOperation(ctx, operation); err != nil {
		t.Fatalf("UpsertCalendarOperation returned error: %v", err)
	}

	startsAt := now.Add(time.Hour)
	endsAt := now.Add(2 * time.Hour)
	artifact := calendar.Artifact{
		ArtifactID:              "artifact_calendar_store_1",
		OperationID:             operation.OperationID,
		Kind:                    calendar.ArtifactKindEventSnapshot,
		IntegrationID:           account.IntegrationID,
		EnvironmentScope:        "test",
		ExternalEventID:         operation.ExternalEventID,
		CalendarRef:             "primary",
		Title:                   "Store Event",
		StartsAt:                &startsAt,
		EndsAt:                  &endsAt,
		Timezone:                "America/Los_Angeles",
		MutationEligibleInPhase: true,
		LifecycleState:          calendar.EventLifecycleStateActive,
		CreatedAt:               now,
	}
	if err := store.UpsertCalendarArtifact(ctx, artifact); err != nil {
		t.Fatalf("UpsertCalendarArtifact returned error: %v", err)
	}

	accounts, err := store.ListCalendarAccounts(ctx, "test")
	if err != nil {
		t.Fatalf("ListCalendarAccounts returned error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].IntegrationID != account.IntegrationID {
		t.Fatalf("expected calendar account round-trip, got %+v", accounts)
	}

	operations, err := store.ListCalendarOperations(ctx, "test", CalendarOperationFilter{
		RunID:           operation.RunID,
		OperationClass:  string(calendar.OperationClassCreateEvent),
		ExternalEventID: operation.ExternalEventID,
	})
	if err != nil {
		t.Fatalf("ListCalendarOperations returned error: %v", err)
	}
	if len(operations) != 1 || operations[0].OperationID != operation.OperationID {
		t.Fatalf("expected calendar operation round-trip, got %+v", operations)
	}

	artifacts, err := store.ListCalendarArtifacts(ctx, "test", operation.OperationID)
	if err != nil {
		t.Fatalf("ListCalendarArtifacts returned error: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ArtifactID != artifact.ArtifactID {
		t.Fatalf("expected calendar artifact round-trip, got %+v", artifacts)
	}

	workflowFiltered, err := store.ListCalendarOperations(ctx, "test", CalendarOperationFilter{WorkflowID: operation.WorkflowID})
	if err != nil {
		t.Fatalf("ListCalendarOperations(workflow) returned error: %v", err)
	}
	if len(workflowFiltered) != 1 || workflowFiltered[0].OperationID != operation.OperationID {
		t.Fatalf("expected workflow-filtered calendar operation, got %+v", workflowFiltered)
	}

	scheduleFiltered, err := store.ListCalendarOperations(ctx, "test", CalendarOperationFilter{ScheduleID: operation.ScheduleID})
	if err != nil {
		t.Fatalf("ListCalendarOperations(schedule) returned error: %v", err)
	}
	if len(scheduleFiltered) != 1 || scheduleFiltered[0].OperationID != operation.OperationID {
		t.Fatalf("expected schedule-filtered calendar operation, got %+v", scheduleFiltered)
	}

	deliveryFiltered, err := store.ListCalendarOperations(ctx, "test", CalendarOperationFilter{DeliveryID: operation.DeliveryID})
	if err != nil {
		t.Fatalf("ListCalendarOperations(delivery) returned error: %v", err)
	}
	if len(deliveryFiltered) != 1 || deliveryFiltered[0].OperationID != operation.OperationID {
		t.Fatalf("expected delivery-filtered calendar operation, got %+v", deliveryFiltered)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

// listRunsForTest is a thin wrapper around ListRunsAllTenantsForTest
// preserved for the store-package's internal regression tests that
// pre-date the tenant-scope migration. Production code MUST go through
// the tenancy layer (tenancy.Runtime.ListRunsForTenant).
func listRunsForTest(t *testing.T, s *SQLiteStore, ctx context.Context) ([]runtime.Run, error) {
	t.Helper()
	return s.ListRunsAllTenantsForTest(ctx)
}
