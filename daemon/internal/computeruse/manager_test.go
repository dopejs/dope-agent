package computeruse_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type stubArtifactRecorder struct{}

type quotaDeniedArtifactRecorder struct{}

type failingDriver struct {
	startErr   error
	executeErr error
}

func (d failingDriver) StartSession(_ context.Context, session computeruse.Session, _ computeruse.CreateSessionInput) (computeruse.Session, error) {
	session.Status = computeruse.SessionStatusActive
	session.DriverKind = "browser"
	return session, d.startErr
}

func (d failingDriver) ExecuteAction(_ context.Context, session computeruse.Session, action computeruse.Action) (computeruse.Session, computeruse.Action, []computeruse.ArtifactCaptureRequest, error) {
	return session, computeruse.Action{}, nil, d.executeErr
}

func (d failingDriver) CloseSession(context.Context, computeruse.Session) (computeruse.Session, error) {
	return computeruse.Session{}, nil
}

func (stubArtifactRecorder) SaveComputerUseArtifact(_ context.Context, input computeruse.ArtifactCaptureRequest) (computeruse.Artifact, error) {
	return computeruse.Artifact{
		ArtifactID:           "artifact_test",
		EnvironmentScope:     "test",
		ComputerUseSessionID: input.ComputerUseSessionID,
		ComputerUseActionID:  input.ComputerUseActionID,
		RunID:                input.RunID,
		Kind:                 input.Kind,
		Status:               computeruse.ArtifactStatusAvailable,
		FileName:             input.FileName,
		MIMEType:             input.MIMEType,
		ByteSize:             int64(len(input.Content)),
		StorageKey:           "computer-use/test/artifact_test",
		CreatedAt:            actionTime(),
	}, nil
}

func (stubArtifactRecorder) ReadComputerUseArtifactContent(context.Context, string) ([]byte, error) {
	return []byte("artifact"), nil
}

func (quotaDeniedArtifactRecorder) SaveComputerUseArtifact(context.Context, computeruse.ArtifactCaptureRequest) (computeruse.Artifact, error) {
	return computeruse.Artifact{}, billing.ErrQuotaDenied
}

func (quotaDeniedArtifactRecorder) ReadComputerUseArtifactContent(context.Context, string) ([]byte, error) {
	return nil, billing.ErrQuotaDenied
}

func actionTime() time.Time {
	return time.Now().UTC()
}

func TestManagerCreatesNavigateAndApprovalGatedActions(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policyEngine,
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if session.Status != computeruse.SessionStatusActive {
		t.Fatalf("expected active session, got %+v", session)
	}

	navigateResult, approval, decision, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindNavigate,
		URL:        "https://example.test/page",
	})
	if err != nil {
		t.Fatalf("CreateAction(navigate) returned error: %v", err)
	}
	if approval != nil || decision != nil || navigateResult.Pending {
		t.Fatalf("expected low-risk navigate to execute immediately, got approval=%+v decision=%+v result=%+v", approval, decision, navigateResult)
	}
	if navigateResult.Action.Status != computeruse.ActionStatusCompleted {
		t.Fatalf("expected completed navigate action, got %+v", navigateResult.Action)
	}

	inputResult, approval, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindInput,
		Value:      "Phase 26",
		TargetMatchContext: &computeruse.TargetMatchContext{
			MatchStrategy:    "dom_selector",
			ExpectedSelector: "#name",
		},
	})
	if err != nil {
		t.Fatalf("CreateAction(input) returned error: %v", err)
	}
	if !inputResult.Pending || approval == nil {
		t.Fatalf("expected pending approval-gated action, got result=%+v approval=%+v", inputResult, approval)
	}
	if inputResult.Action.Status != computeruse.ActionStatusWaitingApproval {
		t.Fatalf("expected waiting approval action, got %+v", inputResult.Action)
	}

	if _, _, err := policyEngine.ResolveApproval(approval.ApprovalID, policy.ResolveApprovalInput{Resolution: "approved", Comment: "allow"}); err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}
	resumed, resumedOK, err := manager.ResumePendingAction(context.Background(), approval.ApprovalID)
	if err != nil {
		t.Fatalf("ResumePendingAction returned error: %v", err)
	}
	if !resumedOK {
		t.Fatal("expected ResumePendingAction to find the pending action")
	}
	if resumed.Status != computeruse.ActionStatusCompleted {
		t.Fatalf("expected resumed action to complete, got %+v", resumed)
	}
	if len(resumed.Artifacts) == 0 {
		t.Fatalf("expected completed high-risk action to record evidence, got %+v", resumed)
	}
}

func TestManagerRejectsUnsupportedDriverAndPageTarget(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use validation"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policy.NewEngine(),
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	if _, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "desktop"}); err == nil || !strings.Contains(err.Error(), "browser-first") {
		t.Fatalf("expected browser-first driver rejection, got %v", err)
	}

	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if _, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindClick,
		PageTarget: computeruse.PageTargetNewTab,
	}); err == nil || !strings.Contains(err.Error(), "single active page") {
		t.Fatalf("expected extra-tab rejection, got %v", err)
	}
}

