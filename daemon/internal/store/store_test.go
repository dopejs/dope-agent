package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
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

	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
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
		ToolCallID:   "tool_call_1",
		RunID:        run.RunID,
		StepID:       step.StepID,
		CapabilityID: "shell",
		ToolName:     "shell.exec",
		Status:       runtime.ToolCallStatusRequested,
		Input:        map[string]any{"cmd": "pwd"},
		CreatedAt:    time.Now().UTC().Add(-20 * time.Second),
		UpdatedAt:    time.Now().UTC().Add(-20 * time.Second),
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
	if items[0].CapabilityID != "shell" {
		t.Fatalf("expected capability id shell, got %s", items[0].CapabilityID)
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