func TestManagerFailsUnsupportedActionAndTargetMismatch(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use failures"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policyEngine,
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if _, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKind("desktop_drag"),
	}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported action rejection, got %v", err)
	}

	result, approval, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindClick,
		TargetMatchContext: &computeruse.TargetMatchContext{
			MatchStrategy:    "dom_selector",
			ExpectedSelector: "#missing-button",
		},
	})
	if err != nil {
		t.Fatalf("CreateAction(target mismatch) returned error: %v", err)
	}
	if !result.Pending || approval == nil {
		t.Fatalf("expected approval-gated mismatch action, got result=%+v approval=%+v", result, approval)
	}
	if _, _, err := policyEngine.ResolveApproval(approval.ApprovalID, policy.ResolveApprovalInput{Resolution: "approved", Comment: "allow"}); err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}
	resumed, resumedOK, err := manager.ResumePendingAction(context.Background(), approval.ApprovalID)
	if err != nil {
		t.Fatalf("ResumePendingAction returned error: %v", err)
	}
	if !resumedOK || resumed.Status != computeruse.ActionStatusFailed {
		t.Fatalf("expected failed resumed action, got ok=%v action=%+v", resumedOK, resumed)
	}
	if resumed.FailureClass != string(computeruse.FailureClassTargetMismatch) {
		t.Fatalf("expected target mismatch failure class, got %+v", resumed)
	}
	if len(resumed.Artifacts) == 0 {
		t.Fatalf("expected target mismatch evidence, got %+v", resumed)
	}
}

func TestManagerMapsNavigationFailureAndUnavailableConsumer(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use failures"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policy.NewEngine(),
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	navigateResult, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindNavigate,
	})
	if err != nil {
		t.Fatalf("CreateAction(navigate failure) returned error: %v", err)
	}
	if navigateResult.Action.Status != computeruse.ActionStatusFailed || navigateResult.Action.FailureClass != string(computeruse.FailureClassNavigationFailure) {
		t.Fatalf("expected navigation failure mapping, got %+v", navigateResult.Action)
	}

	unavailableManager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policy.NewEngine(),
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
		Driver:           failingDriver{executeErr: errors.New("driver unavailable")},
	})
	unavailableSession, err := unavailableManager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession(unavailable manager) returned error: %v", err)
	}
	waitResult, _, _, err := unavailableManager.CreateAction(context.Background(), run.RunID, unavailableSession.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindWait,
		WaitMs:     1,
	})
	if err != nil {
		t.Fatalf("CreateAction(unavailable consumer) returned error: %v", err)
	}
	if waitResult.Action.Status != computeruse.ActionStatusFailed || waitResult.Action.FailureClass != string(computeruse.FailureClassUnavailableConsumer) {
		t.Fatalf("expected unavailable consumer mapping, got %+v", waitResult.Action)
	}
}

func TestManagerSessionAndActionLatencyStayLocal(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use latency"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policy.NewEngine(),
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	createStarted := time.Now()
	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if elapsed := time.Since(createStarted); elapsed > 500*time.Millisecond {
		t.Fatalf("expected session create under 500ms, got %s", elapsed)
	}

	actionStarted := time.Now()
	result, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindSnapshot,
	})
	if err != nil {
		t.Fatalf("CreateAction(snapshot) returned error: %v", err)
	}
	if elapsed := time.Since(actionStarted); elapsed > time.Second {
		t.Fatalf("expected action completion under 1s, got %s", elapsed)
	}
	if len(result.Action.Artifacts) == 0 {
		t.Fatalf("expected artifact evidence, got %+v", result.Action)
	}
}

func TestManagerPropagatesQuotaDeniedArtifactCapture(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use quota denial"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policy.NewEngine(),
		Store:            sqliteStore,
		Artifacts:        quotaDeniedArtifactRecorder{},
	})
	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	_, _, _, err = manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindSnapshot,
	})
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected artifact quota denial to propagate, got %v", err)
	}
}

func TestManagerSupportsCoreActionMatrix(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use action matrix"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	manager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policyEngine,
		Store:            sqliteStore,
		Artifacts:        stubArtifactRecorder{},
	})

	session, err := manager.CreateSession(context.Background(), run.RunID, computeruse.CreateSessionInput{
		DriverKind: "browser",
		InitialURL: "https://example.test/start",
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	navigateOne, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindNavigate,
		URL:        "https://example.test/first",
	})
	if err != nil {
		t.Fatalf("CreateAction(navigate first) returned error: %v", err)
	}
	navigateTwo, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindNavigate,
		URL:        "https://example.test/second",
	})
	if err != nil {
		t.Fatalf("CreateAction(navigate second) returned error: %v", err)
	}
	back, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindBack,
	})
	if err != nil {
		t.Fatalf("CreateAction(back) returned error: %v", err)
	}
	forward, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindForward,
	})
	if err != nil {
		t.Fatalf("CreateAction(forward) returned error: %v", err)
	}
	waitResult, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindWait,
		WaitMs:     10,
	})
	if err != nil {
		t.Fatalf("CreateAction(wait) returned error: %v", err)
	}
	screenshotResult, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindScreenshot,
	})
	if err != nil {
		t.Fatalf("CreateAction(screenshot) returned error: %v", err)
	}

	selectPending, selectApproval, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind:         computeruse.ActionKindSelect,
		SelectedValue:      "large",
		TargetMatchContext: &computeruse.TargetMatchContext{ExpectedSelector: "#size"},
	})
	if err != nil {
		t.Fatalf("CreateAction(select) returned error: %v", err)
	}
	if !selectPending.Pending || selectApproval == nil {
		t.Fatalf("expected select to require approval, got result=%+v approval=%+v", selectPending, selectApproval)
	}
	if _, _, err := policyEngine.ResolveApproval(selectApproval.ApprovalID, policy.ResolveApprovalInput{Resolution: "approved", Comment: "allow select"}); err != nil {
		t.Fatalf("ResolveApproval(select) returned error: %v", err)
	}
	selectResult, ok, err := manager.ResumePendingAction(context.Background(), selectApproval.ApprovalID)
	if err != nil || !ok {
		t.Fatalf("ResumePendingAction(select) returned ok=%v err=%v", ok, err)
	}

	downloadPending, downloadApproval, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind:         computeruse.ActionKindDownload,
		TargetMatchContext: &computeruse.TargetMatchContext{ExpectedSelector: "#export"},
	})
	if err != nil {
		t.Fatalf("CreateAction(download) returned error: %v", err)
	}
	if !downloadPending.Pending || downloadApproval == nil {
		t.Fatalf("expected download to require approval, got result=%+v approval=%+v", downloadPending, downloadApproval)
	}
	if _, _, err := policyEngine.ResolveApproval(downloadApproval.ApprovalID, policy.ResolveApprovalInput{Resolution: "approved", Comment: "allow download"}); err != nil {
		t.Fatalf("ResolveApproval(download) returned error: %v", err)
	}
	downloadResult, ok, err := manager.ResumePendingAction(context.Background(), downloadApproval.ApprovalID)
	if err != nil || !ok {
		t.Fatalf("ResumePendingAction(download) returned ok=%v err=%v", ok, err)
	}

	closeResult, _, _, err := manager.CreateAction(context.Background(), run.RunID, session.ComputerUseSessionID, "tester", computeruse.CreateActionInput{
		ActionKind: computeruse.ActionKindCloseSession,
	})
	if err != nil {
		t.Fatalf("CreateAction(close_session) returned error: %v", err)
	}

	if navigateOne.Action.PageAfter == nil || navigateOne.Action.PageAfter.URL != "https://example.test/first" {
		t.Fatalf("expected first navigate to set page, got %+v", navigateOne.Action)
	}
	if navigateTwo.Action.PageAfter == nil || navigateTwo.Action.PageAfter.URL != "https://example.test/second" {
		t.Fatalf("expected second navigate to set page, got %+v", navigateTwo.Action)
	}
	if back.Action.PageAfter == nil || back.Action.PageAfter.URL != "https://example.test/first" {
		t.Fatalf("expected back to restore prior page, got %+v", back.Action)
	}
	if forward.Action.PageAfter == nil || forward.Action.PageAfter.URL != "https://example.test/second" {
		t.Fatalf("expected forward to restore next page, got %+v", forward.Action)
	}
	if waitResult.Action.Status != computeruse.ActionStatusCompleted {
		t.Fatalf("expected wait completion, got %+v", waitResult.Action)
	}
	if len(screenshotResult.Action.Artifacts) == 0 || screenshotResult.Action.Artifacts[0].Kind != computeruse.ArtifactKindScreenshot {
		t.Fatalf("expected screenshot artifact, got %+v", screenshotResult.Action)
	}
	if selectResult.PageAfter == nil || !strings.Contains(selectResult.PageAfter.Title, "#size=large") {
		t.Fatalf("expected select to project chosen state, got %+v", selectResult)
	}
	var downloadArtifactFound bool
	for _, artifact := range downloadResult.Artifacts {
		if artifact.Kind == computeruse.ArtifactKindDownload {
			downloadArtifactFound = true
			break
		}
	}
	if !downloadArtifactFound {
		t.Fatalf("expected download artifact, got %+v", downloadResult)
	}
	if closeResult.Action.Status != computeruse.ActionStatusCompleted {
		t.Fatalf("expected close_session completion, got %+v", closeResult.Action)
	}
	restoredSession, ok, err := manager.GetSession(context.Background(), run.RunID, session.ComputerUseSessionID)
	if err != nil || !ok {
		t.Fatalf("GetSession returned ok=%v err=%v", ok, err)
	}
	if restoredSession.Status != computeruse.SessionStatusClosed {
		t.Fatalf("expected closed session, got %+v", restoredSession)
	}
}
